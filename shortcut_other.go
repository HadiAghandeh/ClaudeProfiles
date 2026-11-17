//go:build !windows && !linux && !darwin

package main

import "errors"

func (a *app) createShortcut(r *Record, place string, code bool, dir string) (string, error) {
	return "", errors.New("shortcuts are not supported on this operating system")
}

func removeShortcuts(label string) {}

func renameShortcuts(oldLabel, newLabel string) error { return nil }
