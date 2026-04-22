package mode

import "strings"

const (
	ModeEnv = "env"
	ModeTun = "tun"
)

func NormalizeMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ModeEnv:
		return ModeEnv
	case "", ModeTun:
		return ModeTun
	default:
		return ""
	}
}

func ResolveMode(value string) string {
	normalized := NormalizeMode(value)
	if normalized == "" {
		return ModeTun
	}
	return normalized
}
