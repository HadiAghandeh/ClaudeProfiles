//go:build !windows

package main

import "errors"

const (
	placeDesktop = "desktop"
	placeStart   = "start"
)

func (a *app) createShortcut(r *Record, place string, code bool, dir string) (string, error) {
	return "", errors.New("shortcuts are only supported on Windows")
}

func removeShortcuts(label string) {}

func renameShortcuts(oldLabel, newLabel string) error { return nil }
