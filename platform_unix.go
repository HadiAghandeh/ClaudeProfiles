//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func findClaudeCode() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	// Apps started from a desktop menu often have a shorter PATH than a shell.
	home, _ := os.UserHomeDir()
	for _, c := range []string{
		filepath.Join(home, ".local/bin/claude"),
		filepath.Join(home, ".claude/local/claude"),
		filepath.Join(home, ".npm-global/bin/claude"),
		"/opt/homebrew/bin/claude",
		"/usr/local/bin/claude",
		"/usr/bin/claude",
	} {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("Claude Code was not found; install it (see https://docs.claude.com/en/docs/claude-code/setup)")
}

// codeRunsInThisTerminal: when started from a terminal, Claude Code runs right
// there instead of in a new window, which is what CLI users expect. Checking
// for a character device isn't enough (/dev/null is one, and that's what apps
// started from a desktop menu get), so ask for terminal settings instead.
func codeRunsInThisTerminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), ioctlGetTermios)
	return err == nil
}

// runCodeHere replaces this process with Claude Code, using the profile's
// config folder. It only returns on failure.
func runCodeHere(claude, configDir, workDir string) error {
	if err := os.Chdir(workDir); err != nil {
		return err
	}
	env := []string{"CLAUDE_CONFIG_DIR=" + configDir}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "CLAUDE_CONFIG_DIR=") {
			env = append(env, kv)
		}
	}
	return syscall.Exec(claude, []string{claude}, env)
}

// shQuote quotes s for a POSIX shell.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// writeCodeScript writes a small shell script that starts Claude Code for a
// profile and then leaves an interactive shell open (with the same settings),
// like the Windows version's PowerShell tab. It is rewritten on every launch.
func writeCodeScript(title, claude, configDir, workDir, ext string) (string, error) {
	dir := filepath.Join(filepath.Dir(configDir), ".launch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(configDir)+ext)
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '\\033]0;%s\\007' " + shQuote(title),
		"cd " + shQuote(workDir) + " || exit 1",
		"CLAUDE_CONFIG_DIR=" + shQuote(configDir) + "; export CLAUDE_CONFIG_DIR",
		shQuote(claude),
		`exec "${SHELL:-/bin/sh}" -i`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return "", err
	}
	return path, os.Chmod(path, 0o755)
}

func clearScreen() { fmt.Print("\033[H\033[2J") }

// desktopRunning reports whether Claude Desktop is open with this data folder.
// Chromium-based apps such as Claude Desktop keep a symlink named
// SingletonLock in their data folder pointing at "<hostname>-<pid>" while
// they run; the profile is open if that process is alive.
func desktopRunning(folder string) bool {
	target, err := os.Readlink(filepath.Join(folder, "SingletonLock"))
	if err != nil {
		return false
	}
	i := strings.LastIndex(target, "-")
	if i < 0 {
		return false
	}
	pid, err := strconv.Atoi(target[i+1:])
	if err != nil || pid <= 0 {
		return false
	}
	err = syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
