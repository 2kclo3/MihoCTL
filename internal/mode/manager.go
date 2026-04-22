package mode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
	"mihoctl/internal/i18n"
	"mihoctl/internal/mihomo"
	"mihoctl/internal/state"
)

type Manager struct {
	paths  config.Paths
	cfg    *config.Config
	state  *state.State
	client *mihomo.Client
}

func NewManager(paths config.Paths, cfg *config.Config, st *state.State, client *mihomo.Client) *Manager {
	return &Manager{paths: paths, cfg: cfg, state: st, client: client}
}

func (m *Manager) EnsureActiveConfig() error {
	return m.ensureActiveConfig()
}

func (m *Manager) SetTun(enabled bool) error {
	if err := m.ensureActiveConfig(); err != nil {
		if !enabled && isConfigMissingError(err) {
			if runtimeErr := m.setTunRuntimeOnly(false); runtimeErr == nil {
				return nil
			}
		}
		m.recordToggle(&m.state.Modes.TUN, false, "config", err.Error())
		return err
	}
	if err := upsertTunConfigFile(m.cfg.Mihomo.ConfigPath, enabled); err != nil {
		m.recordToggle(&m.state.Modes.TUN, false, "config", err.Error())
		return core.NewActionError("set_tun_failed", "err.mode.set_tun", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.client.ReloadConfig(ctx, m.cfg.Mihomo.ConfigPath); err != nil {
		if !m.controllerReachable() {
			m.recordToggle(&m.state.Modes.TUN, enabled, "config", "")
			return nil
		}
		m.recordToggle(&m.state.Modes.TUN, enabled, "config", err.Error())
		return core.NewActionError("set_tun_failed", "err.mode.set_tun", err, "err.http.check_controller", map[string]any{
			"addr": m.cfg.Controller.Address,
		}, nil)
	}

	actual, err := m.client.GetConfig(ctx)
	if err == nil && actual.Tun.Enable != enabled {
		m.recordToggle(&m.state.Modes.TUN, actual.Tun.Enable, "api", "")
		return core.NewActionError("set_tun_not_applied", "err.mode.set_tun_not_applied", fmt.Errorf("requested=%t actual=%t", enabled, actual.Tun.Enable), "err.mode.tun_permission", map[string]any{
			"expected": enabled,
			"actual":   actual.Tun.Enable,
		}, m.tunPermissionHintData())
	}

	m.recordToggle(&m.state.Modes.TUN, enabled, "config+api", "")
	return nil
}

func (m *Manager) setTunRuntimeOnly(enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.client.SetTun(ctx, enabled); err != nil {
		m.recordToggle(&m.state.Modes.TUN, false, "api", err.Error())
		return core.NewActionError("set_tun_failed", "err.mode.set_tun", err, "err.http.check_controller", map[string]any{
			"addr": m.cfg.Controller.Address,
		}, nil)
	}

	actual, err := m.client.GetConfig(ctx)
	if err == nil && actual.Tun.Enable != enabled {
		m.recordToggle(&m.state.Modes.TUN, actual.Tun.Enable, "api", "")
		return core.NewActionError("set_tun_not_applied", "err.mode.set_tun_not_applied", fmt.Errorf("requested=%t actual=%t", enabled, actual.Tun.Enable), "err.mode.tun_permission", map[string]any{
			"expected": enabled,
			"actual":   actual.Tun.Enable,
		}, m.tunPermissionHintData())
	}

	m.recordToggle(&m.state.Modes.TUN, enabled, "api", "")
	return nil
}

func (m *Manager) TunStatus() (state.ToggleState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := m.client.GetConfig(ctx)
	if err != nil {
		enabled, fileErr := readTunEnabledFromConfigFile(m.cfg.Mihomo.ConfigPath)
		if fileErr == nil && enabled != nil {
			m.recordToggle(&m.state.Modes.TUN, *enabled, "config", "")
			return m.state.Modes.TUN, nil
		}
		return m.state.Modes.TUN, core.NewActionError("tun_status_failed", "err.mode.status_tun", err, "err.http.check_controller", map[string]any{
			"addr": m.cfg.Controller.Address,
		}, nil)
	}

	if desired, err := m.TunConfiguredState(); err == nil && desired != nil && *desired != cfg.Tun.Enable {
		m.recordToggle(&m.state.Modes.TUN, cfg.Tun.Enable, "api", "tun_runtime_mismatch")
		return m.state.Modes.TUN, nil
	}

	m.recordToggle(&m.state.Modes.TUN, cfg.Tun.Enable, "api", "")
	return m.state.Modes.TUN, nil
}

func (m *Manager) TunConfiguredState() (*bool, error) {
	enabled, err := readTunEnabledFromConfigFile(m.cfg.Mihomo.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, core.NewActionError("tun_config_read_failed", "err.mode.status_tun", err, "err.config.check_path", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}
	return enabled, nil
}

func (m *Manager) ModeEnabled(selected string) (bool, error) {
	switch ResolveMode(selected) {
	case ModeEnv:
		status, err := m.SystemProxyStatus()
		if err != nil {
			return false, err
		}
		return status.Known && status.Enabled, nil
	default:
		status, err := m.TunStatus()
		if err != nil {
			return false, err
		}
		return status.Known && status.Enabled, nil
	}
}

func (m *Manager) ApplyMode(selected string, enabled bool) error {
	switch ResolveMode(selected) {
	case ModeEnv:
		return m.SetSystemProxy(enabled)
	default:
		return m.SetTun(enabled)
	}
}

func (m *Manager) SetSystemProxy(enabled bool) error {
	switch runtime.GOOS {
	case "darwin", "linux":
		if err := m.applyLinuxEnvSystemProxy(enabled); err != nil {
			m.recordToggle(&m.state.Modes.SystemProxy, false, "env", err.Error())
			return err
		}
		m.recordToggle(&m.state.Modes.SystemProxy, enabled, "env", "")
		return nil
	default:
		err := core.NewActionError("system_proxy_unsupported", "err.service.unsupported", nil, "", nil, nil)
		m.recordToggle(&m.state.Modes.SystemProxy, false, "manual", err.Error())
		return err
	}
}

func (m *Manager) SystemProxyStatus() (state.ToggleState, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		enabled, err := m.linuxEnvSystemProxyStatus()
		if err != nil {
			return m.state.Modes.SystemProxy, err
		}
		if enabled == nil {
			return m.state.Modes.SystemProxy, nil
		}
		m.recordToggle(&m.state.Modes.SystemProxy, *enabled, "env", "")
		return m.state.Modes.SystemProxy, nil
	default:
		return m.state.Modes.SystemProxy, core.NewActionError("system_proxy_unsupported", "err.service.unsupported", nil, "", nil, nil)
	}
}

func (m *Manager) applyDarwinSystemProxy(serviceName string, enabled bool) error {
	host := m.cfg.SystemProxy.Host
	port := strconv.Itoa(m.cfg.SystemProxy.Port)

	commands := [][]string{
		{"-setwebproxystate", serviceName, yesNo(enabled)},
		{"-setsecurewebproxystate", serviceName, yesNo(enabled)},
		{"-setsocksfirewallproxystate", serviceName, yesNo(enabled)},
	}

	if enabled {
		commands = append([][]string{
			{"-setwebproxy", serviceName, host, port},
			{"-setsecurewebproxy", serviceName, host, port},
			{"-setsocksfirewallproxy", serviceName, host, port},
		}, commands...)
	}

	for _, args := range commands {
		if err := runCommand("networksetup", args...); err != nil {
			return core.NewActionError("system_proxy_set_failed", "err.mode.set_proxy", err, "err.service.need_root", nil, nil)
		}
	}
	return nil
}

func (m *Manager) resolveDarwinService() (string, error) {
	if m.cfg.SystemProxy.ServiceName != "" {
		return m.cfg.SystemProxy.ServiceName, nil
	}

	output, err := runOutput("networksetup", "-listallnetworkservices")
	if err != nil {
		return "", core.NewActionError("system_proxy_service_failed", "err.mode.status_proxy", err, "", nil, nil)
	}

	lines := strings.Split(output, "\n")
	services := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") || strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	if len(services) == 1 {
		return services[0], nil
	}
	return "", core.NewActionError("multiple_services", "err.mode.multiple_services", errors.New(strings.Join(services, ", ")), "err.mode.choose_service", nil, nil)
}

func (m *Manager) recordToggle(target *state.ToggleState, enabled bool, source, lastErr string) {
	target.Known = true
	target.Enabled = enabled
	target.Source = source
	target.LastError = lastErr
	target.UpdatedAt = time.Now()
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func yesNo(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func (m *Manager) ensureActiveConfig() error {
	if _, err := os.Stat(m.cfg.Mihomo.ConfigPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return core.NewActionError("process_config_missing", "err.process.config_missing", err, "err.process.check_config", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}

	source, err := m.defaultSubscriptionConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.Mihomo.ConfigPath), 0o755); err != nil {
		return core.NewActionError("mkdir_failed", "err.path.create_app_home", err, "err.path.check_permission", map[string]any{
			"path": filepath.Dir(m.cfg.Mihomo.ConfigPath),
		}, nil)
	}
	if err := copyFileContents(source, m.cfg.Mihomo.ConfigPath); err != nil {
		return core.NewActionError("subscription_activate_failed", "err.subscription.activate", err, "err.path.check_permission", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}
	return nil
}

func (m *Manager) defaultSubscriptionConfig() (string, error) {
	if len(m.cfg.Subscriptions) == 0 {
		return "", core.NewActionError("process_config_missing", "err.process.config_missing", os.ErrNotExist, "err.process.add_subscription", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}
	if strings.TrimSpace(m.cfg.DefaultSubscription) == "" {
		return "", core.NewActionError("process_config_missing", "err.process.config_missing", os.ErrNotExist, "err.process.choose_subscription", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}

	for _, item := range m.cfg.Subscriptions {
		if item.Name != m.cfg.DefaultSubscription && item.URL != m.cfg.DefaultSubscription {
			continue
		}
		if _, err := os.Stat(item.ConfigPath); err != nil {
			return "", core.NewActionError("process_config_missing", "err.process.config_missing", err, "err.process.update_subscription", map[string]any{
				"path": m.cfg.Mihomo.ConfigPath,
			}, nil)
		}
		return item.ConfigPath, nil
	}

	return "", core.NewActionError("process_config_missing", "err.process.config_missing", os.ErrNotExist, "err.process.update_subscription", map[string]any{
		"path": m.cfg.Mihomo.ConfigPath,
	}, nil)
}

func (m *Manager) controllerReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := m.client.Ping(ctx); err == nil {
		return true
	}
	pid := m.state.Process.PID
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func copyFileContents(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	return target.Close()
}

func (m *Manager) fallbackLogPath() string {
	if strings.TrimSpace(m.state.Process.LogFile) != "" {
		return m.state.Process.LogFile
	}
	return m.paths.LogFile
}

func (m *Manager) tunPermissionHintData() map[string]any {
	binary := strings.TrimSpace(m.cfg.Mihomo.BinaryPath)
	if binary == "" {
		binary = "mihomo"
	}
	helperPath := filepath.Join(strings.TrimSpace(m.paths.ExecDir), "mihoctl-enable-tun")

	return map[string]any{
		"log":   m.fallbackLogPath(),
		"steps": m.tunPermissionSteps(binary, helperPath),
	}
}

func (m *Manager) tunPermissionSteps(binaryPath, helperPath string) string {
	binaryPath = shellQuote(binaryPath)
	logPath := shellQuote(m.fallbackLogPath())
	helperExists := helperPath != "" && fileExists(helperPath)
	alreadyAdmin := os.Geteuid() == 0
	if helperExists {
		helperPath = shellQuote(helperPath)
	}

	tr := i18n.New(m.cfg.Language)
	if runtime.GOOS == "linux" {
		if !tunDeviceAvailable() {
			return tr.Tf("msg.mode.tun.permission.device_missing", map[string]any{
				"log": logPath,
			})
		}
		if !iptablesAvailable() {
			return tr.Tf("msg.mode.tun.permission.iptables_missing", map[string]any{
				"log": logPath,
			})
		}
	}
	if helperExists && (runtime.GOOS == "linux" || runtime.GOOS == "darwin") {
		if alreadyAdmin {
			return tr.Tf("msg.mode.tun.permission.helper.admin", map[string]any{
				"helper": helperPath,
				"log":    logPath,
			})
		}
		return tr.Tf("msg.mode.tun.permission.helper.sudo", map[string]any{
			"helper": helperPath,
			"log":    logPath,
		})
	}

	switch runtime.GOOS {
	case "linux":
		if alreadyAdmin {
			return tr.Tf("msg.mode.tun.permission.setcap.admin", map[string]any{
				"binary": binaryPath,
				"log":    logPath,
			})
		}
		return tr.Tf("msg.mode.tun.permission.setcap.sudo", map[string]any{
			"binary": binaryPath,
			"log":    logPath,
		})
	case "darwin":
		if alreadyAdmin {
			return tr.Tf("msg.mode.tun.permission.restart_on.admin", map[string]any{
				"log": logPath,
			})
		}
		return tr.Tf("msg.mode.tun.permission.restart_on.sudo", map[string]any{
			"log": logPath,
		})
	default:
		if alreadyAdmin {
			return tr.Tf("msg.mode.tun.permission.restart_only.admin", map[string]any{
				"log": logPath,
			})
		}
		return tr.Tf("msg.mode.tun.permission.restart_only.sudo", map[string]any{
			"log": logPath,
		})
	}
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func isConfigMissingError(err error) bool {
	var actionErr *core.ActionError
	if !errors.As(err, &actionErr) {
		return false
	}
	return actionErr.Code == "process_config_missing"
}
