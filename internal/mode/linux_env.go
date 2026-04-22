package mode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mihoctl/internal/core"
)

const (
	linuxProxyEnvFileName  = "system-proxy.env"
	linuxProxyManagedStart = "# >>> mihoctl system proxy >>>"
	linuxProxyManagedEnd   = "# <<< mihoctl system proxy <<<"
)

func (m *Manager) applyLinuxEnvSystemProxy(enabled bool) error {
	envPath := m.linuxProxyEnvFile()
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return core.NewActionError("system_proxy_env_dir_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
			"path": filepath.Dir(envPath),
		}, nil)
	}
	if err := os.WriteFile(envPath, []byte(m.renderLinuxProxyEnv(enabled)), 0o644); err != nil {
		return core.NewActionError("system_proxy_env_write_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
			"path": envPath,
		}, nil)
	}
	if err := m.ensureLinuxShellIntegration(envPath); err != nil {
		return err
	}
	return nil
}

func (m *Manager) linuxEnvSystemProxyStatus() (*bool, error) {
	envPath := m.linuxProxyEnvFile()
	data, err := os.ReadFile(envPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, core.NewActionError("system_proxy_env_read_failed", "err.mode.status_proxy", err, "err.path.check_permission", map[string]any{
			"path": envPath,
		}, nil)
	}
	text := string(data)
	switch {
	case strings.Contains(text, "export MIHOCTL_SYSTEM_PROXY_ENABLED=1"):
		enabled := true
		return &enabled, nil
	case strings.Contains(text, "export MIHOCTL_SYSTEM_PROXY_ENABLED=0"):
		enabled := false
		return &enabled, nil
	default:
		return nil, nil
	}
}

func (m *Manager) linuxProxyEnvFile() string {
	return filepath.Join(m.paths.AppHome, linuxProxyEnvFileName)
}

func (m *Manager) RenderLinuxProxyEnv(enabled bool) string {
	return m.renderLinuxProxyEnv(enabled)
}

func (m *Manager) renderLinuxProxyEnv(enabled bool) string {
	httpProxy := fmt.Sprintf("http://%s:%d", m.cfg.SystemProxy.Host, m.cfg.SystemProxy.Port)
	socksProxy := fmt.Sprintf("socks5://%s:%d", m.cfg.SystemProxy.Host, m.cfg.SystemProxy.Port)

	if !enabled {
		return strings.Join([]string{
			"# Managed by MihoCTL. New shells source this file to follow the current proxy mode.",
			"export MIHOCTL_SYSTEM_PROXY_ENABLED=0",
			"unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY",
			"unset no_proxy NO_PROXY",
			"",
		}, "\n")
	}

	return strings.Join([]string{
		"# Managed by MihoCTL. New shells source this file to follow the current proxy mode.",
		"export MIHOCTL_SYSTEM_PROXY_ENABLED=1",
		fmt.Sprintf("export http_proxy=%q", httpProxy),
		fmt.Sprintf("export https_proxy=%q", httpProxy),
		fmt.Sprintf("export all_proxy=%q", socksProxy),
		"export HTTP_PROXY=\"$http_proxy\"",
		"export HTTPS_PROXY=\"$https_proxy\"",
		"export ALL_PROXY=\"$all_proxy\"",
		"export no_proxy=\"127.0.0.1,localhost,::1\"",
		"export NO_PROXY=\"$no_proxy\"",
		"",
	}, "\n")
}

func (m *Manager) ensureLinuxShellIntegration(envPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return core.NewActionError("system_proxy_home_failed", "err.mode.set_proxy", err, "", nil, nil)
	}
	candidates := []string{
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}

	executable := strings.TrimSpace(m.paths.ExecPath)
	if executable == "" {
		executable = "mihoctl"
	}
	command := strings.Join([]string{
		shellQuote(executable),
		"--config",
		shellQuote(m.paths.ConfigFile),
		"config",
		"env-shell",
	}, " ")
	block := strings.Join([]string{
		linuxProxyManagedStart,
		fmt.Sprintf("eval \"$(%s 2>/dev/null || true)\"", command),
		linuxProxyManagedEnd,
		"",
	}, "\n")

	for _, path := range candidates {
		if err := upsertManagedShellBlock(path, block); err != nil {
			return core.NewActionError("system_proxy_shell_update_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
				"path": path,
			}, nil)
		}
	}
	return nil
}

func upsertManagedShellBlock(path, block string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)
	start := strings.Index(content, linuxProxyManagedStart)
	end := strings.Index(content, linuxProxyManagedEnd)
	switch {
	case start >= 0 && end >= start:
		end += len(linuxProxyManagedEnd)
		content = content[:start] + strings.TrimLeft(block, "\n") + content[end:]
	default:
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += block
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
