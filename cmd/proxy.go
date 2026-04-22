package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/mihomo"
)

func newProxyCommand(application *app.App) *cobra.Command {
	proxyCmd := &cobra.Command{
		Use:   "proxy",
		Short: application.T("cmd.proxy.short"),
	}

	proxyCmd.AddCommand(
		&cobra.Command{
			Use:   "ls",
			Short: application.T("cmd.proxy.ls.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				groups, err := application.MihomoClient().ListProxyGroups(ctx)
				if err != nil {
					return err
				}
				for i, group := range groups {
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.list.group", map[string]any{
						"index": i + 1,
						"name":  group.Name,
						"now":   group.Now,
					}))
					for j, proxy := range group.All {
						fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s\n", j+1, proxy)
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "use <group|index> <proxy|index>",
			Short: application.T("cmd.proxy.use.short"),
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				groups, err := application.MihomoClient().ListProxyGroups(ctx)
				if err != nil {
					return err
				}
				group, proxy, err := resolveProxySelection(groups, args[0], args[1])
				if err != nil {
					return err
				}
				if err := application.MihomoClient().UseProxy(ctx, group, proxy); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.use.success", map[string]any{
					"group": group,
					"proxy": proxy,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: application.T("cmd.proxy.check.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				client := application.MihomoClient()
				groups, err := client.ListProxyGroups(ctx)
				if err != nil {
					return err
				}
				for _, group := range groups {
					delays, err := client.CheckGroupDelay(ctx, group.Name, application.Config.HealthCheck.URL, application.Config.HealthCheck.TimeoutMS)
					if err != nil {
						fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.fail", map[string]any{
							"group": group.Name,
						}))
						continue
					}
					fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.group", map[string]any{
						"group": group.Name,
					}))
					names := make([]string, 0, len(delays))
					for proxy := range delays {
						names = append(names, proxy)
					}
					sort.Strings(names)
					for _, proxy := range names {
						delay := delays[proxy]
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %dms\n", proxy, delay)
					}
				}
				return nil
			},
		},
	)

	return proxyCmd
}

func resolveProxySelection(groups []mihomo.ProxyGroup, groupValue, proxyValue string) (string, string, error) {
	group, err := selectProxyGroup(groups, groupValue)
	if err != nil {
		return "", "", err
	}
	proxy, err := selectProxyNode(group.All, proxyValue)
	if err != nil {
		return "", "", err
	}
	return group.Name, proxy, nil
}

func selectProxyGroup(groups []mihomo.ProxyGroup, value string) (mihomo.ProxyGroup, error) {
	if index, ok := parseProxyIndex(value); ok {
		if index >= 0 && index < len(groups) {
			return groups[index], nil
		}
		return mihomo.ProxyGroup{}, fmt.Errorf("proxy group index out of range: %s", value)
	}
	for _, group := range groups {
		if group.Name == strings.TrimSpace(value) {
			return group, nil
		}
	}
	return mihomo.ProxyGroup{}, fmt.Errorf("proxy group not found: %s", value)
}

func selectProxyNode(nodes []string, value string) (string, error) {
	if index, ok := parseProxyIndex(value); ok {
		if index >= 0 && index < len(nodes) {
			return nodes[index], nil
		}
		return "", fmt.Errorf("proxy node index out of range: %s", value)
	}
	for _, node := range nodes {
		if node == strings.TrimSpace(value) {
			return node, nil
		}
	}
	return "", fmt.Errorf("proxy node not found: %s", value)
}

func parseProxyIndex(value string) (int, bool) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index <= 0 {
		return 0, false
	}
	return index - 1, true
}
