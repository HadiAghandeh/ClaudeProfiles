//go:build linux

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
const ioctlGetTermios = unix.TCGETS

// dataHome is $XDG_DATA_HOME/claude-profiles (~/.local/share/claude-profiles).
func dataHome() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "claude-profiles"), nil
}

func dataRoots() (desktopRoot, codeRoot string, err error) {
	base, err := dataHome()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(base, "Claude-Data"), filepath.Join(base, "Claude-Code-Data"), nil
}

// defaultRecordsFile is ~/.config/claude-profiles/profiles.json.
func defaultRecordsFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "claude-profiles", "profiles.json"), nil
}

// findClaudeDesktop looks for the "claude-desktop" command, which Anthropic's
// Linux beta package (Debian/Ubuntu, https://code.claude.com/docs/en/desktop-linux)
// installs. Community builds use the same command name.
func findClaudeDesktop() (string, error) {
	if p, err := exec.LookPath("claude-desktop"); err == nil {
		return p, nil
	}
	for _, c := range []string{"/usr/bin/claude-desktop", "/usr/local/bin/claude-desktop", "/opt/Claude/claude-desktop"} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Desktop was not found. On Ubuntu 22.04+ or Debian 12+, install the Linux beta (sudo apt install claude-desktop, see https://code.claude.com/docs/en/desktop-linux); Claude Code profiles work without it")
}

func desktopAvailable() bool {
	_, err := findClaudeDesktop()
	return err == nil
}

func desktopCommand(folder string) (*exec.Cmd, error) {
	claude, err := findClaudeDesktop()
	if err != nil {
		return nil, err
	}
	// Claude Desktop (Chromium) refuses to run as root unless its sandbox is
	// turned off, which Anthropic doesn't support. Say so instead of failing
	// silently; WSL in particular often logs in as root.
	if os.Geteuid() == 0 {
		return nil, errors.New("Claude Desktop can't run as root. Use a regular user account (on WSL, create one with \"adduser\" and set it as the default user); Claude Code profiles work as root")
	}
	if folder == "" {
		return exec.Command(claude), nil
	}
	return exec.Command(claude, "--user-data-dir="+folder), nil
}

// terminals are tried in order to open Claude Code from the window or a
// menu entry; each is listed with the flag that runs a command.
var terminals = []struct{ name, flag string }{
	{"x-terminal-emulator", "-e"}, // Debian/Ubuntu's configured default
	{"gnome-terminal", "--"},
	{"ptyxis", "--"},
	{"kgx", "-e"},
	{"konsole", "-e"},
	{"xfce4-terminal", "-x"},
	{"mate-terminal", "-x"},
	{"tilix", "-e"},
	{"kitty", ""},
	{"alacritty", "-e"},
	{"wezterm", "start --"},
	{"foot", ""},
	{"xterm", "-e"},
}

// openCodeTerminal opens a terminal window running Claude Code for a profile.
// $TERMINAL wins if set.
func openCodeTerminal(title, claude, configDir, workDir string) error {
	script, err := writeCodeScript(title, claude, configDir, workDir, ".sh")
	if err != nil {
		return err
	}
	if t := os.Getenv("TERMINAL"); t != "" {
		if p, err := exec.LookPath(t); err == nil {
			return startDetached(exec.Command(p, "-e", script), false)
		}
	}
	for _, t := range terminals {
		p, err := exec.LookPath(t.name)
		if err != nil {
			continue
		}
		args := append(strings.Fields(t.flag), script)
		return startDetached(exec.Command(p, args...), false)
	}
	return errors.New("no terminal app was found; install one (for example gnome-terminal or xterm) or set $TERMINAL")
}

// pathKey makes paths comparable: Linux paths are case-sensitive.
func pathKey(p string) string { return filepath.Clean(p) }

func placeLabel(place string) string {
	if place == placeStart {
		return "app menu"
	}
	return "Desktop"
}
