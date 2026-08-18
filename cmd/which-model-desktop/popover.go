// Popover window (S02 SPEC §2.5/§2.7). One frameless 400x620 WebviewWindow,
// hidden at startup, always-on-top, loading the "/" index.html popover entry.
// It is shown/toggled by the tray and hides (never closes/destroys) on focus
// loss or Escape, preserving webview state across show/hide cycles.
package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// popoverTitle is the window title (not rendered — frameless).
const popoverTitle = "which-model"

// popoverSolidBackground is the mockup's --color-bg #0d0f12, used because no
// transparent backdrop is required for the frontend-drawn rounded corners
// (S02 CONTRACTS §3).
var popoverSolidBackground = application.NewRGBA(13, 15, 18, 255)

// newPopoverWindow creates the hidden popover window with the exact options
// in S02 CONTRACTS §3. Focus loss and Escape both hide it (S02 SPEC §2.7).
func newPopoverWindow(app *application.App) *application.WebviewWindow {
	return app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "popover",
		Title:            popoverTitle,
		Width:            400,
		Height:           620,
		Frameless:        true,
		Hidden:           true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		MinWidth:         400,
		MinHeight:        620,
		MaxWidth:         400,
		MaxHeight:        620,
		URL:              "/",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: popoverSolidBackground,
		HideOnFocusLost:  true, // S02 SPEC §2.7: hide on focus loss
		HideOnEscape:     true, // S02 SPEC §2.7: hide on Escape
	})
}

// showPopover shows the popover and brings it to the front. It never creates
// a window — the caller guarantees w is non-nil (created by newPopoverWindow).
func showPopover(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	w.Show()
	w.Focus()
}

// hidePopover hides the popover, never closing/destroying it (S02 SPEC §2.7).
func hidePopover(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	w.Hide()
}

// togglePopover flips the popover's visibility. Used by the fallback-ladder
// steps 2/4 (S02 CONTRACTS §5); the primary path relies on the tray's
// AttachWindow built-in toggle.
func togglePopover(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	if w.IsVisible() {
		hidePopover(w)
	} else {
		showPopover(w)
	}
}
