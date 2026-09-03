---
kind: feature-contracts
version: "1.0"
feature: S03-settings-window
project: which-model-desktop
---

# S03-settings-window — Contracts

Wails identifiers are **verify at implementation** against the pinned v3 alpha (SPEC §4); S03's own signatures, values, and semantics are normative.

## 1. Package and files (lock table)

| File | Contents |
|---|---|
| `cmd/which-model-desktop/settings.go` | everything in §2; lifecycle, options, close-intercept |
| `cmd/which-model-desktop/dock_darwin.go` | macOS AppKit activation policy transition (Accessory <-> Regular) |
| `cmd/which-model-desktop/dock_darwin.m` | AppKit NSApplication setActivationPolicy bridge |
| `cmd/which-model-desktop/dock_other.go` | Non-macOS stubs for activation policy |
| `icons/which-model.icns` | colour native application artwork used by the macOS bundle |
| `scripts/package-macos.sh` | copies the icon, declares `CFBundleIconFile`, and verifies both before installation |

Not owned: `main.go`/`tray.go`/`popover.go` (S02), `windowservice.go` and bindings (S04 — S04's `WindowService.OpenSettings/CloseSettings` call §2's functions), frontend entries (S01/U07).

## 2. Go signatures

```go
// settingswindow.go (package main)

// settingsWin is the process-wide singleton; access only through the
// functions below. Guarded by settingsMu (creation may fail and be retried,
// so this is a mutex + nil check, not sync.Once — SPEC §3).
var (
    settingsMu  sync.Mutex
    settingsWin *application.WebviewWindow
)

// ensureSettingsWindow returns the settings window, creating it on first
// call with the §3 options, centring it, and installing the close-intercept
// hook (§4). Never shows the window itself. Returns an error only when
// window creation fails; callers log and may retry.
func ensureSettingsWindow(app *application.App) (*application.WebviewWindow, error)

// showSettings ensures the window exists, then Show + un-minimise + Focus.
// Idempotent; creation failure is logged and swallowed (SPEC §3).
func showSettings(app *application.App)

// hideSettings hides the window if it exists; no-op (no creation) otherwise.
func hideSettings()

// setDockIconVisible transitions between Regular (visible=true, with Dock icon)
// and Accessory (visible=false, menu-bar only) activation policy.
func setDockIconVisible(visible bool)

// dockIconVisible reports whether the activation policy is currently Regular.
func dockIconVisible() bool
```

S04 contract point: `WindowService.OpenSettings()` → `showSettings(app)`; `WindowService.CloseSettings()` → `hideSettings()`. No other package/file may touch `settingsWin`.

## 3. Settings `WebviewWindowOptions` (every field set)

| Field (verify name) | Value | Note |
|---|---|---|
| `Name` | `"settings"` | window lookup key |
| `Title` | `"which-model settings"` | hidden-inset shows no text; used by Windows/Linux frames |
| `Width` / `Height` | `820` / `560` | SPEC §2.2 (520 content + ~40 web titlebar) |
| `MinWidth` / `MinHeight` | `820` / `560` | min = max ⇒ fixed |
| `MaxWidth` / `MaxHeight` | `820` / `560` | |
| `DisableResize` | `true` | belt-and-braces with min=max |
| `Hidden` | `true` | shown only by `showSettings` |
| `Frameless` | `false` | native window with hidden-inset titlebar |
| `AlwaysOnTop` | `false` | normal window layering |
| `URL` | `"/settings.html"` | S01's second Vite entry |
| `Mac.TitleBar` | hidden-inset preset (`application.MacTitleBarHiddenInset` or the alpha's equivalent) | traffic lights inset over web content |

No other fields set; alpha defaults apply. `Center()` is called once immediately after creation.

## 4. Close-intercept hook (SPEC §2.5/§2.7)

Installed in `ensureSettingsWindow`, immediately after creation, using the alpha's window-closing event registration (candidate names, verify: `win.RegisterHook(events.Common.WindowClosing, ...)` / `win.OnWindowEvent(...)` with a cancellable event):

| Condition | Handler action |
|---|---|
| user close (traffic light, Cmd-W) while app running | cancel the close event; `win.Hide()` |
| application quitting (`quitting` flag set by the quit path, or alpha-provided distinction) | do not cancel; allow teardown |

The cancel mechanism (`e.Cancel()` vs returning a bool) follows the pinned alpha; the observable contract is: closing never destroys the window mid-session, and quit is never blocked.

## 5. Config keys / events

None. S03 reads no config and emits no events; visibility commands arrive only through §2's functions.

## 6. Verification

Automated: `go build ./...` and `go vet ./...` green (S03 compiles even before S04 wires callers — functions may be temporarily invoked from a debug tray menu item, removed by S04, or left referenced-only with `var _ = showSettings`-style keep-alives; implementer records the choice in a Deviations note if a temporary caller ships). `bash scripts/package-macos.sh` MUST fail before installation unless `Contents/Resources/which-model.icns` is non-empty and `Info.plist` declares `CFBundleIconFile = which-model.icns`.

Manual checklist (macOS primary; run with S02 in place):
1. App starts with NO settings window in the Window menu / Mission Control (lazy: nothing created yet).
2. First `showSettings` (via debug trigger or S04 popover menu): window appears centred, 820×560, titled blank on macOS with inset traffic lights over the web page; `settings.html` stub/app renders.
3. Window cannot be resized by dragging edges/corners; zoom (green) button does not fullscreen-resize the layout beyond 820×560 behaviour recorded if the alpha differs.
4. Click the red close button: window disappears; process keeps running; tray still works.
5. Re-open: same window returns (scroll/page state preserved once U07 lands), `Show`+`Focus` bring it frontmost over other apps; position from step 4 retained (not re-centred).
6. Repeat close/open 5×: no duplicate windows, no leak in Activity Monitor window count.
7. Dock icon lifecycle on macOS: Dock icon appears when Settings is shown, uses `icons/which-model.icns` (question-mark/provider-picker artwork, not the tray glyph or generic executable icon), disappears when closed/hidden, and brings Settings frontmost when clicked.
8. Quit the app while the settings window is open: app exits cleanly (close-intercept does not block quit).
9. Windows/Linux (compile-level gate, D00 §2.9): `GOOS=windows go build ./cmd/which-model-desktop` compiles; native default frame acceptable.
