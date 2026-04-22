package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
)

func newCompletionCommand(application *app.App) *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: application.T("cmd.completion.short"),
	}

	completionCmd.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: application.T("cmd.completion.bash.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenBashCompletion(os.Stdout)
			},
		},
		&cobra.Command{
			Use:   "zsh",
			Short: application.T("cmd.completion.zsh.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenZshCompletion(os.Stdout)
			},
		},
		&cobra.Command{
			Use:   "fish",
			Short: application.T("cmd.completion.fish.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			},
		},
	)

	return completionCmd
}
