//go:build darwin || linux

package cmd

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

type terminalWinsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func realTerminalWidth(out any) int {
	writer, ok := out.(io.Writer)
	if !ok {
		return 0
	}
	file, ok := writer.(*os.File)
	if !ok {
		return 0
	}
	ws := &terminalWinsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 || ws.Col == 0 {
		return 0
	}
	return int(ws.Col)
}
