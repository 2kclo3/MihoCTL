package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
)

const (
	bootTmuxManagedStart = "# >>> mihoctl tmux boot >>>"
	bootTmuxManagedEnd   = "# <<< mihoctl tmux boot <<<"
	bootTmuxSessionName  = "mihoctl-autostart"
)

func newBootTmuxCommand(application *app.App) *cobra.Command {
	tmuxCmd := &cobra.Command{
		Use:   "tmux",
		Short: application.T("cmd.boot.tmux.short"),
	}

	tmuxCmd.AddCommand(
		&cobra.Command{
			Use:   "on",
			Short: application.T("cmd.boot.tmux.on.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				if _, err := exec.LookPath("tmux"); err != nil {
					return core.NewActionError("boot_tmux_missing", "err.boot.tmux_missing", err, "err.boot.tmux_install", nil, nil)
				}
				files, err := installBootTmux(application)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.tmux.on.success"))
				for _, file := range files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.on.hint", map[string]any{
					"command": envSyncCommand(application),
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "off",
			Short: application.T("cmd.boot.tmux.off.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				files, err := uninstallBootTmux(application)
				if err != nil {
					return err
				}
				killBootTmuxSession()
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.tmux.off.success"))
				for _, file := range files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: application.T("cmd.boot.tmux.status.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := bootTmuxStatus(application)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.tmux.status.header"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.status.enabled", map[string]any{
					"value": application.BoolLabel(status.Enabled),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.status.session", map[string]any{
					"value": application.BoolLabel(status.SessionExists),
				}))
				for _, file := range status.Files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.tmux.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				return nil
			},
		},
	)

	return tmuxCmd
}

type tmuxBootStatus struct {
	Enabled       bool
	SessionExists bool
	Files         []string
}

func installBootTmux(application *app.App) ([]string, error) {
	files, err := bootTmuxTargetFiles()
	if err != nil {
		return nil, err
	}

	block := renderBootTmuxBlock(application)
	written := make([]string, 0, len(files))
	for _, path := range files {
		if err := upsertBootManagedShellBlock(path, block); err != nil {
			return nil, core.NewActionError("boot_tmux_write_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
				"path": path,
			}, nil)
		}
		written = append(written, path)
	}
	return written, nil
}

func uninstallBootTmux(application *app.App) ([]string, error) {
	files, err := bootTmuxTargetFiles()
	if err != nil {
		return nil, err
	}

	updated := make([]string, 0, len(files))
	for _, path := range files {
		changed, err := removeBootManagedShellBlock(path)
		if err != nil {
			return nil, core.NewActionError("boot_tmux_write_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
				"path": path,
			}, nil)
		}
		if changed {
			updated = append(updated, path)
		}
	}
	return updated, nil
}

func bootTmuxStatus(application *app.App) (*tmuxBootStatus, error) {
	files, err := bootTmuxTargetFiles()
	if err != nil {
		return nil, err
	}

	status := &tmuxBootStatus{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, core.NewActionError("boot_tmux_read_failed", "err.config.read", err, "err.config.check_path", map[string]any{
				"path": path,
			}, nil)
		}
		if strings.Contains(string(data), bootTmuxManagedStart) {
			status.Enabled = true
			status.Files = append(status.Files, path)
		}
	}
	status.SessionExists = tmuxSessionExists(bootTmuxSessionName)
	return status, nil
}

func renderBootTmuxBlock(application *app.App) string {
	executable := strings.TrimSpace(application.Paths.ExecPath)
	if executable == "" {
		executable = "mihoctl"
	}
	startCommand := strings.Join([]string{
		shellQuote(executable),
		"--config",
		shellQuote(application.Paths.ConfigFile),
		"start",
		">/dev/null 2>&1; exec ${SHELL:-/bin/sh} -l",
	}, " ")
	mihomoPath := strings.TrimSpace(application.Config.Mihomo.BinaryPath)
	if mihomoPath == "" {
		mihomoPath = "mihomo"
	}

	lines := []string{
		bootTmuxManagedStart,
		"if command -v tmux >/dev/null 2>&1; then",
		fmt.Sprintf("  if command -v pgrep >/dev/null 2>&1 && pgrep -f %s >/dev/null 2>&1; then", shellQuote(mihomoPath)),
		"    :",
		fmt.Sprintf("  elif ! tmux has-session -t %s 2>/dev/null; then", shellQuote(bootTmuxSessionName)),
		fmt.Sprintf("    tmux new-session -d -s %s %s", shellQuote(bootTmuxSessionName), shellQuote(startCommand)),
		"  fi",
		"fi",
		bootTmuxManagedEnd,
		"",
	}
	return strings.Join(lines, "\n")
}

func bootTmuxTargetFiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, core.NewActionError("boot_tmux_home_failed", "err.mode.set_proxy", err, "", nil, nil)
	}
	return []string{
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}, nil
}

func upsertBootManagedShellBlock(path, block string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)
	start := strings.Index(content, bootTmuxManagedStart)
	end := strings.Index(content, bootTmuxManagedEnd)
	switch {
	case start >= 0 && end >= start:
		end += len(bootTmuxManagedEnd)
		content = content[:start] + strings.TrimLeft(block, "\n") + content[end:]
	default:
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += block
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeBootManagedShellBlock(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	content := string(data)
	start := strings.Index(content, bootTmuxManagedStart)
	end := strings.Index(content, bootTmuxManagedEnd)
	if start < 0 || end < start {
		return false, nil
	}
	end += len(bootTmuxManagedEnd)
	updated := content[:start] + content[end:]
	updated = strings.TrimLeft(updated, "\n")
	updated = strings.ReplaceAll(updated, "\n\n\n", "\n\n")
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

func tmuxSessionExists(name string) bool {
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func killBootTmuxSession() {
	if !tmuxSessionExists(bootTmuxSessionName) {
		return
	}
	_ = exec.Command("tmux", "kill-session", "-t", bootTmuxSessionName).Run()
}
