// Popover window (S02 SPEC §2.5/§2.7). One frameless 500px-wide WebviewWindow,
// hidden at startup, always-on-top, loading the "/" index.html popover entry.
// It is shown/toggled by the tray and hides (never closes/destroys) on focus
// loss or Escape, preserving webview state across show/hide cycles.
//
// Hide-on-focus-loss is implemented HERE rather than with Wails'
// HideOnFocusLost option, because that option loses a race with AppKit on
// macOS and made left-clicking the menu-bar icon look like a no-op:
//
//   - NSStatusBarButton is configured to fire its action on mouse-DOWN
//     (systemtray_darwin.m:36 `[button sendActionOn:(NSEventMaskLeftMouseDown|
//     NSEventMaskRightMouseDown)]`), then runs a nested highlight/tracking loop
//     until mouse-UP, which re-keys the status bar's own window.
//   - HideOnFocusLost is a bare `w.Hide()` on WindowLostFocus
//     (webview_window.go:406-408), so the popover was shown by the click and
//     immediately hidden again by the tracking loop stealing key — before the
//     user had even released the button.
//
// Instead the window keeps focus loss unhooked by Wails and we hide it
// ourselves, ignoring any blur that lands within popoverBlurGrace of a show
// (that blur is always the tracking loop, never the user). Right-click was
// unaffected because the native NSMenu path never fires the window's action.
package main

import (
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// popoverTitle is the window title (not rendered — frameless).
const popoverTitle = "which-model"

// popoverBlurGrace is how long after a show a focus loss is attributed to the
// status-item mouse-down tracking loop rather than to the user clicking away.
// The loop lasts exactly as long as the button is held; 500ms covers a
// deliberate press without making a real click-away feel sticky.
const popoverBlurGrace = 500 * time.Millisecond

// popoverFocusReclaim is how long after a swallowed blur we re-assert key on
// the popover. Without this the window can stay visible-but-not-key after the
// tracking loop ends, and a later click outside would produce no further
// WindowLostFocus — i.e. the popover would never hide again.
const popoverFocusReclaim = 250 * time.Millisecond

// popoverSolidBackground matches the panel ground (nocturne --color-bg
// #161826) so the one-frame gap between a content resize and the webview
// repaint is invisible. The previous #0d0f12 predated the Nocturne retune.
var popoverSolidBackground = application.NewRGBA(22, 24, 38, 255)

// Popover height bounds for content-driven resizing (SetPopoverHeight). The
// design's panel is content-height — the landing view is ~450pt against the
// old fixed 620, and that ~170pt surplus was absorbed by a flex spacer BETWEEN
// the profile name and the results band, which is exactly the "too much space"
// it produced. Width is pinned at 500 — 25% over the mockup's 400 (S02
// CONTRACTS §3), which the weights editor's label + slider + value row needs.
const (
	popoverWidth     = 500
	popoverMinHeight = 320
	popoverMaxHeight = 620
)

// popoverTrayRef is the tray the popover docks under, published by setupTray so
// a resize can re-anchor the visible window (Cocoa keeps the BOTTOM edge fixed
// on resize, so without repositioning a shrink would leave a gap under the
// menu bar and a grow would run off the bottom).
var popoverTrayRef *application.SystemTray

var (
	// popoverMu guards the show bookkeeping below. All of it is touched from
	// the AppKit main thread (clicks) and from Wails' window-event goroutines.
	popoverMu sync.Mutex
	// popoverShownAt is when the popover was last shown; the zero value means
	// "never shown", which shouldHideOnBlur treats as out of grace.
	popoverShownAt time.Time
	// popoverShowGen increments on every show so a pending focus reclaim from
	// an older show cannot fire against a newer one.
	popoverShowGen uint64
	// popoverReclaimed makes the focus reclaim at most once per show.
	popoverReclaimed bool
	// popoverFocusTimer is the pending focus reclaim timer, tracked so it can
	// be cancelled during hide and application shutdown.
	popoverFocusTimer *time.Timer
)

// newPopoverWindow creates the hidden popover window with the S02 CONTRACTS §3
// options and installs the blur handler described in the file comment. Escape
// still hides via Wails' own HideOnEscape (S02 SPEC §2.7).
func newPopoverWindow(app *application.App) *application.WebviewWindow {
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "popover",
		Title:            popoverTitle,
		Width:            popoverWidth,
		Height:           620,
		Frameless:        true,
		Hidden:           true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		MinWidth:         popoverWidth,
		MinHeight:        620,
		MaxWidth:         popoverWidth,
		MaxHeight:        620,
		URL:              "/",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: popoverSolidBackground,
		// S02 SPEC §2.7 hide-on-focus-loss is ours, not Wails' — see the file
		// comment for why the built-in loses the mouse-down race.
		HideOnFocusLost: false,
		HideOnEscape:    true, // S02 SPEC §2.7: hide on Escape
	})

	// events.Common.WindowLostFocus is the same event Wails' own
	// setupHideOnFocusLost uses (webview_window.go:406), so it is known to fire
	// for a frameless always-on-top window on every platform.
	w.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		onPopoverBlur(w, time.Now())
	})

	return w
}

