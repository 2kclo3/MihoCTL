package mode

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mihoctl/internal/config"
	"mihoctl/internal/mihomo"
	"mihoctl/internal/state"
)

func TestUpsertTunConfigContentAppendsManagedBlock(t *testing.T) {
	content := "mixed-port: 7890\nmode: rule\n"

	updated := upsertTunConfigContent(content, true, "linux")

	if !strings.Contains(updated, "mixed-port: 7890") {
		t.Fatalf("expected original config to be preserved: %s", updated)
	}
	if !strings.Contains(updated, tunManagedStart) {
		t.Fatalf("expected managed marker to be present: %s", updated)
	}
	if !strings.Contains(updated, "  enable: true") {
		t.Fatalf("expected tun enable=true to be written: %s", updated)
	}
	if !strings.Contains(updated, "  auto-redirect: true") {
		t.Fatalf("expected linux-specific tun settings to be written: %s", updated)
	}
	if !strings.Contains(updated, "dns:\n  enable: true") {
		t.Fatalf("expected dns.enable=true to be added for tun mode: %s", updated)
	}
}

func TestUpsertTunConfigContentReplacesExistingTunBlock(t *testing.T) {
	content := strings.Join([]string{
		"mixed-port: 7890",
		"dns:",
		"  enable: false",
		"  ipv6: false",
		"tun:",
		"  enable: false",
		"  stack: gvisor",
		"proxy-groups:",
		"  - name: Auto",
		"",
	}, "\n")

	updated := upsertTunConfigContent(content, true, "darwin")

	if strings.Contains(updated, "stack: gvisor") {
		t.Fatalf("expected old tun block to be replaced: %s", updated)
	}
	if !strings.Contains(updated, "proxy-groups:") {
		t.Fatalf("expected following top-level config to be preserved: %s", updated)
	}
	if !strings.Contains(updated, "  enable: true") {
		t.Fatalf("expected tun enable=true to be written: %s", updated)
	}
	if !strings.Contains(updated, "dns:\n  enable: true\n  ipv6: false") {
		t.Fatalf("expected dns.enable to be turned on while preserving dns block: %s", updated)
	}
	if strings.Contains(updated, "  auto-redirect: true") {
		t.Fatalf("expected darwin config to omit linux-only auto-redirect: %s", updated)
	}
}

func TestUpsertTunConfigContentPreservesDNSEnableWhenTurningTunOff(t *testing.T) {
	content := strings.Join([]string{
		"dns:",
		"  enable: false",
		"",
	}, "\n")

	updated := upsertTunConfigContent(content, false, "linux")

	if !strings.Contains(updated, "dns:\n  enable: false") {
		t.Fatalf("expected turning tun off to avoid rewriting dns.enable: %s", updated)
	}
}

func TestReadTunEnabledFromContent(t *testing.T) {
	content := strings.Join([]string{
		"mixed-port: 7890",
		"tun:",
		"  enable: true # inline comment",
		"  stack: system",
		"",
	}, "\n")

	enabled := readTunEnabledFromContent(content)
	if enabled == nil || !*enabled {
		t.Fatalf("expected tun enabled to be detected, got %#v", enabled)
	}
}

func TestManagerSetTunFallsBackToConfigWhenControllerOffline(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := upsertTunConfigFile(writeTestConfig(t, configPath, "mixed-port: 7890\n"), false); err != nil {
		t.Fatalf("prepare config: %v", err)
	}

	cfg := &config.Config{
		Controller: config.Controller{
			Address: "http://127.0.0.1:1",
		},
		Mihomo: config.Mihomo{
			ConfigPath: configPath,
		},
	}
	manager := NewManager(config.Paths{}, cfg, &state.State{}, mihomo.NewClient(cfg.Controller.Address, ""))

	if err := manager.SetTun(true); err != nil {
		t.Fatalf("expected offline controller to still allow config persistence, got %v", err)
	}
	enabled, err := readTunEnabledFromConfigFile(configPath)
	if err != nil {
		t.Fatalf("read tun config: %v", err)
	}
	if enabled == nil || !*enabled {
		t.Fatalf("expected persisted tun config to be enabled, got %#v", enabled)
	}
	if manager.state.Modes.TUN.Source != "config" {
		t.Fatalf("expected source=config, got %q", manager.state.Modes.TUN.Source)
	}
}

func TestManagerTunStatusFallsBackToConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	writeTestConfig(t, configPath, "mixed-port: 7890\ntun:\n  enable: true\n")

	cfg := &config.Config{
		Controller: config.Controller{
			Address: "http://127.0.0.1:1",
		},
		Mihomo: config.Mihomo{
			ConfigPath: configPath,
		},
	}
	manager := NewManager(config.Paths{}, cfg, &state.State{}, mihomo.NewClient(cfg.Controller.Address, ""))

	status, err := manager.TunStatus()
	if err != nil {
		t.Fatalf("expected status fallback from config, got %v", err)
	}
	if !status.Known || !status.Enabled {
		t.Fatalf("expected tun status enabled from config, got %#v", status)
	}
	if status.Source != "config" {
		t.Fatalf("expected source=config, got %q", status.Source)
	}
}

func TestManagerSetTunDisableFallsBackToRuntimeWhenConfigMissing(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "missing-config.yaml")
	runtimeEnabled := true

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			recorder := newResponseRecorder()

			switch {
			case r.Method == http.MethodPatch && r.URL.Path == "/configs":
				runtimeEnabled = false
				recorder.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodGet && r.URL.Path == "/configs":
				recorder.Header().Set("Content-Type", "application/json")
				_, _ = recorder.Write([]byte(`{"version":"test","tun":{"enable":false}}`))
			default:
				recorder.WriteHeader(http.StatusNotFound)
			}

			return recorder.Result(), nil
		}),
	}

	cfg := &config.Config{
		Controller: config.Controller{
			Address: "http://mihomo.test",
		},
		Mihomo: config.Mihomo{
			ConfigPath: configPath,
		},
	}
	manager := NewManager(
		config.Paths{LogFile: filepath.Join(tempDir, "mihomo.log")},
		cfg,
		&state.State{},
		mihomo.NewClientWithHTTPClient(cfg.Controller.Address, "", httpClient),
	)

	if err := manager.SetTun(false); err != nil {
		t.Fatalf("expected runtime-only tun disable to succeed, got %v", err)
	}
	if runtimeEnabled {
		t.Fatalf("expected runtime tun state to be disabled")
	}
	if !manager.state.Modes.TUN.Known || manager.state.Modes.TUN.Enabled {
		t.Fatalf("expected tun state to be known and disabled, got %#v", manager.state.Modes.TUN)
	}
	if manager.state.Modes.TUN.Source != "api" {
		t.Fatalf("expected source=api, got %q", manager.state.Modes.TUN.Source)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
		code:   http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
}

func (r *responseRecorder) Result() *http.Response {
	return &http.Response{
		StatusCode: r.code,
		Header:     r.header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(r.body.Bytes())),
	}
}

func writeTestConfig(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
