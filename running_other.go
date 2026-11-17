//go:build !windows

package main

func desktopRunning(folder string) bool { return false }
