package cmd

import (
	"fmt"
	"os"

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
				return nil
			},
		},
		&cobra.Command{
			Use:   "off",
			Short: application.T("cmd.boot.off.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
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
