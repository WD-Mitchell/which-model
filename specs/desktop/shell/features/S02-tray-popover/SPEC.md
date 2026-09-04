---
kind: feature-spec
version: "1.0"
feature: S02-tray-popover
project: which-model-desktop
---

# S02-tray-popover — Tray Icon, Score Label, Popover Window

## 1. Purpose

Replace the S01 `main.go` stub with the full host bootstrap (S00 SPEC §2.1), the system-tray icon with a live top-pick score label, the events bridge (S00 §2.2), and the frameless popover window attached to the tray. After S02 the app lives in the menu bar: clicking the tray toggles a 400×620 popover loading the Vite popover entry.

Depends on: S01 (scaffold, wails config), B02 (`service.New`, `StartRefresher`, PickService). Inherits D00, S00.

## 2. Behaviour

1. **Bootstrap order** (`main.go`, replacing the S01 stub body, in exactly this order):
   1. `config.ResolvePaths()` → on error, fatal dialog (§3) and exit 1.
   2. `config.Load(paths)` → on error, fatal dialog with the underlying message.
   3. Construct the events bridge (clause 3) with a nil app reference (events queue until the app exists), then `service.New(paths, cfg, bridge.Emit)` → on error, fatal dialog. When the error is the missing-scores-CSV `io_error` (message contains the CSV path), the dialog text is the remedial form in §3.
   4. `application.New(application.Options{Name: "which-model", ...})` with single-instance enabled; the second-launch callback shows the popover (clause 6). Exact option/field names for single instance: verify at implementation (v3 alpha `SingleInstance` options with `OnSecondInstanceLaunch` callback).
   5. Create tray (clause 4) and popover window (clause 5); hand the app to the bridge.
   6. `go svc.StartRefresher(ctx, 5*time.Minute)` with a context cancelled on app shutdown.
   7. `app.Run()`; non-nil error → fatal dialog, exit 1.
   `main.go` blank-imports `internal/usage/provider/{claude,codex,copilot}`; the desktop binary never builds with `-tags nousage` (D00 §2.10).

2. **Service registration is NOT S02.** `application.Options.Services` stays empty here; S04 adds bindings. S02 calls `service` methods directly from Go only (tray label, refresher).

3. **Events bridge** (S00 §2.2). `bridge.Emit(name string, data any)` never blocks: it sends onto a buffered channel of capacity **64**; when the channel is full the event is dropped and one line is logged (`log.Printf("events: dropped %s (queue full)", name)`). A single goroutine drains the channel and calls `app.EmitEvent(name, data)` (verify exact method name at implementation), after first invoking the host-side tap (clause 4c). Events sent before the app exists queue in the channel. Names/payloads pass through unmodified (D00 CONTRACTS §3 enum).

4. **Tray.**
   a. `app.SystemTray.New()` (verify), with a **template icon** so macOS tints it for menu-bar appearance: `tray.SetTemplateIcon(pngBytes)` using the embedded asset of CONTRACTS §4 (glyph: three shortening horizontal lines + magnifier circle, from the mockup menu-bar SVG). Windows/Linux use the same PNG via `SetIcon` (monochrome acceptable at alpha polish level, D00 §2.9).
   b. `tray.SetLabel(text)` shows the current top-pick score next to the icon, mirroring the mockup's `pickScoreShort`: the top candidate's score rounded to the nearest integer (`math.Round`), rendered as a plain integer string; `"—"` (em dash) when ranking fails or returns no route.
   c. The label recomputes at startup and whenever the bridge tap sees `pick:recorded`, `config:changed`, or `catalog:changed`. Recomputation calls `svc.Pick.Rank(ctx, RankRequest{ProfileSlug: "balanced_implementation", Holds: 1})` — the default landing profile, centre stop of B03's complexity scale. Rank errors are logged, label becomes `"—"`, never a crash.
   d. `tray.OnClick` toggles the popover (primary path uses AttachWindow's built-in toggle; see clause 6).

5. **Popover window.** One `app.Window.NewWithOptions` (verify name; v3 alpha also exposes `app.NewWebviewWindowWithOptions`) WebviewWindow, created hidden at startup with exactly the options in CONTRACTS §3: frameless, hidden, always-on-top, 400×620, URL `/` (the `index.html` popover entry). Height 620 is fixed by this spec: the mockup's landing/weights content measures ≈560–640 depending on holds; the frontend body scrolls inside the window (U05 owns scrolling), the window never resizes.

6. **Attach + fallback ladder** (S00 §2.6, restated as the normative order; the implementer tries each in order and records which shipped in a Deviations section here):
   1. `tray.AttachWindow(popover)` — Wails positions and toggles the window under the tray icon on click.
   2. If attach positioning misbehaves: compute position from `tray.Bounds()` (verify), `SetPosition` under the icon, and toggle show/hide in `tray.OnClick` manually.
   3. If blur-to-hide is flaky: the frontend's Escape/outside-click handler calls `window.hidePopover` (S04 WindowService; until S04 lands, the host also hides on the window's lost-focus event — verify event name `events.Common.WindowLostFocus`).
   4. Worst case (tray click events unusable): tray context menu with an "Open which-model" item that toggles the popover.

