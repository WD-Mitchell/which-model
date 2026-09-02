---
kind: feature-spec
version: "1.0"
feature: S03-settings-window
project: which-model-desktop
---

# S03-settings-window — Lazy Settings WebviewWindow

## 1. Purpose

The 820-wide Settings window hosting the `settings.html` entry (8 pages + detail views, U07+). S03 owns only the native window: lazy singleton creation, fixed geometry, hidden-inset macOS titlebar, and the close-means-hide lifecycle. The `EngineHost.window.openSettings/closeSettings` bindings that call into it land in S04's `WindowService`; S03 exposes the plain Go functions S04 wires up.

Depends on: S01 (scaffold, entries). Inherits D00, S00 (§2.3 windows, §2.6 alpha risks).

## 2. Behaviour

1. **Lazy singleton.** No settings window exists at startup. The first call to `ensureSettingsWindow(app)` creates it (options in CONTRACTS §3) and returns it; every later call returns the same instance. Creation is guarded by a `sync.Once` — safe against concurrent open requests from bindings and menu items.

2. **Geometry.** 820 wide. Total window height **560**: the mockup's 820×520 is content height, and the hidden-inset titlebar region (~40px, drawn by the web page as its drag-region titlebar per S00 §2.3) sits above it. Fixed size: min size = max size = 820×560, resize disabled. Centred on the screen at first creation (`Center()`); subsequent shows keep the user's last position.

3. **Chrome.** Not frameless. macOS uses the hidden-inset titlebar variant (`MacTitleBarHiddenInset`-equivalent in the pinned alpha — traffic lights inset over web content, no native title text); the web titlebar carries `--wails-draggable` styling (U07 owns). Windows/Linux keep the default native frame (D00 §2.9 polish scope). Title `which-model settings`.

4. **URL.** `"/settings.html"` — the second Vite entry from S01. Dev mode serves it from the Vite server via the wails config; production from the bundled dist.

5. **Close = hide.** The window is created once and never destroyed while the app runs. The native close action (traffic-light close, Cmd-W, system menu) is intercepted via the alpha's window close/closing event hook: the handler cancels the close and calls `Hide()`. Webview state (mounted React tree, scroll positions, unsaved field focus) therefore survives across open/close cycles (S00 §4 decision "Settings close = hide").

6. **Open/reopen.** `showSettings()` = `setDockIconVisible(true)` → `ensureSettingsWindow` → `Show()` → `Focus()` (un-minimise first if the alpha reports a minimised state). `hideSettings()` = `setDockIconVisible(false)` → hides if the window exists, and is a no-op (no creation) otherwise. Both are idempotent.

7. **Quit path.** App quit tears the window down through normal application shutdown; the close-intercept hook must not block `app.Quit()` (the hook cancels only user-initiated window closes — verify the alpha distinguishes these; if it cannot, the quit path sets a `quitting` flag the hook checks before cancelling).

8. **Activation policy & Dock / taskbar presence.** which-model runs as a menu-bar app (`ActivationPolicyAccessory` on macOS) by default. While the Settings window is visible:
   - macOS: the app dynamically transitions to `NSApplicationActivationPolicyRegular`, causing the which-model Dock icon to appear, enabling Cmd-Tab window cycling to reach Settings, and focusing Settings when the Dock icon is clicked.
   - Windows/Linux: Settings has a standard taskbar button while visible.
   When Settings is hidden (via native close, Escape, or `closeSettings`), macOS transitions back to `NSApplicationActivationPolicyAccessory` (removing the Dock icon), leaving the menu-bar tray as the always-on interface.
## 3. Error behaviour

- Window creation failure (webview init error) is non-fatal: log host-side; `showSettings()` retries creation on the next call (the `sync.Once` is only marked done on success — implement as a mutex + nil check rather than a literal `sync.Once` if the alpha can fail without panic).
- `hideSettings()`/`showSettings()` never panic when called before/after window existence; all nil-checks live in S03, not in callers.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Total height | 560 (= 520 content + ~40 web titlebar) | D00 token "820×520 content + titlebar"; fixing the native height keeps the mockup's layout pixel-stable |
| Fixed size | min = max = 820×560, resize off | Mockup pages are designed to one geometry; no responsive settings layout in v1 |
| Lifecycle | create-once, hide-on-close, Show+Focus on reopen | macOS settings-window convention; cheap re-open; preserves in-page state |
| Creation timing | lazy, on first open | Most sessions never open Settings; saves a webview at startup |
| Titlebar | hidden-inset on macOS, native default elsewhere | Traffic lights over the web sidebar per mockup; non-mac polish out of scope (D00 §2.9) |
| Wails API verification | NOT verified — module absent from the local cache and not fetchable offline (`go doc` failed). `Window.NewWithOptions` / `WebviewWindowOptions` / `Mac.TitleBar` / close-event hook names follow the published v3 alpha surface; **verify at implementation** | Same constraint recorded in S02 SPEC §4 |
