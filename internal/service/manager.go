package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
)

const (
	systemdUnitPath = "/etc/systemd/system/mihomo.service"
	launchdPlist    = "/Library/LaunchDaemons/com.mihoctl.mihomo.plist"
)

type Manager struct {
	cfg *config.Config
}

type Status struct {
	Registered bool
	Enabled    bool
	Active     bool
	Path       string
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Enable() (*Status, error) {
	switch runtime.GOOS {
	case "linux":
		if !LinuxSystemdAvailable() {
			return nil, core.NewActionError("service_systemd_unavailable", "err.service.systemd_unavailable", nil, "err.service.systemd_unavailable_hint", nil, map[string]any{
				"command": "mihoctl boot shell on",
			})
		}
		if err := os.WriteFile(systemdUnitPath, []byte(m.systemdUnit()), 0o644); err != nil {
			return nil, core.NewActionError("service_enable_failed", "err.service.enable", err, "", nil, nil)
		}
		if err := runCommand("systemctl", "daemon-reload"); err != nil {
			return nil, core.NewActionError("service_enable_failed", "err.service.enable", err, "", nil, nil)
		}
		if err := runCommand("systemctl", "enable", "--now", "mihomo"); err != nil {
			return nil, core.NewActionError("service_enable_failed", "err.service.enable", err, "", nil, nil)
		}
		return &Status{Registered: true, Enabled: true, Active: true, Path: systemdUnitPath}, nil
	case "darwin":
		if err := os.WriteFile(launchdPlist, []byte(m.launchdPlist()), 0o644); err != nil {
			return nil, core.NewActionError("service_enable_failed", "err.service.enable", err, "", nil, nil)
		}
		if err := runCommand("launchctl", "bootstrap", "system", launchdPlist); err != nil {
			return nil, core.NewActionError("service_enable_failed", "err.service.enable", err, "", nil, nil)
		}
		return &Status{Registered: true, Enabled: true, Active: true, Path: launchdPlist}, nil
	default:
		return nil, core.NewActionError("service_unsupported", "err.service.unsupported", nil, "", nil, nil)
	}
}

func (m *Manager) Disable() error {
	switch runtime.GOOS {
	case "linux":
		if !LinuxSystemdAvailable() {
			return core.NewActionError("service_systemd_unavailable", "err.service.systemd_unavailable", nil, "err.service.systemd_unavailable_hint", nil, map[string]any{
				"command": "mihoctl boot shell on",
			})
		}
		_ = runCommand("systemctl", "disable", "--now", "mihomo")
		if err := os.Remove(systemdUnitPath); err != nil && !os.IsNotExist(err) {
			return core.NewActionError("service_disable_failed", "err.service.disable", err, "", nil, nil)
		}
		_ = runCommand("systemctl", "daemon-reload")
		return nil
	case "darwin":
		_ = runCommand("launchctl", "bootout", "system", launchdPlist)
		if err := os.Remove(launchdPlist); err != nil && !os.IsNotExist(err) {
			return core.NewActionError("service_disable_failed", "err.service.disable", err, "", nil, nil)
		}
		return nil
	default:
		return core.NewActionError("service_unsupported", "err.service.unsupported", nil, "", nil, nil)
	}
}

func (m *Manager) Status() (*Status, error) {
	switch runtime.GOOS {
	case "linux":
		if !LinuxSystemdAvailable() {
			return nil, core.NewActionError("service_systemd_unavailable", "err.service.systemd_unavailable", nil, "err.service.systemd_unavailable_hint", nil, map[string]any{
				"command": "mihoctl boot shell on",
			})
		}
		status := &Status{Path: systemdUnitPath}
		if _, err := os.Stat(systemdUnitPath); err == nil {
			status.Registered = true
		}
		status.Enabled = runCommand("systemctl", "is-enabled", "mihomo") == nil
		status.Active = runCommand("systemctl", "is-active", "mihomo") == nil
		return status, nil
	case "darwin":
		status := &Status{Path: launchdPlist}
		if _, err := os.Stat(launchdPlist); err == nil {
			status.Registered = true
			status.Enabled = true
		}
		status.Active = runCommand("launchctl", "print", "system/com.mihoctl.mihomo") == nil
		return status, nil
	default:
		return nil, core.NewActionError("service_unsupported", "err.service.unsupported", nil, "", nil, nil)
	}
}

func (m *Manager) systemdUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Mihomo managed by MihoCTL
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s -d %s -f %s
WorkingDirectory=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, m.cfg.Mihomo.BinaryPath, m.cfg.Mihomo.WorkDir, m.cfg.Mihomo.ConfigPath, m.cfg.Mihomo.WorkDir)
}

func (m *Manager) launchdPlist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.mihoctl.mihomo</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>-d</string>
    <string>%s</string>
    <string>-f</string>
    <string>%s</string>
  </array>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, m.cfg.Mihomo.BinaryPath, m.cfg.Mihomo.WorkDir, m.cfg.Mihomo.ConfigPath, m.cfg.Mihomo.WorkDir, filepath.Join(os.TempDir(), "mihomo.stdout.log"), filepath.Join(os.TempDir(), "mihomo.stderr.log"))
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() == 0 {
			return err
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func LinuxSystemdAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "systemd"
}
