//go:build darwin || linux

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// iconPNG is the app icon (256x256), used for Linux menu entries, macOS
// shortcut bundles and the Linux window.
//
//go:embed winres/icon.png
var iconPNG []byte

// iconFile writes the icon next to the profile data (once) and returns its path.
func iconFile() string {
	_, codeRoot, err := dataRoots()
	if err != nil {
		return ""
	}
	path := filepath.Join(filepath.Dir(codeRoot), "icon.png")
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(iconPNG)) {
		return path
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, iconPNG, 0o644) != nil {
		return ""
	}
	return path
}

// selfExe is this program's resolved path, which shortcuts run.
func selfExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// shortcutArgs are the command-line arguments a shortcut passes, after
// checking the Claude Code folder. It also returns the folder to start in.
func (a *app) shortcutArgs(r *Record, code bool, dir string) (args []string, workDir string, err error) {
	args = []string{"-profile", r.Label}
	if def, err := defaultRecordsFile(); err == nil && pathKey(def) != pathKey(a.recordsFile) {
		args = append(args, "-profiles-file", a.recordsFile)
	}
	if workDir, err = os.UserHomeDir(); err != nil {
		return nil, "", err
	}
	if code {
		args = append(args, "-code")
		if dir != "" {
			if dir, err = filepath.Abs(dir); err != nil {
				return nil, "", err
			}
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return nil, "", fmt.Errorf("folder not found: %s", dir)
			}
			args = append(args, "-dir", dir)
			workDir = dir
		}
	}
	return args, workDir, nil
}
