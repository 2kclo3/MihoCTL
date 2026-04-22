package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/mode"
)

func newOnCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: application.T("cmd.on.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModeToggle(cmd, application, true)
		},
	}
}

func newOffCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: application.T("cmd.off.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModeToggle(cmd, application, false)
		},
	}
}

func runModeToggle(cmd *cobra.Command, application *app.App, enabled bool) error {
	manager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
	currentMode := mode.ResolveMode(application.Config.Mode)
	previousState := snapshotState(application)
	previousEnabled, err := manager.ModeEnabled(currentMode)
	if err != nil {
		previousEnabled = false
	}

	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
		"mode": currentMode,
	}))

	if enabled {
		if err := ensureMihomoRuntimeReady(cmd, application); err != nil {
			return err
		}
	}

	switch currentMode {
	case mode.ModeEnv:
		if err := manager.SetSystemProxy(enabled); err != nil {
			rollbackModeToggle(application, currentMode, previousEnabled, previousState)
			return err
		}
		if err := application.SaveState(); err != nil {
			rollbackModeToggle(application, currentMode, previousEnabled, previousState)
			return err
		}
		if enabled {
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.env.on.success"))
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.env.on.hint", envHintData(application)))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.env.off.success"))
		fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.env.off.hint", envHintData(application)))
		return nil
	default:
		if err := manager.SetTun(enabled); err != nil {
			rollbackModeToggle(application, currentMode, previousEnabled, previousState)
			return err
		}
		if err := application.SaveState(); err != nil {
			rollbackModeToggle(application, currentMode, previousEnabled, previousState)
			return err
		}
		if enabled {
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.tun.on.success"))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.tun.off.success"))
		return nil
	}
}

func envHintData(application *app.App) map[string]any {
	return map[string]any{
		"command": envSyncCommand(application),
	}
}
