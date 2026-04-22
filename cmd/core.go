package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/coremanager"
)

func newCoreCommand(application *app.App) *cobra.Command {
	coreCmd := &cobra.Command{
		Use:   "core",
		Short: application.T("cmd.core.short"),
	}

	var installVersion string
	var upgradeVersion string

	coreCmd.AddCommand(
		func() *cobra.Command {
			cmd := &cobra.Command{
				Use:   "install",
				Short: application.T("cmd.core.install.short"),
				RunE: func(cmd *cobra.Command, args []string) error {
					manager := newCoreManager(application, cmd)
					result, err := manager.Install(cmd.Context(), installVersion)
					if err != nil {
						return err
					}
					if err := application.SaveConfig(); err != nil {
						return err
					}
					if err := application.SaveState(); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.install.success", map[string]any{
						"version": result.Version,
						"path":    result.BinaryPath,
					}))
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.asset", map[string]any{
						"name": result.AssetName,
					}))
					return nil
				},
			}
			cmd.Flags().StringVar(&installVersion, "version", "", application.T("flag.core.version"))
			return cmd
		}(),
		func() *cobra.Command {
			cmd := &cobra.Command{
				Use:   "upgrade",
				Short: application.T("cmd.core.upgrade.short"),
				RunE: func(cmd *cobra.Command, args []string) error {
					manager := newCoreManager(application, cmd)
					result, err := manager.Upgrade(cmd.Context(), upgradeVersion)
					if err != nil {
						return err
					}
					if err := application.SaveConfig(); err != nil {
						return err
					}
					if err := application.SaveState(); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.upgrade.success", map[string]any{
						"from": result.PreviousVersion,
						"to":   result.Version,
						"path": result.BinaryPath,
					}))
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.asset", map[string]any{
						"name": result.AssetName,
					}))
					return nil
				},
			}
			cmd.Flags().StringVar(&upgradeVersion, "version", "", application.T("flag.core.version"))
			return cmd
		}(),
	)

	return coreCmd
}

func newCoreManager(application *app.App, cmd *cobra.Command) *coremanager.Manager {
	return coremanager.NewManager(application.Config, application.State, application.Paths, cmd.ErrOrStderr(), coremanager.Text{
		FetchLatest:    application.T("msg.core.fetch_latest"),
		FetchVersion:   application.T("msg.core.fetch_version"),
		DownloadBinary: application.T("msg.core.download_progress"),
		UseBundled:     application.T("msg.core.use_bundled"),
		CheckUpdate:    application.T("msg.core.check_update"),
	})
}
