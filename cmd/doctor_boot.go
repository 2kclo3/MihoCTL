package cmd

import (
	"mihoctl/internal/app"
	"mihoctl/internal/service"
)

func doctorBootStatusLabel(application *app.App) string {
	status, err := service.NewManager(application.Config).Status()
	if err != nil {
		return application.T("label.unknown")
	}
	if status.Enabled || status.Registered {
		return application.T("label.on")
	}
	return application.T("label.off")
}
