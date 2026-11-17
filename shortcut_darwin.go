//go:build darwin

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shortcutDir: the Desktop, or ~/Applications/Claude Profiles, which
// Launchpad and Spotlight pick up.
func shortcutDir(place string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch place {
	case placeDesktop:
		return filepath.Join(home, "Desktop"), nil
	case placeStart:
		return filepath.Join(home, "Applications", "Claude Profiles"), nil
	}
	return "", fmt.Errorf("unknown shortcut place %q (use desktop or start)", place)
}

func shortcutName(label string, code bool) string {
	app := "Claude"
	if code {
		app = "Claude Code"
	}
	return strings.NewReplacer("/", "_", ":", "_").Replace(app+" - "+strings.TrimSpace(label)) + ".app"
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// icns wraps the 256px PNG icon in a minimal .icns file.
func icns() []byte {
	var b bytes.Buffer
	b.WriteString("icns")
	binary.Write(&b, binary.BigEndian, uint32(8+8+len(iconPNG)))
	b.WriteString("ic08") // 256x256 PNG
	binary.Write(&b, binary.BigEndian, uint32(8+len(iconPNG)))
	b.Write(iconPNG)
	return b.Bytes()
}

// createShortcut builds a tiny .app bundle whose launcher runs this program
// for r. Claude Code bundles open Terminal, where Claude Code then runs.
func (a *app) createShortcut(r *Record, place string, code bool, dir string) (string, error) {
	folder, err := shortcutDir(place)
	if err != nil {
		return "", err
	}
	exe, err := selfExe()
	if err != nil {
		return "", err
	}
	args, _, err := a.shortcutArgs(r, code, strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}
	name := strings.TrimSuffix(shortcutName(r.Label, code), ".app")
	bundle := filepath.Join(folder, name+".app")
	if err := os.RemoveAll(bundle); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "Resources"), 0o755); err != nil {
		return "", err
	}

	launcher := "#!/bin/sh\nexec " + shQuote(exe)
	for _, s := range args {
		launcher += " " + shQuote(s)
	}
	launcher += "\n"
	kind := ""
	if code {
		kind = ".code"
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>` + xmlEscape(name) + `</string>
	<key>CFBundleDisplayName</key><string>` + xmlEscape(name) + `</string>
	<key>CFBundleIdentifier</key><string>io.github.hadiaghandeh.claude-profiles.` + safeFolderName(r.Label) + kind + `</string>
	<key>CFBundleExecutable</key><string>launcher</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>LSUIElement</key><true/>
</dict>
</plist>
`
	files := []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{filepath.Join(bundle, "Contents", "MacOS", "launcher"), []byte(launcher), 0o755},
		{filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0o644},
		{filepath.Join(bundle, "Contents", "Resources", "icon.icns"), icns(), 0o644},
	}
	for _, f := range files {
		if err := os.WriteFile(f.path, f.data, f.mode); err != nil {
			return "", err
		}
		if err := os.Chmod(f.path, f.mode); err != nil {
			return "", err
		}
	}
	return bundle, nil
}

// renameShortcuts moves a profile's launcher bundles to the new name and
// points them at the new label.
func renameShortcuts(oldLabel, newLabel string) error {
	var failed []string
	for _, place := range []string{placeDesktop, placeStart} {
		folder, err := shortcutDir(place)
		if err != nil {
			continue
		}
		for _, code := range []bool{false, true} {
			oldBundle := filepath.Join(folder, shortcutName(oldLabel, code))
			if !exists(oldBundle) {
				continue
			}
			newBundle := filepath.Join(folder, shortcutName(newLabel, code))
			if err := os.Rename(oldBundle, newBundle); err != nil {
				failed = append(failed, filepath.Base(oldBundle)+": "+err.Error())
				continue
			}
			oldName := strings.TrimSuffix(shortcutName(oldLabel, code), ".app")
			newName := strings.TrimSuffix(shortcutName(newLabel, code), ".app")
			edits := map[string][2]string{
				filepath.Join(newBundle, "Contents", "MacOS", "launcher"): {" -profile " + shQuote(oldLabel), " -profile " + shQuote(newLabel)},
				filepath.Join(newBundle, "Contents", "Info.plist"):        {">" + xmlEscape(oldName) + "<", ">" + xmlEscape(newName) + "<"},
			}
			for path, e := range edits {
				raw, err := os.ReadFile(path)
				if err != nil {
					failed = append(failed, err.Error())
					continue
				}
				info, _ := os.Stat(path)
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), e[0], e[1])), info.Mode()); err != nil {
					failed = append(failed, err.Error())
				}
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
				os.RemoveAll(filepath.Join(folder, shortcutName(label, code)))
			}
		}
	}
}
