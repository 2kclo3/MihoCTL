package selfmanager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
)

type Manager struct {
	paths config.Paths
}

type InstallResult struct {
	Path        string
	NeedsPath   bool
	InstallMode string
}

type UninstallResult struct {
	Removed  []string
	Warnings []string
}

func NewManager(paths config.Paths) *Manager {
	return &Manager{paths: paths}
}

func (m *Manager) Install(targetDir string) (*InstallResult, error) {
	if m.paths.ExecPath == "" {
		return nil, core.NewActionError("self_exec_path_missing", "err.self.install", fmt.Errorf("executable path is empty"), "", nil, nil)
	}

	dir := strings.TrimSpace(targetDir)
	mode := "custom"
	if dir == "" {
		var err error
		dir, mode, err = m.defaultInstallDir()
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, core.NewActionError("self_install_dir_failed", "err.self.install", err, "err.path.check_permission", map[string]any{
			"path": dir,
		}, nil)
	}

	targetPath := filepath.Join(dir, "mihoctl")
	if err := copyExecutable(m.paths.ExecPath, targetPath); err != nil {
		return nil, core.NewActionError("self_install_failed", "err.self.install", err, "err.path.check_permission", map[string]any{
			"path": targetPath,
		}, nil)
	}

	return &InstallResult{
		Path:        targetPath,
		NeedsPath:   !pathContains(dir),
		InstallMode: mode,
	}, nil
}

func (m *Manager) Uninstall(cfg *config.Config) (*UninstallResult, error) {
	result := &UninstallResult{}

	stopMihomoRuntime(result, cfg)

	removeFile(result, m.paths.ExecPath)
	removeFile(result, filepath.Join(m.paths.ExecDir, "mihoctl-enable-tun"))
	removeFile(result, filepath.Join(m.paths.ExecDir, "mihoctl-disable-tun"))
	removeDir(result, filepath.Join(m.paths.ExecDir, "bundled"))

	home, _ := os.UserHomeDir()
	if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
		removeFile(result, filepath.Join(xdgDataHome, "bash-completion", "completions", "mihoctl"))
	}
	if home != "" {
		removeFile(result, filepath.Join(home, ".local", "share", "bash-completion", "completions", "mihoctl"))
		removeFile(result, filepath.Join(home, ".config", "fish", "completions", "mihoctl.fish"))
		removeManagedBlock(result, filepath.Join(home, ".profile"), "# >>> mihoctl system proxy >>>", "# <<< mihoctl system proxy <<<", "system proxy integration")
		removeManagedBlock(result, filepath.Join(home, ".bashrc"), "# >>> mihoctl shell integration >>>", "# <<< mihoctl shell integration <<<", "shell integration")
		removeManagedBlock(result, filepath.Join(home, ".bashrc"), "# >>> mihoctl system proxy >>>", "# <<< mihoctl system proxy <<<", "system proxy integration")
	}
	zshHome := home
	if zdotdir := strings.TrimSpace(os.Getenv("ZDOTDIR")); zdotdir != "" {
		zshHome = zdotdir
	}
	if zshHome != "" {
		removeFile(result, filepath.Join(zshHome, ".zsh", "completions", "_mihoctl"))
		removeManagedBlock(result, filepath.Join(zshHome, ".zshrc"), "# >>> mihoctl shell integration >>>", "# <<< mihoctl shell integration <<<", "shell integration")
		removeManagedBlock(result, filepath.Join(zshHome, ".zshrc"), "# >>> mihoctl system proxy >>>", "# <<< mihoctl system proxy <<<", "system proxy integration")
	}

	if cfg != nil {
		removeDir(result, m.paths.AppHome)
		if m.shouldRemoveManagedPath(cfg.Mihomo.WorkDir) {
			removeDir(result, cfg.Mihomo.WorkDir)
		}
		if dir := filepath.Dir(cfg.Mihomo.ConfigPath); dir != "" && dir != cfg.Mihomo.WorkDir && m.shouldRemoveManagedPath(dir) {
			removeDir(result, dir)
		}
		if dir := cfg.Core.DatabaseDir; dir != "" && dir != cfg.Mihomo.WorkDir && m.shouldRemoveManagedPath(dir) {
			removeDir(result, dir)
		}
		if binary := cfg.Mihomo.BinaryPath; binary != "" && binary != m.paths.ExecPath && m.shouldRemoveManagedPath(binary) {
			removeFile(result, binary)
		}
	}

	removeServiceArtifacts(result)
	return result, nil
}

