// S04 — WindowService: the EngineHost.window host-side bindings (S00
// CONTRACTS §3). Routes to the popover/settings windows and the app's quit
// and clipboard facilities. No-op-safe on nil references.
package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowService holds the app plus the popover window so bound methods can
// show/hide windows, quit, and copy to the clipboard. All methods are
// idempotent and nil-safe.
type WindowService struct {
	app     *application.App
	popover *application.WebviewWindow
}

// newWindowService builds the host WindowService. Callers pass the app and
// the popover window once both exist (S04 SPEC §2.2).
func newWindowService(app *application.App, popover *application.WebviewWindow) *WindowService {
	return &WindowService{app: app, popover: popover}
}

// OpenSettings lazily creates (if needed) and shows the settings window.
func (w *WindowService) OpenSettings() error {
	if w.app == nil {
		return nil
	}
	showSettings(w.app)
	return nil
}

// CloseSettings hides the settings window (close = hide; the window is
// never destroyed while the app runs, S03 SPEC §2.5).
func (w *WindowService) CloseSettings() error {
	hideSettings()
	return nil
}

// HidePopover hides the popover, never closing it (S02 SPEC §2.7).
func (w *WindowService) HidePopover() error {
	hidePopover(w.popover)
	return nil
}

// SetPopoverHeight resizes the popover window to the measured natural height
// of its content, clamped to [320, 620]. The frontend calls this whenever the
// active view's height changes, which is what keeps the window content-sized
// like the design's panel instead of stretching a fixed 620 with filler.
func (w *WindowService) SetPopoverHeight(height int) error {
	resizePopover(w.popover, height)
	return nil
}

// SetTrayPick puts the popover's current pick in the menu bar: the profile it
// is ranking for, and the model that came out on top.
//
// The host ranks for the menu bar on its own at startup and on catalog/config
// events, but it cannot see the popover's state — the active profile and the
// ephemeral weight overrides live in the webview and never reach the engine as
// config. So the popover pushes what it is showing, and from the first push it
// owns the title: whatever the user does with the scale or the sliders, the
// menu bar says the same thing the popover does.
func (w *WindowService) SetTrayPick(profileName, modelName, reasoning, provider string) error {
	setTrayPickFromUI(profileName, modelName, reasoning, provider)
	return nil
}

// Quit terminates the application cleanly.
func (w *WindowService) Quit() error {
	if w.app != nil {
		w.app.Quit()
	}
	return nil
}

// CopyToClipboard sets the system clipboard to text via the Wails clipboard
// manager. A nil app is a silent no-op.
func (w *WindowService) CopyToClipboard(text string) error {
	if w.app == nil || w.app.Clipboard == nil {
		return nil
	}
	w.app.Clipboard.SetText(text)
	return nil
}
