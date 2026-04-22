//go:build !darwin && !linux

package cmd

func realTerminalWidth(out any) int {
	return 0
}
