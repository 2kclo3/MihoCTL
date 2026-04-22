package cmd

import "mihoctl/internal/app"

func fallbackValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func modeLastErrorLabel(value string, application *app.App) string {
	switch value {
	case "":
		return application.T("label.none")
	case "tun_runtime_mismatch":
		return application.T("msg.mode.tun.runtime_mismatch")
	default:
		return value
	}
}
