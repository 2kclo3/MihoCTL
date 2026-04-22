package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
)

func newConfigCommand(application *app.App) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: application.T("cmd.config.short"),
	}

	configCmd.AddCommand(
		&cobra.Command{
			Use:    "env-file",
			Short:  "print the managed shell env file path",
			Hidden: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(application.Paths.AppHome, "system-proxy.env"))
				return nil
			},
		},
		&cobra.Command{
			Use:   "set-lang <zh-CN|en-US>",
			Short: application.T("cmd.config.setlang.short"),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				lang := args[0]
				if !application.IsSupportedLanguage(lang) {
					return core.NewActionError("invalid_language", "err.language.invalid", nil, "err.language.suggestion", map[string]any{
						"lang": lang,
					}, nil)
				}
				application.Config.Language = lang
				application.Language = lang
				application.ReloadTranslator()
				if err := application.SaveConfig(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.setlang.success", map[string]any{
					"lang": lang,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "view",
			Short: application.T("cmd.config.view.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.config.view.header"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.language", map[string]any{
					"lang": application.Config.Language,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.repo", map[string]any{
					"repo": application.Config.Core.Repo,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.binary", map[string]any{
					"path": application.Config.Mihomo.BinaryPath,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.database_dir", map[string]any{
					"path": application.Config.Core.DatabaseDir,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.core_version", map[string]any{
					"version": fallbackValue(application.State.Core.Version, application.T("label.unknown")),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.core_asset", map[string]any{
					"name": fallbackValue(application.State.Core.AssetName, application.T("label.unknown")),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.core_source", map[string]any{
					"source": fallbackValue(application.State.Core.Source, application.T("label.unknown")),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.config_path", map[string]any{
					"path": application.Config.Mihomo.ConfigPath,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.controller", map[string]any{
					"addr": application.Config.Controller.Address,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.installed_at", map[string]any{
					"time": formatTime(application.State.Core.InstalledAt, application.T("label.unknown")),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.update_check", map[string]any{
					"value": application.BoolLabel(application.Config.Core.AutoCheckUpdates),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.latest_version", map[string]any{
					"version": fallbackValue(application.State.Core.LatestVersion, application.T("label.unknown")),
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.config.view.subscriptions", map[string]any{
					"count": len(application.Config.Subscriptions),
				}))
				return nil
			},
		},
	)

	return configCmd
}

func formatTime(value time.Time, fallback string) string {
	if value.IsZero() {
		return fallback
	}
	return value.Format(time.RFC3339)
}
