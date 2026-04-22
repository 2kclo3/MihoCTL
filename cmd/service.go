package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/service"
)

func newServiceCommand(application *app.App) *cobra.Command {
	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: application.T("cmd.service.short"),
	}

	serviceCmd.AddCommand(
		&cobra.Command{
			Use:   "enable",
			Short: application.T("cmd.service.enable.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := service.NewManager(application.Config)
				status, err := manager.Enable()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.service.enable.success", map[string]any{
					"path": status.Path,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "disable",
			Short: application.T("cmd.service.disable.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := service.NewManager(application.Config)
				if err := manager.Disable(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.service.disable.success"))
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: application.T("cmd.service.status.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := service.NewManager(application.Config)
				status, err := manager.Status()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.service.status.header"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.service.status.registered", map[string]any{
					"value": application.BoolLabel(status.Registered),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.service.status.enabled", map[string]any{
					"value": application.BoolLabel(status.Enabled),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.service.status.active", map[string]any{
					"value": application.BoolLabel(status.Active),
				}))
				if status.Path != "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.service.status.path", map[string]any{
						"path": status.Path,
					}))
				}
				return nil
			},
		},
	)

	return serviceCmd
}
