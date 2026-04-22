package coremanager

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
	"mihoctl/internal/progress"
	"mihoctl/internal/state"
)

const githubAPI = "https://api.github.com"

var errBundledNotFound = errors.New("bundled core not found")

type Manager struct {
	cfg    *config.Config
	state  *state.State
	paths  config.Paths
	client *http.Client
	out    io.Writer
	text   Text
}

type Text struct {
	FetchLatest    string
	FetchVersion   string
	DownloadBinary string
	UseBundled     string
	CheckUpdate    string
}

type InstallResult struct {
	Version         string
	PreviousVersion string
	AssetName       string
	BinaryPath      string
	Source          string
}

type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	Available      bool
}

type releaseResponse struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func NewManager(cfg *config.Config, st *state.State, paths config.Paths, out io.Writer, text Text) *Manager {
	if out == nil {
		out = io.Discard
	}
	return &Manager{
		cfg:   cfg,
		state: st,
		paths: paths,
		out:   out,
		text:  text,
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (m *Manager) Install(ctx context.Context, version string) (*InstallResult, error) {
	if strings.TrimSpace(version) == "" {
		if result, err := m.installBundled(); err == nil {
			return result, nil
		} else if !errors.Is(err, errBundledNotFound) {
			return nil, err
		}
	}
	return m.install(ctx, false, version)
}

func (m *Manager) Upgrade(ctx context.Context, version string) (*InstallResult, error) {
	return m.install(ctx, true, version)
}

func (m *Manager) install(ctx context.Context, isUpgrade bool, version string) (*InstallResult, error) {
	release, err := m.fetchRelease(ctx, version, true)
	if err != nil {
		return nil, err
	}

	asset, err := selectAsset(release.Assets)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(m.cfg.Core.InstallDir, 0o755); err != nil {
		return nil, core.NewActionError("core_install_dir_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Core.InstallDir,
		}, nil)
	}

	targetPath := filepath.Join(m.cfg.Core.InstallDir, "mihomo")
	if err := m.downloadAndInstall(ctx, asset, targetPath); err != nil {
		return nil, err
	}

	m.cfg.Mihomo.BinaryPath = targetPath
	previousVersion := m.state.Core.Version
	m.state.Core = state.CoreState{
		Version:     release.TagName,
		AssetName:   asset.Name,
		InstalledAt: time.Now(),
		Source:      "github",
	}

	if isUpgrade && previousVersion == "" {
		previousVersion = "unknown"
	}

	return &InstallResult{
		Version:         release.TagName,
		PreviousVersion: previousVersion,
		AssetName:       asset.Name,
		BinaryPath:      targetPath,
		Source:          "github",
	}, nil
}

func (m *Manager) installBundled() (*InstallResult, error) {
	bundle, err := m.findBundledAsset()
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(m.out, strings.ReplaceAll(m.text.UseBundled, "{path}", bundle.Root))

	if err := os.MkdirAll(m.cfg.Core.InstallDir, 0o755); err != nil {
		return nil, core.NewActionError("core_install_dir_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Core.InstallDir,
		}, nil)
	}
	if err := os.MkdirAll(m.cfg.Core.DatabaseDir, 0o755); err != nil {
		return nil, core.NewActionError("core_install_dir_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Core.DatabaseDir,
		}, nil)
	}

	targetPath := filepath.Join(m.cfg.Core.InstallDir, "mihomo")
	if err := copyFile(bundle.BinaryPath, targetPath, 0o755); err != nil {
		return nil, core.NewActionError("bundled_core_copy_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
			"path": targetPath,
		}, nil)
	}
	for _, file := range bundle.DatabaseFiles {
		target := filepath.Join(m.cfg.Core.DatabaseDir, filepath.Base(file))
		if err := copyFile(file, target, 0o644); err != nil {
			return nil, core.NewActionError("bundled_db_copy_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
				"path": target,
			}, nil)
		}
	}

	m.cfg.Mihomo.BinaryPath = targetPath
	m.state.Core = state.CoreState{
		Version:     bundle.Version,
		AssetName:   bundle.AssetName,
		InstalledAt: time.Now(),
		Source:      "bundled",
	}

	return &InstallResult{
		Version:    bundle.Version,
		AssetName:  bundle.AssetName,
		BinaryPath: targetPath,
		Source:     "bundled",
	}, nil
}

