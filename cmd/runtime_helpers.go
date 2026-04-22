package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/core"
	"mihoctl/internal/mode"
	"mihoctl/internal/process"
)

func ensureMihomoRuntimeReady(cmd *cobra.Command, application *app.App) error {
	modeManager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
	if err := modeManager.EnsureActiveConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := application.MihomoClient().Ping(ctx); err == nil {
		return nil
	}

	processManager := process.NewManager(application.Config, application.State, application.Paths)
	processStatus, err := processManager.Status()
	if err != nil {
		return err
	}

	if !processStatus.Running {
		fmt.Fprintln(cmd.ErrOrStderr(), application.T("msg.runtime.starting"))
		if _, err := processManager.Start(); err != nil {
			return err
		}
		if err := application.SaveState(); err != nil {
			return err
		}
	}

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer readyCancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := application.MihomoClient().Ping(readyCtx); err == nil {
			return nil
		}
		select {
		case <-readyCtx.Done():
			return core.NewActionError("controller_request_failed", "err.http.request", readyCtx.Err(), "err.http.check_controller", map[string]any{
				"addr": application.Config.Controller.Address,
			}, nil)
		case <-ticker.C:
		}
	}
}
