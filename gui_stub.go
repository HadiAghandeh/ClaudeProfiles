//go:build !gui || !(windows || linux)

package main

import (
	"errors"
	"fmt"
	"os"
)

// guiBuild is true only in claude-profiles-gui (built with -tags gui on
// Windows or Linux).
const guiBuild = false

func runGUI(string) error { return errors.New("this build has no window; build with -tags gui") }

func showError(msg string) { fmt.Fprintln(os.Stderr, "Error:", msg) }
