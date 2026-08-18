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