func (m *Manager) defaultInstallDir() (string, string, error) {
	candidates := []struct {
		path string
		mode string
	}{
		{path: "/usr/local/bin", mode: "system"},
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, struct {
			path string
			mode string
		}{
			path: filepath.Join(home, ".local", "bin"),
			mode: "user",
		})
	}

	for _, candidate := range candidates {
		if isDirWritable(candidate.path) {
			return candidate.path, candidate.mode, nil
		}
	}

	if home != "" {
		return filepath.Join(home, ".local", "bin"), "user", nil
	}
	return "", "", core.NewActionError("self_install_dir_missing", "err.self.install", fmt.Errorf("no writable install directory found"), "", nil, nil)
}

func isDirWritable(path string) bool {
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		testFile := filepath.Join(path, ".mihoctl-write-test")
		if err := os.WriteFile(testFile, []byte("ok"), 0o644); err == nil {
			_ = os.Remove(testFile)
			return true
		}
	}
	return false
}

func pathContains(dir string) bool {
	pathValue := os.Getenv("PATH")
	for _, item := range filepath.SplitList(pathValue) {
		if item == dir {
			return true
		}
	}
	return false
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(dst), "mihoctl-*")
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
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tempPath, dst)
}

func removeFile(result *UninstallResult, path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", path, err))
		return
	}
	result.Removed = append(result.Removed, path)
}

func removeDir(result *UninstallResult, path string) {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" || path == "." {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := os.RemoveAll(path); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", path, err))
		return
	}
	result.Removed = append(result.Removed, path)
}

func removeManagedBlock(result *UninstallResult, path, startMarker, endMarker, label string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start < 0 || end < start {
		return
	}
	end += len(endMarker)
	updated := content[:start] + content[end:]
	updated = strings.TrimLeft(updated, "\n")
	updated = strings.ReplaceAll(updated, "\n\n\n", "\n\n")
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", path, err))
		return
	}
	result.Removed = append(result.Removed, path+" ("+label+")")
}

func removeServiceArtifacts(result *UninstallResult) {
	switch runtime.GOOS {
	case "linux":
		removeFile(result, "/etc/systemd/system/mihomo.service")
	case "darwin":
		removeFile(result, "/Library/LaunchDaemons/com.mihoctl.mihomo.plist")
	}
}

func stopMihomoRuntime(result *UninstallResult, cfg *config.Config) {
	switch runtime.GOOS {
	case "linux":
		runCleanupCommand(result, "systemctl", "disable", "--now", "mihomo")
		runCleanupCommand(result, "systemctl", "daemon-reload")
	case "darwin":
		runCleanupCommand(result, "launchctl", "bootout", "system", "/Library/LaunchDaemons/com.mihoctl.mihomo.plist")
	}

	// 卸载时尽量清掉残留 Mihomo 进程，避免文件删掉后代理还在继续跑。
	runCleanupCommand(result, "pkill", "-x", "mihomo")
	if cfg != nil && strings.TrimSpace(cfg.Mihomo.BinaryPath) != "" {
		runCleanupCommand(result, "pkill", "-f", cfg.Mihomo.BinaryPath)
	}
}

func runCleanupCommand(result *UninstallResult, name string, args ...string) {
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		if isIgnorableCleanupError(err) {
			return
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s %s: %v", name, strings.Join(args, " "), err))
		return
	}
	result.Removed = append(result.Removed, fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
}

func isIgnorableCleanupError(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	switch exitErr.ExitCode() {
	case 1, 5:
		return true
	default:
		return false
	}
}

func (m *Manager) shouldRemoveManagedPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	managedRoots := []string{
		m.paths.AppHome,
		m.paths.BinDir,
		defaultManagedMihomoDir(),
	}
	for _, root := range managedRoots {
		if isSameOrWithin(path, root) {
			return true
		}
	}
	return false
}

func defaultManagedMihomoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mihomo")
	default:
		return filepath.Join(home, ".config", "mihomo")
	}
}

func isSameOrWithin(path, root string) bool {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(root)
	if cleanPath == cleanRoot {
		return true
	}
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
