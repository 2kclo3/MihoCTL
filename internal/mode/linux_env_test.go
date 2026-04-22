package mode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mihoctl/internal/config"
	"mihoctl/internal/mihomo"
	"mihoctl/internal/state"
)

func TestRenderLinuxProxyEnvEnabled(t *testing.T) {
	t.Parallel()

	manager := NewManager(config.Paths{AppHome: t.TempDir()}, &config.Config{
		SystemProxy: config.SystemProxy{
			Host: "127.0.0.1",
			Port: 7890,
		},
	}, &state.State{}, mihomo.NewClient("http://127.0.0.1:9090", ""))

	script := manager.renderLinuxProxyEnv(true)
	if !strings.Contains(script, `export MIHOCTL_SYSTEM_PROXY_ENABLED=1`) {
		t.Fatalf("expected enabled marker in script: %s", script)
	}
	if !strings.Contains(script, `export http_proxy="http://127.0.0.1:7890"`) {
		t.Fatalf("expected http_proxy export in script: %s", script)
	}
	if !strings.Contains(script, `export all_proxy="socks5://127.0.0.1:7890"`) {
		t.Fatalf("expected all_proxy export in script: %s", script)
	}
}

func TestRenderLinuxProxyEnvDisabled(t *testing.T) {
	t.Parallel()

	manager := NewManager(config.Paths{AppHome: t.TempDir()}, &config.Config{
		SystemProxy: config.SystemProxy{
			Host: "127.0.0.1",
			Port: 7890,
		},
	}, &state.State{}, mihomo.NewClient("http://127.0.0.1:9090", ""))

	script := manager.renderLinuxProxyEnv(false)
	if !strings.Contains(script, `export MIHOCTL_SYSTEM_PROXY_ENABLED=0`) {
		t.Fatalf("expected disabled marker in script: %s", script)
	}
	if !strings.Contains(script, `unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY`) {
		t.Fatalf("expected proxy unset in script: %s", script)
	}
}

func TestUpsertManagedShellBlockIsIdempotent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	profilePath := filepath.Join(tempDir, ".profile")
	block := strings.Join([]string{
		linuxProxyManagedStart,
		`[ -f "/tmp/proxy.env" ] && . "/tmp/proxy.env"`,
		linuxProxyManagedEnd,
		"",
	}, "\n")

	if err := upsertManagedShellBlock(profilePath, block); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := upsertManagedShellBlock(profilePath, block); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	data := readFileForTest(t, profilePath)
	if strings.Count(data, linuxProxyManagedStart) != 1 {
		t.Fatalf("expected exactly one managed block, got: %s", data)
	}
}

func TestEnsureLinuxShellIntegrationUsesDynamicEnvShellCommand(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	t.Setenv("HOME", homeDir)

	manager := NewManager(config.Paths{
		AppHome:    filepath.Join(tempDir, "app"),
		ExecPath:   "/usr/local/bin/mihoctl",
		ConfigFile: filepath.Join(tempDir, "app", "config.json"),
	}, &config.Config{
		SystemProxy: config.SystemProxy{
			Host: "127.0.0.1",
			Port: 7890,
		},
	}, &state.State{}, mihomo.NewClient("http://127.0.0.1:9090", ""))

	if err := manager.ensureLinuxShellIntegration(filepath.Join(tempDir, "app", linuxProxyEnvFileName)); err != nil {
		t.Fatalf("ensureLinuxShellIntegration failed: %v", err)
	}

	for _, name := range []string{".profile", ".bashrc", ".zshrc"} {
		data := readFileForTest(t, filepath.Join(homeDir, name))
		if !strings.Contains(data, `config env-shell`) {
			t.Fatalf("expected dynamic env-shell command in %s, got: %s", name, data)
		}
		if strings.Contains(data, `system-proxy.env" ] && .`) {
			t.Fatalf("expected static env-file sourcing to be removed in %s, got: %s", name, data)
		}
	}
}

func readFileForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", path, err)
	}
	return string(data)
}
