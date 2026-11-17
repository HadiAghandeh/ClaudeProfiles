//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// ioctlGetTermios reads terminal settings; it fails when stdin isn't a terminal.
const ioctlGetTermios = unix.TIOCGETA

// appSupport is ~/Library/Application Support/Claude Profiles.
func appSupport() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Claude Profiles"), nil
}

func dataRoots() (desktopRoot, codeRoot string, err error) {
	base, err := appSupport()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(base, "Claude-Data"), filepath.Join(base, "Claude-Code-Data"), nil
}

func defaultRecordsFile() (string, error) {
	base, err := appSupport()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "profiles.json"), nil
}

// findClaudeDesktop returns the Claude.app bundle.
func findClaudeDesktop() (string, error) {
	home, _ := os.UserHomeDir()
	for _, c := range []string{"/Applications/Claude.app", filepath.Join(home, "Applications", "Claude.app")} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Desktop was not found in Applications; install it from https://claude.ai/download")
}

func desktopAvailable() bool {
	_, err := findClaudeDesktop()
	return err == nil
}

// desktopCommand starts Claude.app. With a data folder it asks for a new
// instance (-n), which then keeps its login and settings in that folder.
func desktopCommand(folder string) (*exec.Cmd, error) {
	app, err := findClaudeDesktop()
	if err != nil {
		return nil, err
	}
	if folder == "" {
		return exec.Command("open", "-a", app), nil
	}
	return exec.Command("open", "-n", "-a", app, "--args", "--user-data-dir="+folder), nil
}

// openCodeTerminal opens Terminal.app running Claude Code for a profile.
func openCodeTerminal(title, claude, configDir, workDir string) error {
	script, err := writeCodeScript(title, claude, configDir, workDir, ".command")
	if err != nil {
		return err
	}
	return exec.Command("open", "-a", "Terminal", script).Run()
}

// pathKey makes paths comparable: macOS paths ignore case by default.
func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

func placeLabel(place string) string {
	if place == placeStart {
		return "Applications folder"
	}
	return "Desktop"
}
