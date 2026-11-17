//go:build !windows

package main

import "os/exec"

// setNewConsole is a no-op off Windows; the tool targets Windows, but this keeps
// `go vet` and tests working on any OS.
func setNewConsole(cmd *exec.Cmd, newConsole bool) {}