7. **Hide behaviour.** The popover hides (never closes/destroys) on: focus loss, Escape (frontend assist per clause 6.3), and successful launch when `close_popover_after_launch` (frontend-driven, S04+). Hiding preserves webview state.

## 3. Error behaviour

- Fatal init errors show a native modal dialog then `os.Exit(1)` — never a blank window (S00 §3). Dialog API: v3 alpha message dialogs (verify exact constructor, e.g. `application.ErrorDialog()` builder); if no app instance exists yet, fall back to `osascript -e 'display alert ...'` on darwin and stderr elsewhere.
- Missing scores CSV dialog wording (exact):
  Title: `which-model can't start`
  Message: `The model catalog is missing.\n\nExpected file:\n<path>\n\nRun "which-model catalog refresh" in a terminal, then reopen which-model.`
  where `<path>` is the path carried in the service error. All other init errors: title `which-model can't start`, message = wrapped error text.
- Tray-label rank failures and dropped events are log-only; the shell never crashes on service errors after startup.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Popover height | 620 fixed (400 wide per D00 tokens) | Mockup content spans ≈560–640 across views; fixed window + frontend scroll beats resize churn |
| Label profile | `balanced_implementation`, Holds 1 | Centre of the complexity scale = the popover's default landing state; label matches what the user sees on open |
| Label format | `strconv.Itoa(int(math.Round(score)))` or `—` | Mockup `pickScoreShort = Math.round(pick.score)`; menu bar has no room for 2dp |
| Event buffer | channel cap 64, drop-with-log on overflow | Emit must never block a service write lock; 64 ≫ realistic burst; dropped UI refreshes are self-healing on next event |
| Tray icon form | Template PNG (@1x/@2x) embedded via `go:embed`, SVG source kept in-repo | macOS wants template rasters for auto light/dark tinting; SVG is the editable source of truth |
| Single-instance action | second launch → show popover | Menu-bar apps must not spawn twice; showing the popover is the least surprising response |
| Wails API verification | NOT verified against the module — `go doc` unavailable offline (module absent from cache, proxy fetch failed with checksum error). All `application.*` names in this pair follow the published v3 alpha surface and are **verify at implementation** | Recorded per process rules; the pinned go.mod version is authoritative |


## Shutdown lifecycle correction — #51

Application shutdown first marks the application as quitting, then cancels the
tracked tray startup fallback and popover focus-reclaim timers, and closes the
event bridge. The normal ApplicationStarted callback cancels its fallback timer;
hiding the popover cancels its pending focus timer. Focus callbacks check the
show generation and quitting state before touching the window. Timer cancellation
is idempotent. Closing the bridge is safe concurrently through sync.Once and
signals its drain goroutine to exit; main also defers Close for early returns.
Already-running callbacks retain their existing native-runtime lifetime constraints.

Pinned regressions in cmd/which-model-desktop/shutdown_test.go verify repeated
timer cancellation, hide cancellation, and concurrent bridge closure.
Run: `go test -race ./cmd/which-model-desktop`.