// onPopoverBlur is the WindowLostFocus body, split out so the timing rule is
// testable through shouldHideOnBlur.
func onPopoverBlur(w *application.WebviewWindow, now time.Time) {
	if w == nil {
		return
	}
	popoverMu.Lock()
	shownAt := popoverShownAt
	popoverMu.Unlock()

	if shouldHideOnBlur(shownAt, now, popoverBlurGrace) {
		w.Hide()
		return
	}
	reclaimPopoverFocus(w)
}

// shouldHideOnBlur reports whether a focus loss at `now` is a real click-away
// (true) or the status-item tracking loop stealing key immediately after the
// popover was shown at `shownAt` (false). A zero shownAt — the popover was
// never shown from the tray — is always out of grace.
func shouldHideOnBlur(shownAt, now time.Time, grace time.Duration) bool {
	if shownAt.IsZero() {
		return true
	}
	return now.Sub(shownAt) >= grace
}

// reclaimPopoverFocus re-asserts key on the popover once the status-item
// tracking loop has ended. At most one reclaim runs per show, and it is
// abandoned if the popover has since been re-shown or hidden.
func reclaimPopoverFocus(w *application.WebviewWindow) {
	popoverMu.Lock()
	if popoverReclaimed {
		popoverMu.Unlock()
		return
	}
	popoverReclaimed = true
	gen := popoverShowGen
	if popoverFocusTimer != nil {
		popoverFocusTimer.Stop()
		popoverFocusTimer = nil
	}
	popoverFocusTimer = time.AfterFunc(popoverFocusReclaim, func() {
		popoverMu.Lock()
		stale := popoverShowGen != gen
		popoverMu.Unlock()
		if stale || applicationIsQuitting() || !w.IsVisible() {
			return
		}
		w.Focus()
	})
	popoverMu.Unlock()
}

// markPopoverShown records a show for the grace-period rule. Called on every
// path that makes the popover visible (tray click, tray menu, hotkey, second
// instance launch).
func markPopoverShown() {
	popoverMu.Lock()
	popoverShownAt = time.Now()
	popoverShowGen++
	popoverReclaimed = false
	popoverMu.Unlock()
}

// showPopover shows the popover and brings it to the front. Idempotent and
// nil-safe; it never creates a window.
func showPopover(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	// Marked before Show so a blur arriving between Show and Focus is inside
	// the grace window.
	markPopoverShown()
	w.Show()
	w.Focus()
}

// showPopoverAt positions the popover under the tray icon and shows it. tray
// may be nil (hotkey path, or the tray hidden by show_menu_bar_icon=false), in
// which case the window keeps its last position.
func showPopoverAt(tray *application.SystemTray, w *application.WebviewWindow) {
	if w == nil {
		return
	}
	if tray != nil {
		// Errors here only mean "tray not running yet"; showing an
		// unpositioned popover beats not showing one (S02 SPEC §3). Logged
		// rather than swallowed: a silent failure here leaves the window at
		// whatever position it last had, which is how it ended up hanging off
		// the right edge of the screen (measured: pos=(1231,41) w=399 on a
		// 1440pt-wide display, ~190pt off-screen).
		if err := tray.PositionWindow(w, 0); err != nil {
			log.Printf("popover: position under tray failed: %v", err)
		}
	}
	showPopover(w)
	// Whatever positioned it — the tray or a stale frame — the popover must end
	// up fully on screen. A half-off-screen popover is indistinguishable from a
	// click that did nothing.
	clampPopoverToScreen(w)
}

