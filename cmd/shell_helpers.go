package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mihoctl/internal/app"
)

func envSyncCommand(application *app.App) string {
	if startupFile := currentShellStartupFile(); startupFile != "" {
		return "source " + displayHomePath(startupFile)
	}
	return "source " + displayHomePath(filepath.Join(application.Paths.AppHome, "system-proxy.env"))
}

func currentShellRestartCommand() string {
	shellName := currentShellName()
	if shellName == "" {
		if shellPath := strings.TrimSpace(os.Getenv("SHELL")); shellPath != "" {
			return "exec " + shellPath + " -l"
		}
		return ""
	}
	return "exec " + shellName + " -l"
}

func currentShellStartupFile() string {
	home := userHomeDir()
	if home == "" {
		return ""
	}
	switch currentShellName() {
	case "zsh":
		zdotdir := strings.TrimSpace(os.Getenv("ZDOTDIR"))
		if zdotdir != "" {
			return filepath.Join(zdotdir, ".zshrc")
		}
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return ""
	}
}

func currentShellName() string {
	return filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func displayHomePath(path string) string {
	home := userHomeDir()
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + strings.TrimPrefix(path, prefix)
	}
	return path
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func bootElevatedCommand(application *app.App, action string) string {
	executable := application.Paths.ExecPath
	if strings.TrimSpace(executable) == "" {
		executable = "mihoctl"
	}
	return fmt.Sprintf("sudo %s --config %s boot %s", shellQuote(executable), shellQuote(application.Paths.ConfigFile), action)
}
