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
// and the hotkey (S05 SPEC §2.1: same code path).
func trayToggle(pop *application.WebviewWindow) func() {
	return func() {
		togglePopover(pop)
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
// reconcile, and settings:changed subscription. It owns the tray so it can
// hide it when show_menu_bar_icon is false. traySetup recreates the tray
// (with popover attach + label refresh) when re-shown. Returns a handle with
// Close() for shutdown.
func buildIntegrations(
	app *application.App,
	svc *service.Services,
	pop *application.WebviewWindow,
	traySetup func() (*application.SystemTray, func()),
) *integrations {
	gui, err := svc.Settings().Get(context.Background())
	if err != nil {
		log.Printf("integrations: cannot read settings: %v", err)
	}

	in := &integrations{
		app:    app,
		hotkey: newHotkeyManager(app, trayToggle(pop)),
	}
	if gui.Shortcut != "" {
		in.hotkey.Apply(gui.Shortcut)
	}
	in.reconcileLoginItem(gui.LaunchAtLogin)

	// Tray visibility: default visible; hide when the setting is off.
	// The popover stays reachable via the hotkey + second launch (S05 SPEC
	// §2.6). traySetup may be nil when no tray was created.
	if traySetup != nil {
		tray, refresh := traySetup()
		_ = refresh
		if !gui.ShowMenuBarIcon {
			if tray != nil {
				tray.Destroy()
			}
			notice(app, "menu bar icon hidden — relaunch the app or edit config.toml to restore")
		}
	}

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