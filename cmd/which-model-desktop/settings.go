// Settings window (S03). A lazy singleton resizable WebviewWindow hosting the
// settings.html entry, opening at 820x560. Created on first showSettings, never
// destroyed while the app runs: the native close action is intercepted and
// turned into a Hide() so webview state survives across open/close cycles (S00
// §4 decision "Settings close = hide").
//
// macOS uses the hidden titlebar over full-size content: AppKit draws the real
// traffic lights — standard size, standard hover symbols, working zoom — and
// the page draws its own draggable title row underneath, reserving 78px on the
// left for them (U07 SettingsShell). The page draws no window buttons itself.
package main

import (
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Settings geometry. 820x560 (520 content + ~40 web titlebar) is the OPENING
// size, no longer the only one: the window is resizable, so these are a
// starting point plus a floor small enough to keep the sidebar and one content
// column readable. S03 SPEC §2.2 specified a fixed window; that is relaxed
// deliberately — several pages (Providers, Benchmark groups) carry tables that
// were truncating at 820.
const (
	settingsName      = "settings"
	settingsTitle     = "which-model settings"
	settingsWidth     = 820
	settingsHeight    = 560
	settingsMinWidth  = 720
	settingsMinHeight = 460
)

// settingsBackground is nocturne's --color-bg (#161826) — the settings page's
// own background, mirrored onto the native window so the two never disagree.
var settingsBackground = application.NewRGBA(0x16, 0x18, 0x26, 255)

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
		Width:  settingsWidth,
		Height: settingsHeight,
		// Floor only; no Max*, so zoom and the resize grips work.
		MinWidth:    settingsMinWidth,
		MinHeight:   settingsMinHeight,
		Hidden:      true, // shown only by showSettings
		Frameless:   false,
		AlwaysOnTop: false,
		URL:           "/settings.html",
		// Nocturne --color-bg, so the frame behind the webview matches the page
		// instead of flashing white before first paint.
		BackgroundColour: settingsBackground,
		// The real AppKit window buttons: standard size, standard spacing, and
		// the hover symbols and press states that only the OS draws. The
		// web-drawn set that replaced them earlier could not reproduce those,
		// and with the window now resizable, zoom is a live control rather than
		// the inert dot a fixed-size window justified.
		Mac: application.MacWindow{
			// Hidden titlebar over full-size content: the page still draws its
			// own title row, and AppKit insets the buttons into it.
			TitleBar: application.MacTitleBarHidden,
			// The UI is dark-only; force dark chrome so the window frame
			// matches it on light-mode systems too.
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})

	w.Center()

	// Even padding above, below and left of the window buttons; AppKit's own
	// placement assumes its 28pt titlebar, not the page's 40px title row.
	positionTrafficLights(w)

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