func (m *Manager) fetchRelease(ctx context.Context, version string, announce bool) (*releaseResponse, error) {
	if announce {
		fmt.Fprintln(m.out, m.fetchMessage(version))
	}
	apiPath := fmt.Sprintf("/repos/%s/releases/latest", m.cfg.Core.Repo)
	messageData := map[string]any{
		"repo": m.cfg.Core.Repo,
	}
	if version != "" {
		normalized := normalizeTag(version)
		apiPath = fmt.Sprintf("/repos/%s/releases/tags/%s", m.cfg.Core.Repo, normalized)
		messageData["version"] = normalized
	}

	url := githubAPI + apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "mihoctl")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, core.NewActionError("core_release_fetch_failed", releaseFetchMessageKey(version), err, "err.core.check_network", messageData, nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, core.NewActionError("core_release_fetch_failed", releaseFetchMessageKey(version), fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))), "err.core.check_network", messageData, nil)
	}

	payload := &releaseResponse{}
	if err := json.NewDecoder(resp.Body).Decode(payload); err != nil {
		return nil, core.NewActionError("core_release_decode_failed", releaseFetchMessageKey(version), err, "err.core.check_network", messageData, nil)
	}
	if payload.TagName == "" || len(payload.Assets) == 0 {
		return nil, core.NewActionError("core_release_empty", releaseFetchMessageKey(version), fmt.Errorf("empty release payload"), "err.core.check_network", messageData, nil)
	}
	return payload, nil
}

func (m *Manager) ShouldCheckForUpdate() bool {
	if !m.cfg.Core.AutoCheckUpdates {
		return false
	}
	if m.state.Core.Version == "" {
		return false
	}
	interval := time.Duration(m.cfg.Core.CheckIntervalHour) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if m.state.Core.LastCheckedAt.IsZero() {
		return true
	}
	return time.Since(m.state.Core.LastCheckedAt) >= interval
}

func (m *Manager) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	fmt.Fprintln(m.out, m.text.CheckUpdate)

	release, err := m.fetchRelease(ctx, "", false)
	if err != nil {
		return nil, err
	}

	info := &UpdateInfo{
		CurrentVersion: m.state.Core.Version,
		LatestVersion:  release.TagName,
		Available:      compareVersions(release.TagName, m.state.Core.Version) > 0,
	}
	m.state.Core.LastCheckedAt = time.Now()
	m.state.Core.LatestVersion = release.TagName
	m.state.Core.UpdateAvailable = info.Available
	return info, nil
}

func (m *Manager) downloadAndInstall(ctx context.Context, asset releaseAsset, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mihoctl")

	resp, err := m.client.Do(req)
	if err != nil {
		return core.NewActionError("core_download_failed", "err.core.download", err, "err.core.check_network", map[string]any{
			"url": asset.BrowserDownloadURL,
		}, nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return core.NewActionError("core_download_failed", "err.core.download", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))), "err.core.check_network", map[string]any{
			"url": asset.BrowserDownloadURL,
		}, nil)
	}

	label := strings.ReplaceAll(m.text.DownloadBinary, "{name}", asset.Name)
	reporter := progress.New(m.out, label, resp.ContentLength)
	defer reporter.Finish()

	networkReader := reporter.Wrap(resp.Body)
	reader := io.Reader(networkReader)
	if strings.HasSuffix(asset.Name, ".gz") {
		gzipReader, err := gzip.NewReader(networkReader)
		if err != nil {
			return core.NewActionError("core_unzip_failed", "err.core.unpack", err, "", nil, nil)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "mihomo-*")
	if err != nil {
		return core.NewActionError("core_tempfile_failed", "err.core.install_dir", err, "err.path.check_permission", map[string]any{
			"path": filepath.Dir(targetPath),
		}, nil)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// 先写临时文件再替换目标文件，避免升级失败时留下半截二进制。
	if _, err := io.Copy(tempFile, reader); err != nil {
		tempFile.Close()
		return core.NewActionError("core_write_failed", "err.core.download", err, "", nil, nil)
	}
	if err := tempFile.Close(); err != nil {
		return core.NewActionError("core_write_failed", "err.core.download", err, "", nil, nil)
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return core.NewActionError("core_chmod_failed", "err.core.install_permission", err, "err.path.check_permission", map[string]any{
			"path": targetPath,
		}, nil)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return core.NewActionError("core_replace_failed", "err.core.replace", err, "err.path.check_permission", map[string]any{
			"path": targetPath,
		}, nil)
	}
	return nil
}

func selectAsset(assets []releaseAsset) (releaseAsset, error) {
	type candidate struct {
		asset releaseAsset
		score int
	}

	var candidates []candidate
	for _, asset := range assets {
		score := scoreAsset(asset.Name)
		if score < 0 {
			continue
		}
		candidates = append(candidates, candidate{asset: asset, score: score})
	}
	if len(candidates) == 0 {
		return releaseAsset{}, core.NewActionError("core_asset_not_found", "err.core.asset_not_found", fmt.Errorf("%s/%s", runtime.GOOS, runtime.GOARCH), "", nil, nil)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].asset.Name < candidates[j].asset.Name
		}
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].asset, nil
}

