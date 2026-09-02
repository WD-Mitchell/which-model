//go:build !darwin || ios

// Non-macOS stub. Windows and Linux manage taskbar presence per-window
// automatically through their window managers when the window is shown or hidden.
package main

var stubDockVisible bool

func setDockIconVisible(visible bool) {
	stubDockVisible = visible
}

func dockIconVisible() bool {
	return stubDockVisible
}
