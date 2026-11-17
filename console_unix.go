//go:build darwin || linux

package main

import (
	"os/exec"
	"syscall"
)

// setNewConsole starts the child in its own session, so apps launched from the
// menu keep running when the terminal that started them closes.
func setNewConsole(cmd *exec.Cmd, newConsole bool) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
