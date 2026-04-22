package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"mihoctl/internal/config"
	"mihoctl/internal/core"
	"mihoctl/internal/i18n"
	"mihoctl/internal/mihomo"
	"mihoctl/internal/state"
)

type App struct {
	Config   *config.Config
	State    *state.State
	Paths    config.Paths
	Language string
	tr       *i18n.Translator
}

func New(opts config.BootstrapOptions) (*App, error) {
	cfg, paths, err := config.Load(opts)
	if err != nil {
		return nil, err
	}

	st, err := state.Load(paths.StateFile)
	if err != nil {
		return nil, err
	}

	return &App{
		Config:   cfg,
		State:    st,
		Paths:    paths,
		Language: config.ResolveLanguage(opts, cfg),
		tr:       i18n.New(config.ResolveLanguage(opts, cfg)),
	}, nil
}

func (a *App) T(key string) string {
	return a.tr.T(key)
}

func (a *App) Tf(key string, data map[string]any) string {
	return a.tr.Tf(key, data)
}

func (a *App) ReloadTranslator() {
	a.tr = i18n.New(a.Language)
}

func (a *App) SaveConfig() error {
	return config.Save(a.Paths.ConfigFile, a.Config)
}

func (a *App) SaveState() error {
	return state.Save(a.Paths.StateFile, a.State)
}

func (a *App) MihomoClient() *mihomo.Client {
	return mihomo.NewClient(a.Config.Controller.Address, a.Config.Controller.Secret)
}

func (a *App) BoolLabel(v bool) string {
	if v {
		return a.T("label.yes")
	}
	return a.T("label.no")
}

func (a *App) ToggleLabel(known, enabled bool) string {
	if !known {
		return a.T("label.unknown")
	}
	if enabled {
		return a.T("label.on")
	}
	return a.T("label.off")
}

func (a *App) IsSupportedLanguage(lang string) bool {
	switch lang {
	case "zh-CN", "en-US":
		return true
	default:
		return false
	}
}

func (a *App) FormatError(err error) string {
	var actionErr *core.ActionError
	if errors.As(err, &actionErr) {
		suggestionData := actionErr.SuggestionData
		if suggestionData == nil {
			suggestionData = actionErr.MessageData
		}
		lines := []string{
			fmt.Sprintf("%s: %s", a.T("label.error"), a.tr.Tf(actionErr.MessageKey, actionErr.MessageData)),
			fmt.Sprintf("%s: %s", a.T("label.error_code"), actionErr.Code),
		}
		if actionErr.Cause != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", a.T("label.reason"), actionErr.Cause))
		}
		if actionErr.SuggestionKey != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", a.T("label.suggestion"), a.tr.Tf(actionErr.SuggestionKey, suggestionData)))
		}
		return strings.Join(lines, "\n")
	}
	return fmt.Sprintf("%s: %v", a.T("label.error"), err)
}

func EnsureWritable(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}
