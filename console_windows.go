//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

const createNewConsole = 0x00000010

// setNewConsole gives a console child its own window, so it keeps running after
// this launcher exits instead of sharing (and losing) our console.
func setNewConsole(cmd *exec.Cmd, newConsole bool) {
	if newConsole {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	}
}
