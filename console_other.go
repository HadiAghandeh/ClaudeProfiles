//go:build !windows && !darwin && !linux

package main

import "os/exec"

func setNewConsole(cmd *exec.Cmd, newConsole bool) {}
