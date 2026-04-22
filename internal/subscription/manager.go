package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
	"mihoctl/internal/mihomo"
	"mihoctl/internal/progress"
)

type Manager struct {
	cfg    *config.Config
	paths  config.Paths
	client *mihomo.Client
	out    io.Writer
	label  string
}

type AddResult struct {
	Entry             config.Subscription
	Update            *UpdateResult
	ResolvedFrom      string
	DetectedUserAgent string
}

type UpdateResult struct {
	Name     string
	URL      string
	Path     string
	Bytes    int
	Reloaded bool
}

func NewManager(cfg *config.Config, paths config.Paths, client *mihomo.Client, out io.Writer, label string) *Manager {
	if out == nil {
		out = io.Discard
	}
	return &Manager{cfg: cfg, paths: paths, client: client, out: out, label: label}
}

func (m *Manager) Add(ctx context.Context, rawURL string) (*AddResult, error) {
	importLink, err := ResolveImportLink(rawURL)
	if err != nil {
		return nil, core.NewActionError("subscription_invalid_url", "err.subscription.invalid_url", err, "err.subscription.check_url", map[string]any{
			"url": rawURL,
		}, nil)
	}
	parsed, err := url.ParseRequestURI(importLink.URL)
	if err != nil {
		return nil, core.NewActionError("subscription_invalid_url", "err.subscription.invalid_url", err, "err.subscription.check_url", map[string]any{
			"url": rawURL,
		}, nil)
	}
	for _, item := range m.cfg.Subscriptions {
		if item.URL == importLink.URL {
			return nil, core.NewActionError("subscription_duplicate", "err.subscription.duplicate", nil, "", nil, nil)
		}
	}

	nameBase := strings.TrimSpace(importLink.Name)
	if nameBase == "" {
		nameBase = parsed.Host
	}
	name := uniqueName(m.cfg.Subscriptions, nameBase)
	firstSubscription := len(m.cfg.Subscriptions) == 0
	entry := config.Subscription{
		Name:       name,
		URL:        importLink.URL,
		ConfigPath: filepath.Join(m.paths.SubDir, sanitizeFileName(name)+".yaml"),
		UserAgent:  strings.TrimSpace(importLink.UserAgent),
	}
	m.cfg.Subscriptions = append(m.cfg.Subscriptions, entry)
	if firstSubscription {
		m.cfg.DefaultSubscription = entry.Name
	}

	result, err := m.UpdateOne(ctx, entry.Name)
	if err != nil {
		m.cfg.Subscriptions = removeSubscriptionByName(m.cfg.Subscriptions, entry.Name)
		if firstSubscription {
			m.cfg.DefaultSubscription = ""
		}
		return nil, err
	}
	addResult := &AddResult{
		Entry:  entry,
		Update: result,
	}
	if importLink.Wrapped && importLink.Original != importLink.URL {
		addResult.ResolvedFrom = importLink.Original
	}
	if entry.UserAgent != "" {
		addResult.DetectedUserAgent = entry.UserAgent
	}
	return addResult, nil
}

func (m *Manager) List() []config.Subscription {
	return m.cfg.Subscriptions
}

type BatchUpdateResult struct {
	Successes []UpdateResult
	Failures  []BatchFailure
}

type BatchFailure struct {
	Name string
	URL  string
	Err  error
}

type fetchAttemptResult struct {
	response  *http.Response
	userAgent string
}

const compatibilityClientVersion = "999.999.999"

func (m *Manager) Update(ctx context.Context, target string) (*BatchUpdateResult, error) {
	if len(m.cfg.Subscriptions) == 0 {
		return nil, core.NewActionError("subscription_empty", "err.subscription.empty", nil, "", nil, nil)
	}

	result := &BatchUpdateResult{}
	if strings.TrimSpace(target) != "" {
		item, err := m.findSubscription(target)
		if err != nil {
			return nil, err
		}
		updateResult, updateErr := m.updateSubscription(ctx, item)
		if updateErr != nil {
			result.Failures = append(result.Failures, BatchFailure{Name: item.Name, URL: item.URL, Err: updateErr})
		} else {
			result.Successes = append(result.Successes, *updateResult)
		}
		return result, nil
	}

	for _, item := range m.cfg.Subscriptions {
		updateResult, updateErr := m.updateSubscription(ctx, item)
		if updateErr != nil {
			result.Failures = append(result.Failures, BatchFailure{Name: item.Name, URL: item.URL, Err: updateErr})
			continue
		}
		result.Successes = append(result.Successes, *updateResult)
	}
	return result, nil
}

