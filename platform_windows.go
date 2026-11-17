//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dataRoots returns where Claude Desktop and Claude Code profile data live.
func dataRoots() (desktopRoot, codeRoot string, err error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(local, "Claude-Data"), filepath.Join(local, "Claude-Code-Data"), nil
}

// defaultRecordsFile keeps profiles.json next to the .exe, as earlier
// versions did, so existing profile lists keep working.
func defaultRecordsFile() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "profiles.json"), nil
}

func findClaudeDesktop() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	for _, c := range []string{
		filepath.Join(local, `Microsoft\WindowsApps\claude-desktop.exe`),
		filepath.Join(local, `Programs\Claude\Claude.exe`),
		filepath.Join(local, `AnthropicClaude\Claude.exe`),
	} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Desktop was not found; install it from https://claude.ai/download")
}

func desktopAvailable() bool {
	_, err := findClaudeDesktop()
	return err == nil
}

// desktopCommand starts Claude Desktop with its own data folder, or the
// normal profile when folder is "".
func desktopCommand(folder string) (*exec.Cmd, error) {
	claude, err := findClaudeDesktop()
	if err != nil {
		return nil, err
	}
	if folder == "" {
		return exec.Command(claude), nil
	}
	return exec.Command(claude, "--user-data-dir="+folder), nil
}

func findClaudeCode() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	for _, c := range []string{
		filepath.Join(os.Getenv("USERPROFILE"), `.local\bin\claude.exe`),
		filepath.Join(os.Getenv("APPDATA"), `npm\claude.cmd`),
	} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Code was not found; install it (see https://docs.claude.com/en/docs/claude-code/setup)")
}

// codeRunsInThisTerminal: on Windows Claude Code always gets its own terminal
// tab, as before.
func codeRunsInThisTerminal() bool { return false }

func runCodeHere(claude, configDir, workDir string) error {
	return errors.New("not supported on Windows")
}

// openCodeTerminal starts Claude Code in a new Windows Terminal tab (or a
// PowerShell window), with the profile's config folder.
func openCodeTerminal(title, claude, configDir, workDir string) error {
	// Windows Terminal treats ';' as a command separator.
	title = strings.ReplaceAll(title, ";", ",")
	script := fmt.Sprintf("$Host.UI.RawUI.WindowTitle = %s; $env:CLAUDE_CONFIG_DIR = %s; Set-Location -LiteralPath %s; & %s",
		psQuote(title), psQuote(configDir), psQuote(workDir), psQuote(claude))
	psArgs := []string{"-NoExit", "-NoProfile", "-EncodedCommand", encodePS(script)}
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		return startDetached(exec.Command(wt, append([]string{"new-tab", "--title", title, "powershell.exe"}, psArgs...)...), false)
	}
	return startDetached(exec.Command("powershell.exe", psArgs...), true)
}

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

// pathKey makes paths comparable: Windows paths ignore case.
func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

// placeLabel names where a shortcut goes, for messages.
func placeLabel(place string) string {
	if place == placeStart {
		return "Start menu"
	}
	return "Desktop"
}
