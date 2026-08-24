// S05 — OS integrations: global hotkey, launch-at-login reconcile, and tray
// visibility, driven by startup settings and settings:changed events. All
// failures are non-fatal (log + host:notice; S05 SPEC §3).
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// serviceSettingsChanged is the D00 §3 event the service emits after a
// settings write (internal/service/events.go).
const serviceSettingsChanged = "settings:changed"

// trayToggle is the shared popover show/hide used by both the tray click path
// and the hotkey (S05 SPEC §2.1: same code path). tray may be nil (no tray, or
// the menu bar icon turned off), in which case the popover keeps its last
// position instead of being placed under the icon.
func trayToggle(tray *application.SystemTray, pop *application.WebviewWindow) func() {
	return func() {
		togglePopoverAt(tray, pop)
	}
}

// resolvedExecPath returns os.Executable() resolved through symlinks, used for
// login-item artifacts (S05 SPEC §2.4). Never returns "".
func resolvedExecPath() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return exe
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		return resolved
	}
	return exe
}

// integrations bundles the S05 runtime pieces so main can hold one handle.
type integrations struct {
	app    *application.App
	hotkey *hotkeyManager
	off    func()
}

// settingsChanged is the single reconcile entry (startup + events).
func (in *integrations) settingsChanged(_ *application.App, svc *service.Services, gui service.GUISettings) {
	_ = svc
	if in.hotkey != nil {
		in.hotkey.Apply(gui.Shortcut)
	}
	in.reconcileLoginItem(gui.LaunchAtLogin)
}

// buildIntegrations reads startup settings and wires the hotkey, login-item
// reconcile, and settings:changed subscription. It owns the tray's existence:
// traySetup is called (creating the tray with its popover attach, label and
// right-click menu) only when show_menu_bar_icon is on. Returns a handle with
// Close() for shutdown.
func buildIntegrations(
	app *application.App,
	svc *service.Services,
	pop *application.WebviewWindow,
	traySetup func() (*application.SystemTray, func()),
) *integrations {
	// showTray defaults to true and only follows the setting when the settings
	// read succeeded: a read failure must not silently take the menu-bar icon
	// away, which is the app's only visible surface.
	showTray := true
	gui, err := svc.Settings().Get(context.Background())
	if err != nil {
		log.Printf("integrations: cannot read settings: %v", err)
	} else {
		showTray = gui.ShowMenuBarIcon
	}

	// Tray visibility (S05 SPEC §2.6): the tray is simply not created when the
	// icon is off — SystemTray.Destroy() is a no-op before the app runs
	// (systemtray.go: `if s.impl == nil { return }`) and the tray is still in
	// App.pendingRun at that point, so destroying it here would leave the icon
	// on screen and hand the tray menu a released NSStatusItem. The popover
	// stays reachable via the hotkey and a second launch.
	//
	// The tray is built before the hotkey manager so the hotkey can share its
	// show path and land the popover under the menu-bar icon however it was
	// opened (S05 SPEC §2.1). traySetup may be nil when no tray is wanted.
	var tray *application.SystemTray
	if traySetup != nil {
		if showTray {
			tray, _ = traySetup()
		} else {
			notice(app, "menu bar icon hidden — relaunch the app or edit config.toml to restore")
		}
	}

	in := &integrations{
		app:    app,
		hotkey: newHotkeyManager(app, trayToggle(tray, pop)),
	}
	if gui.Shortcut != "" {
		in.hotkey.Apply(gui.Shortcut)
	}
	in.reconcileLoginItem(gui.LaunchAtLogin)

	// Subscribe to settings:changed for hotkey/login-item re-apply.
	off := app.Event.On(serviceSettingsChanged, func(ev *application.CustomEvent) {
		var guiSettings service.GUISettings
		switch d := ev.Data.(type) {
		case service.GUISettings:
			guiSettings = d
		default:
			// Event payload decoded by the runtime as a plain map.
			guiSettings = decodeGUISettings(d)
		}
		in.settingsChanged(nil, svc, guiSettings)
	})
	in.off = off

	return in
}

// Close removes the settings listener and unregisters the hotkey.
func (in *integrations) Close() {
	if in.off != nil {
		in.off()
		in.off = nil
	}
	if in.hotkey != nil {
		in.hotkey.Close()
	}
}

// decodeGUISettings best-effort decodes an arbitrary (map[string]any) event
// payload back into GUISettings. Unknown/corrupt fields keep zero values, so
// a followed reconcile skips no-op hotkey re-register (empty shortcut) and
// leaves login-item state untouched for that field.
func decodeGUISettings(d any) service.GUISettings {
	m, ok := d.(map[string]any)
	if !ok {
		return service.GUISettings{}
	}
	g := service.GUISettings{}
	if s, ok := m["shortcut"].(string); ok {
		g.Shortcut = s
	}
	if b, ok := m["launch_at_login"].(bool); ok {
		g.LaunchAtLogin = b
	}
	if b, ok := m["show_menu_bar_icon"].(bool); ok {
		g.ShowMenuBarIcon = b
	}
	return g
}

// reconcileLoginItem sets the login item to match `enabled`; failures are
// logged + host:notice (S05 SPEC §2.4, §3).
func (in *integrations) reconcileLoginItem(enabled bool) {
	path := resolvedExecPath()
	if err := setLoginItem(enabled, path); err != nil {
		log.Printf("integrations: login item reconcile failed: %v", err)
		notice(in.app, "could not update launch at login")
	}
}