func (m *Manager) UpdateOne(ctx context.Context, target string) (*UpdateResult, error) {
	item, err := m.findSubscription(target)
	if err != nil {
		return nil, err
	}
	return m.updateSubscription(ctx, item)
}

func (m *Manager) Use(ctx context.Context, target string) (*config.Subscription, error) {
	item, err := m.findSubscription(target)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(item.ConfigPath); err != nil {
		return nil, core.NewActionError("subscription_config_missing", "err.subscription.config_missing", err, "err.subscription.update_first", map[string]any{
			"name": item.Name,
		}, nil)
	}
	if _, err := m.activateSubscription(ctx, item); err != nil {
		return nil, err
	}
	m.cfg.DefaultSubscription = item.Name
	return &item, nil
}

func (m *Manager) Remove(ctx context.Context, target string) (*config.Subscription, error) {
	item, err := m.findSubscription(target)
	if err != nil {
		return nil, err
	}

	m.cfg.Subscriptions = removeSubscriptionByName(m.cfg.Subscriptions, item.Name)
	if item.ConfigPath != "" {
		_ = os.Remove(item.ConfigPath)
	}

	if m.cfg.DefaultSubscription == item.Name || m.cfg.DefaultSubscription == item.URL {
		m.cfg.DefaultSubscription = ""
		_ = os.Remove(m.cfg.Mihomo.ConfigPath)
	}

	return &item, nil
}

func (m *Manager) SetUserAgent(target, userAgent string) (*config.Subscription, error) {
	item, err := m.findSubscription(target)
	if err != nil {
		return nil, err
	}
	trimmedUA := strings.TrimSpace(userAgent)
	for i := range m.cfg.Subscriptions {
		entry := &m.cfg.Subscriptions[i]
		if entry.Name != item.Name && entry.URL != item.URL {
			continue
		}
		entry.UserAgent = trimmedUA
		updated := *entry
		return &updated, nil
	}
	return nil, core.NewActionError("subscription_not_found", "err.subscription.not_found", errors.New(target), "", map[string]any{
		"name": target,
	}, nil)
}

func (m *Manager) ClearUserAgent(target string) (*config.Subscription, error) {
	return m.SetUserAgent(target, "")
}

func (m *Manager) updateSubscription(ctx context.Context, item config.Subscription) (*UpdateResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return nil, core.NewActionError("subscription_invalid_url", "err.subscription.invalid_url", err, "err.subscription.check_url", map[string]any{
			"url": item.URL,
		}, nil)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	attempt, err := m.fetchSubscription(ctx, client, req, item)
	if err != nil {
		return nil, core.NewActionError("subscription_download_failed", "err.subscription.download", err, "err.subscription.check_url", map[string]any{
			"url": item.URL,
		}, nil)
	}
	resp := attempt.response
	defer resp.Body.Close()

	if attempt.userAgent != "" && attempt.userAgent != item.UserAgent {
		item.UserAgent = attempt.userAgent
		m.updateSubscriptionMeta(item)
	}

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		suggestionKey := "err.subscription.check_url"
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			suggestionKey = "err.subscription.check_auth"
		}
		return nil, core.NewActionError("subscription_download_failed", "err.subscription.download", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))), suggestionKey, map[string]any{
			"url": item.URL,
		}, nil)
	}

	if err := os.MkdirAll(m.paths.SubDir, 0o755); err != nil {
		return nil, core.NewActionError("subscription_dir_failed", "err.path.create_app_home", err, "err.path.check_permission", map[string]any{
			"path": m.paths.SubDir,
		}, nil)
	}

	file, err := os.Create(item.ConfigPath)
	if err != nil {
		return nil, core.NewActionError("subscription_write_failed", "err.config.write", err, "err.path.check_permission", map[string]any{
			"path": item.ConfigPath,
		}, nil)
	}
	defer file.Close()

	reporter := progress.New(m.out, m.label, resp.ContentLength)
	defer reporter.Finish()

	n, err := io.Copy(file, reporter.Wrap(resp.Body))
	if err != nil {
		return nil, core.NewActionError("subscription_download_failed", "err.subscription.download", err, "err.subscription.check_url", map[string]any{
			"url": item.URL,
		}, nil)
	}

	result := &UpdateResult{
		Name:  item.Name,
		URL:   item.URL,
		Path:  item.ConfigPath,
		Bytes: int(n),
	}

	if m.isDefault(item) {
		reloaded, err := m.activateSubscription(ctx, item)
		if err == nil {
			result.Reloaded = reloaded
		}
	}
	return result, nil
}

