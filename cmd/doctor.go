package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/mode"
)

func newDoctorCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: application.T("cmd.doctor.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.doctor.header"))

			_, binaryErr := os.Stat(application.Config.Mihomo.BinaryPath)
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   application.T("msg.doctor.binary"),
				"status": doctorStatusLabel(binaryErr == nil, application),
			}))

			_, configErr := os.Stat(application.Config.Mihomo.ConfigPath)
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   application.T("msg.doctor.config"),
				"status": doctorStatusLabel(configErr == nil, application),
			}))

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, apiErr := application.MihomoClient().Ping(ctx)
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   application.T("msg.doctor.api"),
				"status": doctorStatusLabel(apiErr == nil, application),
			}))
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   application.T("msg.doctor.boot"),
				"status": doctorBootStatusLabel(application),
			}))

			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
				"mode": mode.ResolveMode(application.Config.Mode),
			}))

			envStatus, _ := manager.SystemProxyStatus()
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   "ENV",
				"status": application.ToggleLabel(envStatus.Known, envStatus.Enabled),
			}))

			tunStatus, _ := manager.TunStatus()
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.doctor.item", map[string]any{
				"name":   "TUN",
				"status": application.ToggleLabel(tunStatus.Known, tunStatus.Enabled),
			}))

			return nil
		},
	}
}

func doctorStatusLabel(ok bool, application *app.App) string {
	if ok {
		return application.T("label.ok")
	}
	return application.T("label.fail")
}
