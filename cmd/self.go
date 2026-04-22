package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/selfmanager"
)

func newSelfCommand(application *app.App) *cobra.Command {
	selfCmd := &cobra.Command{
		Use:   "self",
		Short: application.T("cmd.self.short"),
	}

	var installDir string
	var uninstallYes bool
	installCmd := &cobra.Command{
		Use:   "install",
		Short: application.T("cmd.self.install.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := selfmanager.NewManager(application.Paths)
			result, err := manager.Install(installDir)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.self.install.success", map[string]any{
				"path": result.Path,
			}))
			if result.NeedsPath {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.self.install.path_hint", map[string]any{
					"dir": filepath.Dir(result.Path),
				}))
			}
			return nil
		},
	}
	installCmd.Flags().StringVar(&installDir, "dir", "", application.T("flag.self.dir"))

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: application.T("cmd.self.uninstall.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !uninstallYes {
				return fmt.Errorf("refusing to uninstall without --yes")
			}
			manager := selfmanager.NewManager(application.Paths)
			result, err := manager.Uninstall(application.Config)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.self.uninstall.success"))
			for _, item := range result.Removed {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.self.uninstall.removed", map[string]any{
					"path": item,
				}))
			}
			for _, item := range result.Warnings {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.self.uninstall.warning", map[string]any{
					"path": item,
				}))
			}
			return nil
		},
	}
	uninstallCmd.Flags().BoolVar(&uninstallYes, "yes", false, "confirm uninstall")

	selfCmd.AddCommand(installCmd, uninstallCmd)
	return selfCmd
}
