// Settings window (S03). A lazy singleton 820x560 WebviewWindow hosting the
// settings.html entry. Created on first showSettings, never destroyed while
// the app runs: the native close action is intercepted and turned into a
// Hide() so webview state survives across open/close cycles (S00 §4 decision
// "Settings close = hide"). macOS uses the hidden-inset titlebar variant; the
// web page draws its draggable titlebar over the full-size content.
package main

import (
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Settings geometry (S03 SPEC §2.2): 820 wide, 560 total height (520 content
// + ~40 web titlebar), fixed: min = max = 820x560, resize disabled.
const (
	settingsName   = "settings"
	settingsTitle  = "which-model settings"
	settingsWidth  = 820
	settingsHeight = 560
)

var (
	// settingsMu guards settingsWin; creation may fail and be retried, so this
	// is a mutex + nil check, not sync.Once (S03 SPEC §3).
	settingsMu  sync.Mutex
	settingsWin *application.WebviewWindow

	// settingsQuitting is set by the app quit path (main wires it to the app's
	// OnShutdown callback). The close-intercept hook checks it so user-initiated
	// closes hide the window while an app-quit close is allowed through (S03
	// SPEC §2.7). Guarded by settingsMu.
	settingsQuitting bool
)

// setQuitting marks the app as quitting so the settings close hook stops
// intercepting (S03 SPEC §2.7). Called once from the app-shutdown path.
func setQuitting(quitting bool) {
	settingsMu.Lock()
	settingsQuitting = quitting
	settingsMu.Unlock()
}

// applicationIsQuitting reports whether the app-quit path has run.
func applicationIsQuitting() bool {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return settingsQuitting
}

// ensureSettingsWindow returns the settings window, creating it on first call
// with the S03 CONTRACTS §3 options, centring it, and installing the
// close-intercept hook. It never shows the window itself. It returns an error
// only when window creation fails; callers log and may retry.
func ensureSettingsWindow(app *application.App) (*application.WebviewWindow, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	if settingsWin != nil {
		return settingsWin, nil
	}

	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          settingsName,
		Title:         settingsTitle,
		Width:         settingsWidth,
		Height:        settingsHeight,
		MinWidth:      settingsWidth,
		MinHeight:     settingsHeight,
		MaxWidth:      settingsWidth,
		MaxHeight:     settingsHeight,
		DisableResize: true,
		Hidden:        true, // shown only by showSettings
		Frameless:     false,
		AlwaysOnTop:   false,
		URL:           "/settings.html",
		Mac: application.MacWindow{
			// Traffic-light buttons inset over web content, no native title.
			TitleBar: application.MacTitleBarHiddenInset,
		},
	})

	w.Center()

	// Close = hide: cancel the native close (traffic light, Cmd-W, menu) and
	// Hide the window instead of destroying it — unless the app is quitting,
	// in which case the close proceeds so teardown is never blocked (S03 SPEC
	// §2.5/§2.7). Hook runs before the default destroy listener; calling
	// Cancel() prevents the window from being removed/marked destroyed.
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if applicationIsQuitting() {
			return // allow teardown
		}
		event.Cancel()
		w.Hide()
	})

	settingsWin = w
	return w, nil
}

// showSettings ensures the window exists, then Show + un-minimise + Focus.
// Idempotent; creation failure is logged and swallowed (S03 SPEC §3).
func showSettings(app *application.App) {
	w, err := ensureSettingsWindow(app)
	if err != nil {
		log.Printf("settings: failed to create settings window: %v", err)
		return
	}
	if w.IsMinimised() {
		w.Restore()
	}
	w.Show()
	w.Focus()
}

// hideSettings hides the window if it exists; no-op (no creation) otherwise.
func hideSettings() {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if settingsWin != nil {
		settingsWin.Hide()
	}
}
