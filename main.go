package main

import (
	"fmt"
	"os"

	"mihoctl/cmd"
	"mihoctl/internal/app"
	"mihoctl/internal/config"
)

func main() {
	opts := config.ParseBootstrapOptions(os.Args[1:])

	application, err := app.New(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCommand(application)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, application.FormatError(err))
		os.Exit(1)
	}
}
