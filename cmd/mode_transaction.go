package cmd

import (
	"mihoctl/internal/app"
	"mihoctl/internal/config"
	"mihoctl/internal/mode"
	"mihoctl/internal/state"
)

func snapshotConfig(application *app.App) config.Config {
	cfg := *application.Config
	if cfg.Subscriptions != nil {
		cfg.Subscriptions = append([]config.Subscription(nil), cfg.Subscriptions...)
	}
	return cfg
}

func snapshotState(application *app.App) state.State {
	return *application.State
}

func restoreConfigSnapshot(application *app.App, snapshot config.Config) {
	*application.Config = snapshot
}

func restoreStateSnapshot(application *app.App, snapshot state.State) {
	*application.State = snapshot
}

func rollbackModeToggle(application *app.App, selected string, previousEnabled bool, previousState state.State) {
	manager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
	_ = manager.ApplyMode(selected, previousEnabled)
	restoreStateSnapshot(application, previousState)
}

func rollbackModeSwitch(application *app.App, previousMode, nextMode string, previousEnabled bool, previousConfig config.Config, previousState state.State) {
	manager := mode.NewManager(application.Paths, application.Config, application.State, application.MihomoClient())
	if previousEnabled {
		_ = manager.ApplyMode(nextMode, false)
		_ = manager.ApplyMode(previousMode, true)
	}
	restoreConfigSnapshot(application, previousConfig)
	restoreStateSnapshot(application, previousState)
	_ = application.SaveConfig()
	_ = application.SaveState()
}
