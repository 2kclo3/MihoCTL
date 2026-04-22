package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
)

func maybeNotifyCoreUpdate(cmd *cobra.Command, application *app.App) error {
	if shouldSkipUpdateCheck(cmd) {
		return nil
	}

	manager := newCoreManager(application, cmd)
	if !manager.ShouldCheckForUpdate() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	info, err := manager.CheckForUpdate(ctx)
	if err != nil {
		return nil
	}
	_ = application.SaveState()

	if !info.Available {
		return nil
	}

	fmt.Fprintln(cmd.ErrOrStderr(), application.Tf("msg.core.update_available", map[string]any{
		"current": info.CurrentVersion,
		"latest":  info.LatestVersion,
	}))

	if !isInteractiveInput() {
		return nil
	}

	answer, err := askForConfirmation(cmd, application.T("msg.core.update_prompt"))
	if err != nil {
		return nil
	}
	if !answer {
		return nil
	}

	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer upgradeCancel()

	result, err := manager.Upgrade(upgradeCtx, "")
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), application.FormatError(err))
		return nil
	}
	_ = application.SaveConfig()
	_ = application.SaveState()

	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.core.upgrade.success", map[string]any{
		"from": result.PreviousVersion,
		"to":   result.Version,
		"path": result.BinaryPath,
	}))
	return nil
}

func shouldSkipUpdateCheck(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	skips := []string{
		"mihoctl",
		"mihoctl core",
		"mihoctl update",
		"mihoctl self",
		"mihoctl config",
		"mihoctl doctor",
	}
	for _, prefix := range skips {
		if path == prefix || strings.HasPrefix(path, prefix+" ") {
			return true
		}
	}
	return false
}

func isInteractiveInput() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func askForConfirmation(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
