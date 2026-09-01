// S05 — hotkey: the global shortcut that toggles the popover (S00 CONTRACTS
// §5, S05 SPEC §2). parseShortcut resolves a GUISettings.Shortcut string to
// the hotkey registration inputs; the platform-specific modifier slices come
// from the build-tagged modifiers_*.go files (the modifier constants do not
// coexist across GOOS).
package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"
)

// parseShortcut maps GUISettings.Shortcut → hotkey registration inputs.
// Unknown input returns the alt+space mapping and ok=false (caller emits
// notice; S05 SPEC §3).
func parseShortcut(s string) ([]hotkey.Modifier, hotkey.Key, bool) {
	switch s {
	case "alt+space":
		return altSpaceModifiers(), hotkey.KeySpace, true
	case "ctrl+space":
		return []hotkey.Modifier{hotkey.ModCtrl}, hotkey.KeySpace, true
	case "cmd+shift+m":
		return cmdShiftMModifiers(), hotkey.KeyM, true
	default:
		return altSpaceModifiers(), hotkey.KeySpace, false
	}
}

// hotkeyManager owns the global shortcut lifecycle. Apply re-registers only
// when the shortcut string changes; failures are non-fatal (log + notice).
type hotkeyManager struct {
	app    *application.App
	toggle func()
	cur    string
	unreg  func()
}

// newHotkeyManager builds a manager. toggle is the popover show/hide closure
// (same code path as a tray click, S05 SPEC §2.1).
func newHotkeyManager(app *application.App, toggle func()) *hotkeyManager {
	return &hotkeyManager{app: app, toggle: toggle}
}

// Apply ensures the given shortcut is registered. No-op when unchanged.
// On parse/register failure: log, and emit host:notice (S05 SPEC §2.2).
func (m *hotkeyManager) Apply(shortcut string) {
	if m.cur == shortcut {
		return
	}
	if m.unreg != nil {
		m.unreg()
		m.unreg = nil
		m.cur = ""
	}
	mods, key, ok := parseShortcut(shortcut)
	if !ok {
		log.Printf("hotkey: unknown shortcut %q; using alt+space", shortcut)
		notice(m.app, "unknown shortcut in config; using alt+space")
	}
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		log.Printf("hotkey: register %q failed: %v", shortcut, err)
		notice(m.app, "shortcut unavailable")
		return
	}
	m.cur = shortcut
	m.unreg = func() { _ = hk.Unregister() }
	toggle := m.toggle
	go func() {
		for range hk.Keydown() {
			if toggle != nil {
				toggle()
			}
		}
	}()
}

// Close unregisters the hotkey (called on quit, S05 SPEC §2.1).
func (m *hotkeyManager) Close() {
	if m.unreg != nil {
		m.unreg()
		m.unreg = nil
	}
}
