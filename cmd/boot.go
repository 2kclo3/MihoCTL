package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
	"mihoctl/internal/mode"
	"mihoctl/internal/service"
)

func newBootCommand(application *app.App) *cobra.Command {
	bootCmd := &cobra.Command{
		Use:   "boot",
		Short: application.T("cmd.boot.short"),
	}

	bootCmd.AddCommand(
		&cobra.Command{
			Use:   "on",
			Short: application.T("cmd.boot.on.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				current, err := bootServiceStatus(application)
				if err != nil {
					return err
				}
				if current != nil && (current.Enabled || current.Registered) {
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.on.already"))
					if current.Path != "" {
						fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.status.path", map[string]any{
							"path": current.Path,
						}))
					}
					if shellStatus, err := bootShellStatus(application); err == nil && shellStatus.Enabled {
						if _, err := uninstallBootShell(application); err == nil {
							fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.shell.disabled_by_boot"))
						}
					}
					return nil
				}
				if err := requireBootPrivileges(application, "on"); err != nil {
					return err
				}
				if err := ensureBootReady(application); err != nil {
					return err
				}

				status, err := service.NewManager(application.Config).Enable()
				if err != nil {
					return err
				}

				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.on.success"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.on.detail", map[string]any{
					"path": status.Path,
				}))
				if shellStatus, err := bootShellStatus(application); err == nil && shellStatus.Enabled {
					if _, err := uninstallBootShell(application); err == nil {
						fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.shell.disabled_by_boot"))
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "off",
			Short: application.T("cmd.boot.off.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				current, err := bootServiceStatus(application)
				if err != nil {
					return err
				}
				if current != nil && !current.Enabled && !current.Registered {
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.off.already"))
					return nil
				}
				if err := requireBootPrivileges(application, "off"); err != nil {
					return err
				}
				if err := service.NewManager(application.Config).Disable(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.off.success"))
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: application.T("cmd.boot.status.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := service.NewManager(application.Config).Status()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.boot.status.header"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.status.enabled", map[string]any{
					"value": application.BoolLabel(status.Enabled || status.Registered),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.status.active", map[string]any{
					"value": application.BoolLabel(status.Active),
				}))
				if status.Path != "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.boot.status.path", map[string]any{
						"path": status.Path,
					}))
				}
				return nil
			},
		},
		newBootShellCommand(application),
	)

	return bootCmd
}

func requireBootPrivileges(application *app.App, action string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	return core.NewActionError("boot_need_root", "err.boot.need_root", nil, "err.boot.need_root_hint", nil, map[string]any{
		"command": bootElevatedCommand(application, action),
	})
}

func ensureBootReady(application *app.App) error {
	if _, err := os.Stat(application.Config.Mihomo.BinaryPath); err != nil {
		return core.NewActionError("boot_binary_missing", "err.process.binary_not_found", err, "err.process.install_core", map[string]any{
			"path": application.Config.Mihomo.BinaryPath,
		}, nil)
	}
	return mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient()).EnsureActiveConfig()
}

func bootServiceStatus(application *app.App) (*service.Status, error) {
	if runtime.GOOS == "linux" && !service.LinuxSystemdAvailable() {
		return nil, core.NewActionError("service_systemd_unavailable", "err.service.systemd_unavailable", nil, "err.service.systemd_unavailable_hint", nil, map[string]any{
			"command": "mihoctl boot shell on",
		})
	}
	return service.NewManager(application.Config).Status()
}
