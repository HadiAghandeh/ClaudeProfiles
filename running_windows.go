//go:build windows

package main

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// desktopRunning reports whether Claude Desktop is open with this data folder.
// While it runs, Claude Desktop (like other Chromium-based apps) keeps a file
// named "lockfile" in its data folder open without sharing, so opening it
// fails with a sharing violation. Checking costs one file open.
func desktopRunning(folder string) bool {
	p, err := windows.UTF16PtrFromString(filepath.Join(folder, "lockfile"))
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		windows.CloseHandle(h)
		return false // a leftover lockfile from a closed app
	}
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