func scoreAsset(name string) int {
	lower := strings.ToLower(name)
	if !strings.HasPrefix(lower, "mihomo-"+runtime.GOOS+"-") {
		return -1
	}
	if strings.Contains(lower, ".deb") || strings.Contains(lower, ".rpm") || strings.Contains(lower, ".apk") {
		return -1
	}

	score := 100
	switch runtime.GOARCH {
	case "amd64":
		if !strings.Contains(lower, "-amd64-") {
			return -1
		}
	case "arm64":
		if !(strings.Contains(lower, "-arm64-v8-") || strings.Contains(lower, "-arm64-")) {
			return -1
		}
		score += 10
	case "386":
		if !strings.Contains(lower, "-386-") {
			return -1
		}
	case "arm":
		if !strings.Contains(lower, "-armv7-") {
			return -1
		}
	default:
		return -1
	}

	if strings.HasSuffix(lower, ".gz") {
		score += 20
	}
	if strings.Contains(lower, "compatible") {
		score -= 20
	}
	if strings.Contains(lower, "go120") || strings.Contains(lower, "go122") || strings.Contains(lower, "go124") {
		score -= 15
	}
	if strings.Count(lower, "-") <= 4 {
		score += 15
	}
	return score
}

func normalizeTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	if first := version[0]; first < '0' || first > '9' {
		return version
	}
	return "v" + version
}

func releaseFetchMessageKey(version string) string {
	if strings.TrimSpace(version) != "" {
		return "err.core.fetch_release_by_tag"
	}
	return "err.core.fetch_release"
}

type bundledAsset struct {
	Root          string
	Version       string
	BinaryPath    string
	AssetName     string
	DatabaseFiles []string
}

func (m *Manager) findBundledAsset() (*bundledAsset, error) {
	for _, root := range m.bundledRoots() {
		platformDir := filepath.Join(root, runtime.GOOS+"-"+runtime.GOARCH)
		binaryPath := filepath.Join(platformDir, "mihomo")
		if stat, err := os.Stat(binaryPath); err != nil || stat.IsDir() {
			continue
		}

		version := readFirstLine(filepath.Join(platformDir, "version.txt"))
		if version == "" {
			version = readFirstLine(filepath.Join(root, "version.txt"))
		}
		if version == "" {
			version = "bundled"
		}

		files := collectRegularFiles(filepath.Join(root, "common"))
		files = append(files, collectRegularFiles(filepath.Join(platformDir, "db"))...)
		for _, file := range collectRegularFiles(platformDir) {
			name := filepath.Base(file)
			if name == "mihomo" || name == "version.txt" {
				continue
			}
			files = append(files, file)
		}

		return &bundledAsset{
			Root:          root,
			Version:       normalizeTag(version),
			BinaryPath:    binaryPath,
			AssetName:     filepath.Base(root) + "/" + runtime.GOOS + "-" + runtime.GOARCH,
			DatabaseFiles: uniqueStrings(files),
		}, nil
	}
	return nil, errBundledNotFound
}

func (m *Manager) bundledRoots() []string {
	candidates := []string{
		filepath.Join(m.paths.ExecDir, "bundled"),
		filepath.Join(m.paths.ExecDir, "assets", "bundled"),
		filepath.Join(m.paths.CWD, "bundled"),
		filepath.Join(m.paths.CWD, "assets", "bundled"),
	}
	return uniqueStrings(candidates)
}

func collectRegularFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return files
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func readFirstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(dst), "mihoctl-copy-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(tempFile, in); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return err
	}
	return os.Rename(tempPath, dst)
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if va > vb {
			return 1
		}
		if va < vb {
			return -1
		}
	}
	return 0
}

func parseVersion(version string) []int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		result = append(result, n)
	}
	return result
}

func (m *Manager) fetchMessage(version string) string {
	if strings.TrimSpace(version) != "" {
		return strings.ReplaceAll(m.text.FetchVersion, "{version}", normalizeTag(version))
	}
	return m.text.FetchLatest
}
