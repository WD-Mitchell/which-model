//go:build !darwin || ios

// Non-macOS stub for the tray left-click repair. The bug being worked around is
// specific to Wails' macOS NSStatusItem wiring (see traybutton_darwin.m); the
// Windows and Linux system-tray backends deliver clicks through their own paths
// and SystemTray.OnClick works there, so this is a no-op and setupTray's
// tray.OnClick registration is the live one.
package main

func installTrayButtonHandler(_ func()) {}

func testClickStatusButton() bool { return false }

func statusButtonFrameCG() (x, y, w, h float64, ok bool) { return 0, 0, 0, 0, false }
