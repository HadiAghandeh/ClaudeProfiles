//go:build !(gui && windows)

package main

import (
	"errors"
	"fmt"
	"os"
)

// guiBuild is true only in claude-profiles-gui.exe (built with -tags gui).
const guiBuild = false

func runGUI(string) error { return errors.New("this build has no window; build with -tags gui") }

func showError(msg string) { fmt.Fprintln(os.Stderr, "Error:", msg) }
