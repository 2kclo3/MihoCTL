//go:build darwin || linux

package process

import (
	"os/exec"
	"syscall"
)

func prepareDetachedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
