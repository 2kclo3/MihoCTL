package cmd

import (
	"github.com/spf13/cobra"

	"mihoctl/internal/app"
)

func NewRootCommand(application *app.App) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "mihoctl",
		Short:         application.T("cmd.root.short"),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return maybeNotifyCoreUpdate(cmd, application)
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().String("lang", application.Language, application.T("flag.lang"))
	rootCmd.PersistentFlags().String("config", application.Paths.ConfigFile, application.T("flag.config"))

	startCmd := newStartCommand(application)
	stopCmd := newStopCommand(application)
	restartCmd := newRestartCommand(application)
	completionCmd := newCompletionCommand(application)
	coreCmd := newCoreCommand(application)
	selfCmd := newSelfCommand(application)
	configCmd := newConfigCommand(application)
	bootCmd := newBootCommand(application)

	startCmd.Hidden = true
	stopCmd.Hidden = true
	restartCmd.Hidden = true
	completionCmd.Hidden = true
	coreCmd.Hidden = true
	selfCmd.Hidden = true
	configCmd.Hidden = true

	rootCmd.AddCommand(
		newStatusCommand(application),
		bootCmd,
		newSubscriptionCommand(application),
		newProxyCommand(application),
		newModeCommand(application),
		newOnCommand(application),
		newOffCommand(application),
		newUpdateCommand(application),
		newDoctorCommand(application),
		startCmd,
		stopCmd,
		restartCmd,
		completionCmd,
		coreCmd,
		selfCmd,
		configCmd,
	)

	return rootCmd
}