func (m *Manager) fetchSubscription(ctx context.Context, client *http.Client, baseReq *http.Request, item config.Subscription) (*fetchAttemptResult, error) {
	candidates := buildUserAgentCandidates(item.UserAgent)
	var lastResp *http.Response

	for _, candidate := range candidates {
		req := baseReq.Clone(ctx)
		// 部分机场会按客户端特征限制订阅拉取，401/403 时尝试常见 Clash/Mihomo 客户端 UA 兜底。
		applySubscriptionHeaders(req, candidate)

		resp, err := client.Do(req)
		if err != nil {
			if lastResp != nil {
				lastResp.Body.Close()
			}
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			if lastResp != nil {
				lastResp.Body.Close()
			}
			lastResp = resp
			continue
		}
		if lastResp != nil {
			lastResp.Body.Close()
		}
		return &fetchAttemptResult{response: resp, userAgent: candidate}, nil
	}

	if lastResp != nil {
		return &fetchAttemptResult{response: lastResp}, nil
	}
	resp, err := client.Do(baseReq.Clone(ctx))
	if err != nil {
		return nil, err
	}
	return &fetchAttemptResult{response: resp}, nil
}

func (m *Manager) activateSubscription(ctx context.Context, item config.Subscription) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(m.cfg.Mihomo.ConfigPath), 0o755); err != nil {
		return false, err
	}
	if err := copyFile(item.ConfigPath, m.cfg.Mihomo.ConfigPath); err != nil {
		return false, core.NewActionError("subscription_activate_failed", "err.subscription.activate", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}

	// 切换默认配置时优先保证本地 active config 已落盘，热重载失败不应阻塞后续启动。
	reloadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.client.ReloadConfig(reloadCtx, m.cfg.Mihomo.ConfigPath); err != nil {
		return false, nil
	}
	return true, nil
}

func (m *Manager) findSubscription(target string) (config.Subscription, error) {
	if len(m.cfg.Subscriptions) == 0 {
		return config.Subscription{}, core.NewActionError("subscription_empty", "err.subscription.empty", nil, "", nil, nil)
	}

	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		if m.cfg.DefaultSubscription == "" {
			return config.Subscription{}, core.NewActionError("subscription_no_default", "err.subscription.no_default", nil, "err.subscription.use_first", nil, nil)
		}
		for _, item := range m.cfg.Subscriptions {
			if item.Name == m.cfg.DefaultSubscription || item.URL == m.cfg.DefaultSubscription {
				return normalizeSubscription(item, m.paths.SubDir), nil
			}
		}
		return config.Subscription{}, core.NewActionError("subscription_no_default", "err.subscription.no_default", nil, "err.subscription.use_first", nil, nil)
	}

	if index, ok := parseSelectionIndex(trimmed); ok {
		if index >= 0 && index < len(m.cfg.Subscriptions) {
			return normalizeSubscription(m.cfg.Subscriptions[index], m.paths.SubDir), nil
		}
		return config.Subscription{}, core.NewActionError("subscription_not_found", "err.subscription.not_found", errors.New(trimmed), "", map[string]any{
			"name": trimmed,
		}, nil)
	}

	for _, item := range m.cfg.Subscriptions {
		if item.Name == trimmed || item.URL == trimmed {
			return normalizeSubscription(item, m.paths.SubDir), nil
		}
	}
	return config.Subscription{}, core.NewActionError("subscription_not_found", "err.subscription.not_found", errors.New(trimmed), "", map[string]any{
		"name": trimmed,
	}, nil)
}

