//go:build !windows && !darwin && !linux

package main

import (
	"errors"
	"os/exec"
	"path/filepath"
)

var errUnsupported = errors.New("this operating system is not supported; use Windows, macOS or Linux")

func dataRoots() (string, string, error)                    { return "", "", errUnsupported }
func defaultRecordsFile() (string, error)                   { return "", errUnsupported }
func desktopAvailable() bool                                { return false }
func desktopCommand(string) (*exec.Cmd, error)              { return nil, errUnsupported }
func findClaudeCode() (string, error)                       { return "", errUnsupported }
func codeRunsInThisTerminal() bool                          { return false }
func runCodeHere(string, string, string) error              { return errUnsupported }
func openCodeTerminal(string, string, string, string) error { return errUnsupported }
func clearScreen()                                          {}
func desktopRunning(string) bool                            { return false }
func pathKey(p string) string                               { return filepath.Clean(p) }
func placeLabel(place string) string                        { return place }
