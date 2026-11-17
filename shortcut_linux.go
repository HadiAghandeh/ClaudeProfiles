//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// shortcutDir: the Desktop folder, or ~/.local/share/applications, which is
// what app menus (GNOME, KDE, …) read.
func shortcutDir(place string) (string, error) {
	switch place {
	case placeDesktop:
		return desktopDir()
	case placeStart:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "applications"), nil
	}
	return "", fmt.Errorf("unknown shortcut place %q (use desktop or start)", place)
}

// desktopDir follows the XDG user-dirs setting (it differs per language).
func desktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if out, err := exec.Command("xdg-user-dir", "DESKTOP").Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" && d != home {
			return d, nil
		}
	}
	cfg, _ := os.UserConfigDir()
	if f, err := os.Open(filepath.Join(cfg, "user-dirs.dirs")); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if v, ok := strings.CutPrefix(line, "XDG_DESKTOP_DIR="); ok {
				v = strings.ReplaceAll(strings.Trim(v, `"`), "$HOME", home)
				if v != "" && v != home {
					return v, nil
				}
			}
		}
	}
	return filepath.Join(home, "Desktop"), nil
}

func shortcutFile(label string, code bool) string {
	kind := ""
	if code {
		kind = "-code"
	}
	return "claude-profiles-" + safeFolderName(label) + kind + ".desktop"
}

// execQuote quotes one argument for a .desktop Exec line.
func execQuote(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	if s != "" && !strings.ContainsAny(s, " \t\n\"'\\><~|&;$*?#()`") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(s) + `"`
}

// entryValue escapes a .desktop string value.
func entryValue(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`).Replace(s)
}

// createShortcut writes a .desktop launcher that opens Claude Desktop (or
// Claude Code, in dir) as r. Claude Code entries ask the desktop to open a
// terminal, where Claude Code then runs.
func (a *app) createShortcut(r *Record, place string, code bool, dir string) (string, error) {
	folder, err := shortcutDir(place)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	exe, err := selfExe()
	if err != nil {
		return "", err
	}
	args, workDir, err := a.shortcutArgs(r, code, strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}
	execLine := execQuote(exe)
	for _, s := range args {
		execLine += " " + execQuote(s)
	}
	name, comment, terminal := "Claude - "+r.Label, "Open Claude Desktop as "+r.Label, "false"
	if code {
		name, comment, terminal = "Claude Code - "+r.Label, "Open Claude Code as "+r.Label, "true"
	}
	entry := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Version=1.0",
		"Name=" + entryValue(name),
		"Comment=" + entryValue(comment),
		"Exec=" + execLine,
		"Path=" + entryValue(workDir),
		"Icon=" + entryValue(iconFile()),
		"Terminal=" + terminal,
		"Categories=Development;Utility;",
		"X-Claude-Profiles-Label=" + entryValue(r.Label),
		"",
	}, "\n")
	path := filepath.Join(folder, shortcutFile(r.Label, code))
	if err := os.WriteFile(path, []byte(entry), 0o755); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return "", err
	}
	if place == placeDesktop {
		// GNOME only runs Desktop launchers marked as trusted.
		_ = exec.Command("gio", "set", path, "metadata::trusted", "true").Run()
	} else {
		_ = exec.Command("update-desktop-database", folder).Run()
	}
	return path, nil
}

// renameShortcuts rewrites a profile's launchers for its new name.
func renameShortcuts(oldLabel, newLabel string) error {
	var failed []string
	for _, place := range []string{placeDesktop, placeStart} {
		folder, err := shortcutDir(place)
		if err != nil {
			continue
		}
		for _, code := range []bool{false, true} {
			oldPath := filepath.Join(folder, shortcutFile(oldLabel, code))
			raw, err := os.ReadFile(oldPath)
			if err != nil {
				continue
			}
			var lines []string
			for _, line := range strings.Split(string(raw), "\n") {
				switch {
				case strings.HasPrefix(line, "Exec="):
					line = strings.Replace(line, " -profile "+execQuote(oldLabel), " -profile "+execQuote(newLabel), 1)
				case strings.HasPrefix(line, "Name="), strings.HasPrefix(line, "Comment="), strings.HasPrefix(line, "X-Claude-Profiles-Label="):
					line = strings.Replace(line, entryValue(oldLabel), entryValue(newLabel), 1)
				}
				lines = append(lines, line)
			}
			newPath := filepath.Join(folder, shortcutFile(newLabel, code))
			if err := os.WriteFile(newPath, []byte(strings.Join(lines, "\n")), 0o755); err != nil {
				failed = append(failed, filepath.Base(oldPath)+": "+err.Error())
				continue
			}
			if newPath != oldPath {
				os.Remove(oldPath)
			}
			if place == placeDesktop {
				_ = exec.Command("gio", "set", newPath, "metadata::trusted", "true").Run()
			}
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not update shortcuts: %s", strings.Join(failed, "; "))
	}
	return nil
}

func removeShortcuts(label string) {
	for _, place := range []string{placeDesktop, placeStart} {
		if folder, err := shortcutDir(place); err == nil {
			for _, code := range []bool{false, true} {
				os.Remove(filepath.Join(folder, shortcutFile(label, code)))
			}
		}
	}
}