func (m *Manager) isDefault(item config.Subscription) bool {
	defaultValue := strings.TrimSpace(m.cfg.DefaultSubscription)
	if defaultValue == "" {
		return false
	}
	return defaultValue == item.Name || defaultValue == item.URL
}

func normalizeSubscription(item config.Subscription, subDir string) config.Subscription {
	if item.ConfigPath == "" {
		item.ConfigPath = filepath.Join(subDir, sanitizeFileName(item.Name)+".yaml")
	}
	return item
}

func parseSelectionIndex(value string) (int, bool) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index <= 0 {
		return 0, false
	}
	return index - 1, true
}

func (m *Manager) updateSubscriptionMeta(updated config.Subscription) {
	for i := range m.cfg.Subscriptions {
		item := &m.cfg.Subscriptions[i]
		if item.Name != updated.Name && item.URL != updated.URL {
			continue
		}
		item.UserAgent = updated.UserAgent
		if item.ConfigPath == "" {
			item.ConfigPath = updated.ConfigPath
		}
		return
	}
}

func buildUserAgentCandidates(preferred string) []string {
	candidates := make([]string, 0, len(commonSubscriptionUserAgents)+2)
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	add(preferred)
	for _, value := range commonSubscriptionUserAgents {
		add(value)
	}
	add("")
	return candidates
}

func applySubscriptionHeaders(req *http.Request, userAgent string) {
	req.Header.Set("Accept", "application/x-yaml, text/yaml, text/plain, */*")
	if username := req.URL.User.Username(); username != "" {
		password, _ := req.URL.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Authorization", "Basic "+token)
	} else {
		req.Header.Del("Authorization")
	}
	if strings.TrimSpace(userAgent) == "" {
		req.Header.Del("User-Agent")
		return
	}
	req.Header.Set("User-Agent", userAgent)
}

var commonSubscriptionUserAgents = func() []string {
	flClashUA := buildFlClashCompatibleUserAgent()
	return []string{
		buildClashVergeCompatibleUserAgent(),
		flClashUA,
		buildClashForWindowsCompatibleUserAgent(),
		buildV2RayNCompatibleUserAgent(),
		"ClashMetaForAndroid",
		"ClashMeta",
		"Mihomo",
		"Clash",
		"FlClash",
	}
}()

func buildFlClashCompatibleUserAgent() string {
	// 使用兼容高版本号模板，避免每次真实客户端小版本变化都要更新代码。
	return fmt.Sprintf("FlClash/v%s clash-verge Platform/%s", compatibilityClientVersion, flClashPlatformName())
}

func buildClashVergeCompatibleUserAgent() string {
	return "clash-verge/v" + compatibilityClientVersion
}

func buildClashForWindowsCompatibleUserAgent() string {
	return "ClashforWindows/" + compatibilityClientVersion
}

func buildV2RayNCompatibleUserAgent() string {
	return "v2rayN/" + compatibilityClientVersion
}

func flClashPlatformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
}

func uniqueName(items []config.Subscription, base string) string {
	name := base
	if name == "" {
		name = "subscription"
	}
	used := make(map[string]struct{}, len(items))
	for _, item := range items {
		used[item.Name] = struct{}{}
	}
	if _, ok := used[name]; !ok {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func removeSubscriptionByName(items []config.Subscription, name string) []config.Subscription {
	result := make([]config.Subscription, 0, len(items))
	for _, item := range items {
		if item.Name == name {
			continue
		}
		result = append(result, item)
	}
	return result
}

func sanitizeFileName(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "\t", "-")
	value = replacer.Replace(strings.TrimSpace(value))
	if value == "" {
		return "subscription"
	}
	return value
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(dst), "mihoctl-sub-*")
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
	return os.Rename(tempPath, dst)
}
