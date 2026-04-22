package process

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
	"mihoctl/internal/state"
)

type Manager struct {
	cfg   *config.Config
	state *state.State
	paths config.Paths
}

type Status struct {
	Running bool
	PID     int
	Uptime  time.Duration
}

func NewManager(cfg *config.Config, st *state.State, paths config.Paths) *Manager {
	return &Manager{cfg: cfg, state: st, paths: paths}
}

func (m *Manager) Start() (*state.ProcessState, error) {
	current, err := m.Status()
	if err != nil {
		return nil, err
	}
	if current.Running {
		return nil, core.NewActionError("process_already_running", "err.process.already_running", nil, "err.process.cleanup_pid", nil, nil)
	}

	binary, err := exec.LookPath(m.cfg.Mihomo.BinaryPath)
	if err != nil {
		return nil, core.NewActionError("process_binary_not_found", "err.process.binary_not_found", err, "err.process.install_core", map[string]any{
			"path": m.cfg.Mihomo.BinaryPath,
		}, nil)
	}

	if _, err := os.Stat(m.cfg.Mihomo.ConfigPath); err != nil {
		if len(m.cfg.Subscriptions) == 0 {
			return nil, core.NewActionError("process_config_missing", "err.process.config_missing", err, "err.process.add_subscription", map[string]any{
				"path": m.cfg.Mihomo.ConfigPath,
			}, nil)
		}
		if m.cfg.DefaultSubscription == "" {
			return nil, core.NewActionError("process_config_missing", "err.process.config_missing", err, "err.process.choose_subscription", map[string]any{
				"path": m.cfg.Mihomo.ConfigPath,
			}, nil)
		}
		return nil, core.NewActionError("process_config_missing", "err.process.config_missing", err, "err.process.update_subscription", map[string]any{
			"path": m.cfg.Mihomo.ConfigPath,
		}, nil)
	}
	if err := os.MkdirAll(filepath.Dir(m.paths.LogFile), 0o755); err != nil {
		return nil, err
	}

	// 后台进程启动时直接将输出重定向到日志，方便无头环境排障。
	logFile, err := os.OpenFile(m.paths.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, core.NewActionError("process_log_open_failed", "err.process.start_failed", err, "err.process.check_log", map[string]any{
			"path": m.paths.LogFile,
		}, nil)
	}
	defer logFile.Close()

	cmd := exec.Command(binary, "-d", m.cfg.Mihomo.WorkDir, "-f", m.cfg.Mihomo.ConfigPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	prepareDetachedCommand(cmd)

	if err := cmd.Start(); err != nil {
		return nil, core.NewActionError("process_start_failed", "err.process.start_failed", err, "", nil, nil)
	}

	m.state.Process = state.ProcessState{
		PID:       cmd.Process.Pid,
		StartedAt: time.Now(),
		LogFile:   m.paths.LogFile,
	}
	return &m.state.Process, nil
}

func (m *Manager) Stop() error {
	if m.state.Process.PID == 0 {
		return core.NewActionError("process_not_running", "err.process.not_running", nil, "", nil, nil)
	}
	if err := stopPID(m.state.Process.PID); err != nil {
		return core.NewActionError("process_stop_failed", "err.process.stop_failed", err, "", nil, nil)
	}
	m.state.Process = state.ProcessState{}
	return nil
}

func (m *Manager) Restart() (*state.ProcessState, error) {
	if m.state.Process.PID != 0 && isRunning(m.state.Process.PID) {
		if err := m.Stop(); err != nil {
			return nil, err
		}
	}
	return m.Start()
}

func (m *Manager) Status() (*Status, error) {
	pid := m.state.Process.PID
	if pid == 0 {
		return &Status{}, nil
	}
	if !isRunning(pid) {
		m.state.Process = state.ProcessState{}
		return &Status{}, nil
	}

	uptime := time.Duration(0)
	if !m.state.Process.StartedAt.IsZero() {
		uptime = time.Since(m.state.Process.StartedAt)
	}
	return &Status{
		Running: true,
		PID:     pid,
		Uptime:  uptime,
	}, nil
}

func isRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func stopPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isRunning(pid) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}
