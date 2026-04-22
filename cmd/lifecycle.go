package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
	"mihoctl/internal/mode"
	"mihoctl/internal/process"
	"mihoctl/internal/service"
)

func newStartCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: application.T("cmd.start.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := process.NewManager(application.Config, application.State, application.Paths)
			info, err := manager.Start()
			if err != nil {
				return err
			}
			if err := application.SaveState(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.start.success", map[string]any{
				"pid": info.PID,
				"log": info.LogFile,
			}))
			return nil
		},
	}
}

func newStopCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: application.T("cmd.stop.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			processManager := process.NewManager(application.Config, application.State, application.Paths)
			modeManager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())

			currentMode := mode.ResolveMode(application.Config.Mode)
			envEnabled := false
			if currentMode == mode.ModeEnv {
				if enabled, err := modeManager.ModeEnabled(mode.ModeEnv); err == nil {
					envEnabled = enabled
				}
			}

			err := processManager.Stop()
			if err != nil && !isProcessNotRunningError(err) {
				return err
			}

			envDisabled := false
			if envEnabled {
				if err := modeManager.SetSystemProxy(false); err != nil {
					return err
				}
				envDisabled = true
			}

			if err := application.SaveState(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.stop.success"))
			if envDisabled {
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.stop.env_off"))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.env.off.hint", envHintData(application)))
			}
			return nil
		},
	}
}

func newRestartCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: application.T("cmd.restart.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := process.NewManager(application.Config, application.State, application.Paths)
			info, err := manager.Restart()
			if err != nil {
				return err
			}
			if err := application.SaveState(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.restart.success", map[string]any{
				"pid": info.PID,
			}))
			return nil
		},
	}
}

func newStatusCommand(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: application.T("cmd.status.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			processManager := process.NewManager(application.Config, application.State, application.Paths)
			processStatus, err := processManager.Status()
			if err != nil {
				return err
			}
			modeManager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
			serviceStatus, serviceErr := service.NewManager(application.Config).Status()
			envStatus, envErr := modeManager.SystemProxyStatus()
			tunStatus, tunErr := modeManager.TunStatus()
			tunConfigured, tunConfiguredErr := modeManager.TunConfiguredState()

			client := application.MihomoClient()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			version, apiErr := client.Ping(ctx)
			fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.status.header"))

			if serviceErr == nil {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.status.service", map[string]any{
					"registered": application.BoolLabel(serviceStatus.Registered),
					"enabled":    application.BoolLabel(serviceStatus.Enabled),
					"active":     application.BoolLabel(serviceStatus.Active),
				}))
			} else if processStatus.Running {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.status.process.running", map[string]any{
					"pid":    processStatus.PID,
					"uptime": formatDuration(processStatus.Uptime),
				}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.status.process.stopped"))
			}

			if apiErr == nil {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.status.api.ok", map[string]any{
					"version": version,
				}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.status.api.fail", map[string]any{
					"addr": application.Config.Controller.Address,
				}))
			}

			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.current", map[string]any{
				"mode": mode.ResolveMode(application.Config.Mode),
			}))
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.status.item", map[string]any{
				"name":   "ENV",
				"status": application.ToggleLabel(envStatus.Known, envStatus.Enabled),
				"source": fallbackValue(envStatus.Source, application.T("label.unknown")),
				"error":  fallbackValue(envStatus.LastError, application.T("label.none")),
			}))
			fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.status.item", map[string]any{
				"name":   "TUN",
				"status": application.ToggleLabel(tunStatus.Known, tunStatus.Enabled),
				"source": fallbackValue(tunStatus.Source, application.T("label.unknown")),
				"error":  modeLastErrorLabel(tunStatus.LastError, application),
			}))
			if tunConfiguredErr == nil && tunConfigured != nil && tunStatus.Known && *tunConfigured != tunStatus.Enabled {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.tun.mismatch", map[string]any{
					"config":  application.ToggleLabel(true, *tunConfigured),
					"runtime": application.ToggleLabel(true, tunStatus.Enabled),
				}))
			}
			if envErr != nil {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.status.hint", map[string]any{
					"name": "ENV",
				}))
			}
			if tunErr != nil {
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.mode.status.hint", map[string]any{
					"name": "TUN",
				}))
			}
			return nil
		},
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	if d < time.Hour {
		return d.Truncate(time.Minute).String()
	}
	return d.Truncate(time.Minute).String()
}

func isProcessNotRunningError(err error) bool {
	var actionErr *core.ActionError
	return errors.As(err, &actionErr) && actionErr.Code == "process_not_running"
}
