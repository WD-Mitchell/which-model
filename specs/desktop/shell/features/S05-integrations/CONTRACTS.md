---
kind: feature-contracts
version: "1.0"
feature: S05-integrations
project: which-model-desktop
---

# S05-integrations — Contracts

## 1. Files

| File | Contents |
|---|---|
| `cmd/which-model-desktop/hotkey.go` | §3: `parseShortcut`, `hotkeyManager` lifecycle |
| `cmd/which-model-desktop/loginitem_darwin.go` | §4 surface + §5.1 plist template (`//go:build darwin`) |
| `cmd/which-model-desktop/loginitem_windows.go` | §4 surface + §5.2 registry value (`//go:build windows`) |
| `cmd/which-model-desktop/loginitem_linux.go` | §4 surface + §5.3 desktop file (`//go:build linux`) |

## 2. Deviation from D00 §3 — `host:notice`

| Event | Emitted by | Payload |
|---|---|---|
| `host:notice` | shell (`cmd/which-model-desktop`) only | `{"message": string}` |

Shell-local addition to the event stream; NOT added to the D00 §3 enum, `internal/service/events.go`, or `packages/core/src/events.ts`'s service union (it may be typed there as a separate `HostEvent`). `wailsHost.on()` forwards it verbatim (S04 CONTRACTS §4). Emitter helper: `func notice(app *application.App, message string)`. U05 follow-up wiring: toast listener on `host:notice`. Exact message copies used by S05:

| Trigger | `message` |
|---|---|
| hotkey registration failure | `shortcut unavailable` |
| unknown shortcut string in config | `unknown shortcut in config; using alt+space` |
| login-item write/remove failure | `could not update launch at login` |
| tray hidden | `menu bar icon hidden — relaunch the app or edit config.toml to restore` |

## 3. Hotkey (`hotkey.go`)

```go
// parseShortcut maps GUISettings.Shortcut → hotkey registration inputs.
// Unknown input returns the alt+space mapping and ok=false (caller emits notice).
func parseShortcut(s string) (mods []hotkey.Modifier, key hotkey.Key, ok bool)

type hotkeyManager struct{ /* app, popover toggle fn, current string, *hotkey.Hotkey */ }
func newHotkeyManager(app *application.App, toggle func()) *hotkeyManager
func (m *hotkeyManager) Apply(shortcut string)   // no-op if same string; else unregister → parse → register; failure → log + settings:changed(current GUISettings) + notice
func (m *hotkeyManager) Close()                   // unregister; called on quit
```

Mapping table (fixed by S00 CONTRACTS §5):

| `GUISettings.Shortcut` | Modifiers | Key |
|---|---|---|
| `alt+space` | `hotkey.ModOption` (darwin) / `hotkey.ModAlt` | `hotkey.KeySpace` |
| `ctrl+space` | `hotkey.ModCtrl` | `hotkey.KeySpace` |
| `cmd+shift+m` | `hotkey.ModCmd` (darwin) / `hotkey.ModWin` (windows) / `ModCtrl+ModShift` fallback semantics do NOT apply — linux uses Super via `hotkey.Mod4` | `hotkey.KeyM` (+ `hotkey.ModShift`) |

`Apply` is called once at start with the loaded settings and from the `settings:changed` subscription with the payload's `shortcut`. Hotkey fire → `toggle()` (popover show/hide, same path as tray click).

## 4. Login item surface (shared; one impl per GOOS)

```go
// setLoginItem reconciles the platform artifact to `enabled`.
// execPath: os.Executable() resolved via filepath.EvalSymlinks.
// Removal when absent returns nil.
func setLoginItem(enabled bool, execPath string) error

func loginItemPath() (string, error) // darwin/linux artifact path; windows returns the registry value name "which-model"
```

Reconciled at start and on every `settings:changed` (compare `launch_at_login` against artifact existence; rewrite if exec path stale). Errors → log + `host:notice` per §2.

## 5. Artifact templates (verbatim; `%s` = execPath)

### 5.1 macOS — `~/Library/LaunchAgents/com.wdmitchell.which-model.plist`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.wdmitchell.which-model</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
```

### 5.2 Windows — registry

Key `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, string value name `which-model`, data `"%s"` (quoted exec path). Enable = SetStringValue; disable = DeleteValue (absent → nil).

### 5.3 Linux — `~/.config/autostart/which-model.desktop`

```ini
[Desktop Entry]
Type=Application
Name=which-model
Exec=%s
X-GNOME-Autostart-enabled=true
```

## 6. Verification

**Unit (Go, in-package tests):**
- `parseShortcut` table test: the 3 valid strings → expected mods/key per §3 table (per-GOOS expectations); unknown string → alt+space mapping + `ok=false`.
- Plist rendering golden test: render §5.1 with a fixture path, byte-compare against `testdata/loginitem.plist.golden`. Same pattern for the `.desktop` file.
- Reconcile logic test with a temp dir standing in for the artifact location: enable→file exists with exec path; enable again→idempotent; disable→gone; disable again→nil.

**Manual checklist (integration gate):**
1. Default shortcut toggles popover from any app; works with tray icon hidden.
2. Change shortcut in settings → old chord dead, new chord live, no restart.
3. Occupy the chord with another app first → app still runs, toast `shortcut unavailable`.
4. Toggle launch-at-login on → artifact exists with correct binary path (per-OS §5); toggle off → removed; hand-delete artifact then flip settings → recreated.
5. Enable `copy_command_instead` → Launch puts the fully substituted command on the clipboard (paste to verify), no process spawned, toast shows the command.
6. Set `show_menu_bar_icon=false` → tray icon disappears, toast with the exact §2 copy, shortcut still opens popover; restore via config.toml edit + relaunch.
7. With the app running, launch the binary again → existing instance shows the popover; no second tray icon (S02 behaviour, verified here).