// hidePopover hides the popover, never closing/destroying it (S02 SPEC §2.7).
// Idempotent and nil-safe.
func hidePopover(w *application.WebviewWindow) {
	popoverMu.Lock()
	if popoverFocusTimer != nil {
		popoverFocusTimer.Stop()
		popoverFocusTimer = nil
	}
	popoverMu.Unlock()
	if w == nil {
		return
	}
	w.Hide()
}

// cancelPopoverTimers stops the tracked focus reclaim timer, if any.
func cancelPopoverTimers() {
	popoverMu.Lock()
	defer popoverMu.Unlock()
	if popoverFocusTimer != nil {
		popoverFocusTimer.Stop()
		popoverFocusTimer = nil
	}
}

// togglePopover flips the popover's visibility (hotkey path, S05 SPEC §2.1).
func togglePopover(w *application.WebviewWindow) {
	togglePopoverAt(nil, w)
}

// clampPopoverHeight bounds a requested content height. Pure, for tests.
func clampPopoverHeight(h int) int {
	if h < popoverMinHeight {
		return popoverMinHeight
	}
	if h > popoverMaxHeight {
		return popoverMaxHeight
	}
	return h
}

// resizePopover sets the popover's height to fit the measured content (the
// frontend calls this through WindowService.SetPopoverHeight whenever the
// active view's natural height changes). Min/max are moved together with the
// size — the window is created popoverWidth x 620 with equal min=max, which would
// otherwise clamp every SetSize back to 620. A visible window is re-anchored
// under the tray afterwards.
func resizePopover(w *application.WebviewWindow, height int) {
	if w == nil {
		return
	}
	h := clampPopoverHeight(height)
	_, cur := w.Size()
	if cur == h {
		return
	}
	w.SetMinSize(popoverWidth, h)
	w.SetMaxSize(popoverWidth, h)
	w.SetSize(popoverWidth, h)
	if w.IsVisible() {
		if popoverTrayRef != nil {
			_ = popoverTrayRef.PositionWindow(w, 0)
		}
		clampPopoverToScreen(w)
	}
}

// togglePopoverAt is togglePopover with tray-relative positioning on the show
// half — the tray's OnClick handler (S02 SPEC §2.6).
func togglePopoverAt(tray *application.SystemTray, w *application.WebviewWindow) {
	if w == nil {
		return
	}
	if w.IsVisible() {
		hidePopover(w)
		return
	}
	showPopoverAt(tray, w)
}

// clampPopoverToScreen nudges the popover fully inside its screen's visible
// frame, preserving its position otherwise. No-op when the geometry is
// unavailable (zero-sized screen), and safe to call on every show.
func clampPopoverToScreen(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	screen, err := w.GetScreen()
	if err != nil || screen == nil {
		log.Printf("popover: clamp skipped, GetScreen err=%v", err)
		return
	}
	work := screen.WorkArea
	if work.Width <= 0 || work.Height <= 0 {
		return
	}

	x, y := w.Position()
	width, height := w.Size()
	nx, ny := clampWindow(x, y, width, height, work)
	log.Printf("popover: shown at (%d,%d) %dx%d, work area %dx%d+%d+%d",
		x, y, width, height, work.Width, work.Height, work.X, work.Y)
	if nx != x || ny != y {
		log.Printf("popover: clamped from (%d,%d) to (%d,%d) within %dx%d+%d+%d",
			x, y, nx, ny, work.Width, work.Height, work.X, work.Y)
		w.SetPosition(nx, ny)
	}
}

// clampWindow returns the nearest position that keeps a width x height window
// inside work. Pure, so the arithmetic is unit-testable without a screen.
func clampWindow(x, y, width, height int, work application.Rect) (int, int) {
	maxX := work.X + work.Width - width
	maxY := work.Y + work.Height - height
	if x > maxX {
		x = maxX
	}
	if x < work.X {
		x = work.X
	}
	if y > maxY {
		y = maxY
	}
	if y < work.Y {
		y = work.Y
	}
	return x, y
}
