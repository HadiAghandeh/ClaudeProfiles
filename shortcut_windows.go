//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// Where shortcuts can be placed.
const (
	placeDesktop = "desktop"
	placeStart   = "start"
)

// startMenuFolder groups this app's shortcuts in the Start menu.
const startMenuFolder = "Claude Profiles"

func shortcutDir(place string) (string, error) {
	switch place {
	case placeDesktop:
		// The real Desktop, also when OneDrive redirects it.
		return windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
	case placeStart:
		programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
		if err != nil {
			return "", err
		}
		return filepath.Join(programs, startMenuFolder), nil
	}
	return "", fmt.Errorf("unknown shortcut place %q (use desktop or start)", place)
}

var unsafeFileChars = strings.NewReplacer(`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")

func shortcutName(label string, code bool) string {
	app := "Claude"
	if code {
		app = "Claude Code"
	}
	return unsafeFileChars.Replace(app+" - "+strings.TrimSpace(label)) + ".lnk"
}

// launcherExe is what shortcuts run. The windowed build is preferred because it
// starts without flashing a console window.
func launcherExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if !guiBuild {
		if gui := filepath.Join(filepath.Dir(exe), "claude-profiles-gui.exe"); exists(gui) {
			return gui, nil
		}
	}
	return exe, nil
}

// createShortcut writes a .lnk that starts Claude Desktop (or Claude Code, in
// dir) as r, and returns its path. An existing shortcut of the same name is
// replaced.
func (a *app) createShortcut(r *Record, place string, code bool, dir string) (string, error) {
	folder, err := shortcutDir(place)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	target, err := launcherExe()
	if err != nil {
		return "", err
	}

	args := []string{"-profile", r.Label}
	if !strings.EqualFold(a.recordsFile, filepath.Join(filepath.Dir(target), "profiles.json")) {
		args = append(args, "-profiles-file", a.recordsFile)
	}
	workDir := os.Getenv("USERPROFILE")
	desc := "Open Claude Desktop as " + r.Label
	if code {
		args = append(args, "-code")
		if dir = strings.Trim(strings.TrimSpace(dir), `"`); dir != "" {
			if dir, err = filepath.Abs(dir); err != nil {
				return "", err
			}
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return "", fmt.Errorf("folder not found: %s", dir)
			}
			args = append(args, "-dir", dir)
			workDir = dir
		}
		desc = "Open Claude Code as " + r.Label
	}
	for i, s := range args {
		args[i] = windows.EscapeArg(s)
	}

	path := filepath.Join(folder, shortcutName(r.Label, code))
	const script = `$ErrorActionPreference = 'Stop'
$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:LNK_PATH)
$s.TargetPath = $env:LNK_TARGET
$s.Arguments = $env:LNK_ARGS
$s.WorkingDirectory = $env:LNK_DIR
$s.IconLocation = $env:LNK_ICON
$s.Description = $env:LNK_DESC
$s.Save()`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePS(script))
	cmd.Env = append(os.Environ(),
		"LNK_PATH="+path,
		"LNK_TARGET="+target,
		"LNK_ARGS="+strings.Join(args, " "),
		"LNK_DIR="+workDir,
		"LNK_ICON="+target+",0",
		"LNK_DESC="+desc,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New("could not create the shortcut: " + msg)
	}
	return path, nil
}

// renameShortcuts moves a profile's existing shortcuts to the new name and
// points them at the new label, keeping everything else (folder, target).
func renameShortcuts(oldLabel, newLabel string) error {
	const script = `$ErrorActionPreference = 'Stop'
$sh = New-Object -ComObject WScript.Shell
$o = $sh.CreateShortcut($env:LNK_OLD)
$n = $sh.CreateShortcut($env:LNK_NEW)
$n.TargetPath = $o.TargetPath
$n.Arguments = $o.Arguments.Replace($env:ARG_OLD, $env:ARG_NEW)
$n.WorkingDirectory = $o.WorkingDirectory
$n.IconLocation = $o.IconLocation
$n.Description = $o.Description.Replace($env:LABEL_OLD, $env:LABEL_NEW)
$n.Save()
if ($env:DELETE_OLD -eq '1') { Remove-Item -LiteralPath $env:LNK_OLD }`
	var failed []string
	for _, place := range []string{placeDesktop, placeStart} {
		folder, err := shortcutDir(place)
		if err != nil {
			continue
		}
		for _, code := range []bool{false, true} {
			oldPath := filepath.Join(folder, shortcutName(oldLabel, code))
			newPath := filepath.Join(folder, shortcutName(newLabel, code))
			if !exists(oldPath) {
				continue
			}
			deleteOld := "1"
			if strings.EqualFold(oldPath, newPath) { // only the letter case changed: same file
				deleteOld = "0"
			}
			cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePS(script))
			cmd.Env = append(os.Environ(),
				"LNK_OLD="+oldPath,
				"LNK_NEW="+newPath,
				"ARG_OLD=-profile "+windows.EscapeArg(oldLabel),
				"ARG_NEW=-profile "+windows.EscapeArg(newLabel),
				"LABEL_OLD="+oldLabel,
				"LABEL_NEW="+newLabel,
				"DELETE_OLD="+deleteOld,
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if out, err := cmd.CombinedOutput(); err != nil {
				failed = append(failed, filepath.Base(oldPath)+": "+strings.TrimSpace(string(out)))
			}
		}
	}
	if len(failed) > 0 {
		return errors.New("could not update shortcuts: " + strings.Join(failed, "; "))
	}
	return nil
}

// removeShortcuts deletes the Desktop and Start menu shortcuts this app made
// for label. Errors are ignored: a missing shortcut is fine.
func removeShortcuts(label string) {
	for _, place := range []string{placeDesktop, placeStart} {
		folder, err := shortcutDir(place)
		if err != nil {
			continue
		}
		for _, code := range []bool{false, true} {
			os.Remove(filepath.Join(folder, shortcutName(label, code)))
		}
		if place == placeStart {
			os.Remove(folder) // only succeeds once the folder is empty
		}
	}
}
