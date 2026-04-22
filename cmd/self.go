package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/config"
	"mihoctl/internal/core"
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
				return core.NewActionError("self_uninstall_confirm_required", "err.self.uninstall_confirm", nil, "err.self.uninstall_confirm_hint", nil, nil)
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
			if command := currentShellCleanupHint(application.Config); command != "" {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.self.uninstall.shell_hint", map[string]any{
					"command": command,
				}))
			}
			return nil
		},
	}
	uninstallCmd.Flags().BoolVar(&uninstallYes, "yes", false, application.T("flag.self.yes"))

	selfCmd.AddCommand(installCmd, uninstallCmd)
	return selfCmd
}

func currentShellCleanupHint(cfg *config.Config) string {
	if !managedProxyEnvActive(cfg) {
		return ""
	}
	if command := currentShellRestartCommand(); command != "" {
		return command
	}
	return currentShellUnsetProxyCommand()
}

func managedProxyEnvActive(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	managedHTTP := fmt.Sprintf("http://%s:%d", cfg.SystemProxy.Host, cfg.SystemProxy.Port)
	managedSOCKS := fmt.Sprintf("socks5://%s:%d", cfg.SystemProxy.Host, cfg.SystemProxy.Port)
	candidates := map[string]string{
		"http_proxy":  managedHTTP,
		"https_proxy": managedHTTP,
		"HTTP_PROXY":  managedHTTP,
		"HTTPS_PROXY": managedHTTP,
		"all_proxy":   managedSOCKS,
		"ALL_PROXY":   managedSOCKS,
	}
	for key, expected := range candidates {
		if os.Getenv(key) == expected {
			return true
		}
	}
	return false
}

func currentShellUnsetProxyCommand() string {
	switch currentShellName() {
	case "fish":
		return "set -e http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY no_proxy NO_PROXY"
	default:
		return "unset http_proxy https_proxy all_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY no_proxy NO_PROXY"
	}
}
