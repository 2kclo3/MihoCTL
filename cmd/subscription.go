package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/config"
	"mihoctl/internal/subscription"
)

func newSubscriptionCommand(application *app.App) *cobra.Command {
	subCmd := &cobra.Command{
		Use:   "sub",
		Short: application.T("cmd.sub.short"),
	}

	subCmd.AddCommand(
		&cobra.Command{
			Use:   "add <url>",
			Short: application.T("cmd.sub.add.short"),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				addResult, err := manager.Add(ctx, args[0])
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				if addResult.ResolvedFrom != "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.add.resolved", map[string]any{
						"from": addResult.ResolvedFrom,
						"url":  addResult.Entry.URL,
					}))
				}
				if addResult.DetectedUserAgent != "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.add.user_agent", map[string]any{
						"user_agent": addResult.DetectedUserAgent,
					}))
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.add.success", map[string]any{
					"name": addResult.Entry.Name,
					"url":  addResult.Entry.URL,
				}))
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.update.item_success", map[string]any{
					"name": addResult.Update.Name,
					"url":  addResult.Update.URL,
					"path": addResult.Update.Path,
				}))
				if addResult.Update.Reloaded {
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.sub.update.reloaded"))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: application.T("cmd.sub.list.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				entries := manager.List()
				if len(entries) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.sub.list.empty"))
					return nil
				}
				for i, entry := range entries {
					entry = normalizeEntry(entry, application.Paths.SubDir)
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.list.item", map[string]any{
						"index":   i + 1,
						"name":    entry.Name,
						"url":     entry.URL,
						"path":    entry.ConfigPath,
						"default": defaultMark(entry.Name == application.Config.DefaultSubscription || entry.URL == application.Config.DefaultSubscription, application),
					}))
				}
				if application.Config.DefaultSubscription == "" {
					fmt.Fprintln(cmd.OutOrStdout(), application.T("msg.sub.list.no_default"))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "update [url]",
			Short: application.T("cmd.sub.update.short"),
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				target := ""
				if len(args) == 1 {
					target = args[0]
				}

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := manager.Update(ctx, target)
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
				return nil
			},
		},
		&cobra.Command{
			Use:   "use <name|index>",
			Short: application.T("cmd.sub.use.short"),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				entry, err := manager.Use(ctx, args[0])
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.use.success", map[string]any{
					"name": entry.Name,
					"path": application.Config.Mihomo.ConfigPath,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <name|url|index>",
			Short: application.T("cmd.sub.remove.short"),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				entry, err := manager.Remove(ctx, args[0])
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.remove.success", map[string]any{
					"name": entry.Name,
					"url":  entry.URL,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "set-ua <name|url|index> <user-agent>",
			Short: application.T("cmd.sub.setua.short"),
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				entry, err := manager.SetUserAgent(args[0], args[1])
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.ua.set", map[string]any{
					"name":       entry.Name,
					"user_agent": entry.UserAgent,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "clear-ua <name|url|index>",
			Short: application.T("cmd.sub.clearua.short"),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager := subscriptionManagerFromCmd(application, cmd)
				entry, err := manager.ClearUserAgent(args[0])
				if err != nil {
					return err
				}
				if err := application.SaveConfig(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.sub.ua.clear", map[string]any{
					"name": entry.Name,
				}))
				return nil
			},
		},
	)

	return subCmd
}

func subscriptionManagerFromCmd(application *app.App, cmd *cobra.Command) *subscription.Manager {
	return subscription.NewManager(application.Config, application.Paths, application.MihomoClient(), cmd.ErrOrStderr(), application.T("msg.sub.download_progress"))
}

func defaultMark(isDefault bool, application *app.App) string {
	if isDefault {
		return application.T("label.default")
	}
	return ""
}

func normalizeEntry(entry config.Subscription, subDir string) config.Subscription {
	if entry.ConfigPath == "" {
		entry.ConfigPath = filepath.Join(subDir, entry.Name+".yaml")
	}
	return entry
}
