//go:build !darwin && !linux

package process

import "os/exec"

func prepareDetachedCommand(cmd *exec.Cmd) {}
