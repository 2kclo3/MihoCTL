package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
	"mihoctl/internal/mode"
)

func newModeCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:       "mode [env|tun]",
		Short:     application.T("cmd.mode.short"),
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{mode.ModeEnv, mode.ModeTun},
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
			currentMode := mode.ResolveMode(application.Config.Mode)
			previousConfig := snapshotConfig(application)
			previousState := snapshotState(application)
			if len(args) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
					"mode": currentMode,
				}))
				return nil
			}

			nextMode := mode.NormalizeMode(args[0])
			if nextMode == "" {
				return core.NewActionError("mode_invalid", "err.mode.invalid", nil, "err.mode.invalid_hint", map[string]any{
					"mode": args[0],
				}, nil)
			}
			if nextMode == currentMode {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
					"mode": nextMode,
				}))
				return nil
			}

			wasEnabled, err := manager.ModeEnabled(currentMode)
			if err != nil {
				return err
			}
			if wasEnabled {
				if err := manager.ApplyMode(currentMode, false); err != nil {
					return err
				}
			}
			application.Config.Mode = nextMode
			if wasEnabled {
				if err := ensureMihomoRuntimeReady(cmd, application); err != nil {
					rollbackModeSwitch(application, currentMode, nextMode, wasEnabled, previousConfig, previousState)
					return err
				}
				if err := manager.ApplyMode(nextMode, true); err != nil {
					rollbackModeSwitch(application, currentMode, nextMode, wasEnabled, previousConfig, previousState)
					return err
				}
			}
			if err := application.SaveConfig(); err != nil {
				rollbackModeSwitch(application, currentMode, nextMode, wasEnabled, previousConfig, previousState)
				return err
			}
			if err := application.SaveState(); err != nil {
				rollbackModeSwitch(application, currentMode, nextMode, wasEnabled, previousConfig, previousState)
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
				"mode": nextMode,
			}))
			if wasEnabled {
				switch nextMode {
				case mode.ModeEnv:
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.env.on.success"))
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.env.on.hint", envHintData(application)))
				default:
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.mode.tun.on.success"))
				}
			}
			return nil
		},
	}
}
