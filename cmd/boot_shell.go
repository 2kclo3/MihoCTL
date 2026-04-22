package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
)

const (
	bootShellManagedStart = "# >>> mihoctl shell boot >>>"
	bootShellManagedEnd   = "# <<< mihoctl shell boot <<<"
)

func newBootShellCommand(application *app.App) *cobra.Command {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: application.T("cmd.boot.shell.short"),
	}

	shellCmd.AddCommand(
		&cobra.Command{
			Use:   "on",
			Short: application.T("cmd.boot.shell.on.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				files, err := installBootShell(application)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.shell.on.success"))
				for _, file := range files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.shell.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.shell.on.hint", map[string]any{
					"command": envSyncCommand(application),
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "off",
			Short: application.T("cmd.boot.shell.off.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				files, err := uninstallBootShell(application)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.shell.off.success"))
				for _, file := range files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.shell.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: application.T("cmd.boot.shell.status.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := bootShellStatus(application)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.shell.status.header"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.shell.status.enabled", map[string]any{
					"value": application.BoolLabel(status.Enabled),
				}))
				for _, file := range status.Files {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.shell.file", map[string]any{
						"path": displayHomePath(file),
					}))
				}
				return nil
			},
		},
	)

	return shellCmd
}

type shellBootStatus struct {
	Enabled bool
	Files   []string
}

func installBootShell(application *app.App) ([]string, error) {
	files, err := bootShellTargetFiles()
	if err != nil {
		return nil, err
	}

	block := renderBootShellBlock(application)
	written := make([]string, 0, len(files))
	for _, path := range files {
		if err := upsertBootManagedShellBlock(path, block); err != nil {
			return nil, core.NewActionError("boot_shell_write_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
				"path": path,
			}, nil)
		}
		written = append(written, path)
	}
	return written, nil
}

func uninstallBootShell(application *app.App) ([]string, error) {
	files, err := bootShellTargetFiles()
	if err != nil {
		return nil, err
	}

	updated := make([]string, 0, len(files))
	for _, path := range files {
		changed, err := removeBootManagedShellBlock(path)
		if err != nil {
			return nil, core.NewActionError("boot_shell_write_failed", "err.mode.set_proxy", err, "err.path.check_permission", map[string]any{
				"path": path,
			}, nil)
		}
		if changed {
			updated = append(updated, path)
		}
	}
	return updated, nil
}

func bootShellStatus(application *app.App) (*shellBootStatus, error) {
	files, err := bootShellTargetFiles()
	if err != nil {
		return nil, err
	}

	status := &shellBootStatus{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, core.NewActionError("boot_shell_read_failed", "err.config.read", err, "err.config.check_path", map[string]any{
				"path": path,
			}, nil)
		}
		if strings.Contains(string(data), bootShellManagedStart) {
			status.Enabled = true
			status.Files = append(status.Files, path)
		}
	}
	return status, nil
}

func renderBootShellBlock(application *app.App) string {
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
		bootShellManagedStart,
		fmt.Sprintf("if command -v pgrep >/dev/null 2>&1 && pgrep -f %s >/dev/null 2>&1; then", shellQuote(mihomoPath)),
		"  :",
		"else",
		fmt.Sprintf("  (%s) &", startCommand),
		"fi",
		bootShellManagedEnd,
		"",
	}
	return strings.Join(lines, "\n")
}

func bootShellTargetFiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, core.NewActionError("boot_shell_home_failed", "err.mode.set_proxy", err, "", nil, nil)
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
	start := strings.Index(content, bootShellManagedStart)
	end := strings.Index(content, bootShellManagedEnd)
	switch {
	case start >= 0 && end >= start:
		end += len(bootShellManagedEnd)
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
	start := strings.Index(content, bootShellManagedStart)
	end := strings.Index(content, bootShellManagedEnd)
	if start < 0 || end < start {
		return false, nil
	}
	end += len(bootShellManagedEnd)
	updated := content[:start] + content[end:]
	updated = strings.TrimLeft(updated, "\n")
	updated = strings.ReplaceAll(updated, "\n\n\n", "\n\n")
	return true, os.WriteFile(path, []byte(updated), 0o644)
}
