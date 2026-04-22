package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/coremanager"
)

func newUpdateCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:       "update [all|core|sub]",
		Short:     application.T("cmd.update.short"),
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"all", "core", "sub"},
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) == 1 && args[0] != "" {
				target = args[0]
			}

			if target == "all" || target == "sub" {
				manager := subscriptionManagerFromCmd(application, cmd)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := manager.Update(ctx, "")
				if err != nil {
					return err
				}
				for _, success := range result.Successes {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.update.item_success", map[string]any{
						"name": success.Name,
						"url":  success.URL,
						"path": success.Path,
					}))
					if success.Reloaded {
						fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.sub.update.reloaded"))
					}
				}
				for _, failure := range result.Failures {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.update.item_fail", map[string]any{
						"name": failure.Name,
						"url":  failure.URL,
						"err":  application.FormatError(failure.Err),
					}))
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.update.summary", map[string]any{
					"success": len(result.Successes),
					"failed":  len(result.Failures),
				}))
				_ = application.SaveConfig()
			}

			if target == "all" || target == "core" {
				coreManager := newCoreManager(application, cmd)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				var (
					result *coremanager.InstallResult
					err    error
				)
				if application.State.Core.Version == "" {
					result, err = coreManager.Install(ctx, "")
				} else {
					result, err = coreManager.Upgrade(ctx, "")
				}
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				if err := application.SaveState(); err != nil {
					return err
				}
				if result.PreviousVersion == "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.install.success", map[string]any{
						"version": result.Version,
						"path":    result.BinaryPath,
					}))
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.upgrade.success", map[string]any{
						"from": result.PreviousVersion,
						"to":   result.Version,
						"path": result.BinaryPath,
					}))
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.asset", map[string]any{
					"name": result.AssetName,
				}))
			}

			return nil
		},
	}
}
