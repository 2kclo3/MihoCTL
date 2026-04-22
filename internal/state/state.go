package state

import (
	"encoding/json"
	"os"
	"time"

	"mihoctl/internal/core"
)

type State struct {
	Process ProcessState `json:"process"`
	Core    CoreState    `json:"core"`
	Modes   ModeState    `json:"modes"`
}

type ProcessState struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	LogFile   string    `json:"log_file"`
}

type ModeState struct {
	SystemProxy ToggleState `json:"system_proxy"`
	TUN         ToggleState `json:"tun"`
}

type CoreState struct {
	Version         string    `json:"version"`
	AssetName       string    `json:"asset_name"`
	InstalledAt     time.Time `json:"installed_at"`
	Source          string    `json:"source"`
	LastCheckedAt   time.Time `json:"last_checked_at"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
}

type ToggleState struct {
	Known     bool      `json:"known"`
	Enabled   bool      `json:"enabled"`
	Source    string    `json:"source"`
	LastError string    `json:"last_error"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Load(path string) (*State, error) {
	st := &State{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := Save(path, st); err != nil {
			return nil, err
		}
		return st, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, core.NewActionError("read_state_failed", "err.config.read", err, "err.config.check_path", map[string]any{
			"path": path,
		}, nil)
	}
	if len(data) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, core.NewActionError("parse_state_failed", "err.config.parse", err, "err.config.fix_format", map[string]any{
			"path": path,
		}, nil)
	}
	return st, nil
}

func Save(path string, st *State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return core.NewActionError("marshal_state_failed", "err.config.write", err, "", nil, nil)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return core.NewActionError("write_state_failed", "err.config.write", err, "err.path.check_permission", map[string]any{
			"path": path,
		}, nil)
	}
	return nil
}
