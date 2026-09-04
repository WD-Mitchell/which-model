---
kind: feature-contracts
version: "1.0"
feature: S02-tray-popover
project: which-model-desktop
---

# S02-tray-popover — Contracts

All Wails identifiers below (`application.App`, `WebviewWindow`, `SystemTray`, option/field/method names) are **verify at implementation** against the go.mod-pinned v3 alpha (see SPEC §4); the semantics, values, and Go signatures S02 owns are normative.

## 1. Package and files (lock table)

| File | Contents |
|---|---|
| `cmd/which-model-desktop/main.go` | full bootstrap (SPEC §2.1), blank imports, `fatalStartup`, events bridge (§2) — replaces the S01 stub body |
| `cmd/which-model-desktop/tray.go` | `setupTray`, `trayLabel`, label-refresh wiring |
| `cmd/which-model-desktop/popover.go` | `newPopoverWindow`, show/hide/toggle helpers, fallback-ladder code |
| `cmd/which-model-desktop/assets/tray-icon.svg` | editable glyph source (§4) |
| `cmd/which-model-desktop/assets/tray-iconTemplate.png` | 18×18 raster of the SVG, black on transparent |
| `cmd/which-model-desktop/assets/tray-iconTemplate@2x.png` | 36×36 raster, same artwork |
| `cmd/which-model-desktop/assets/assets.go` | `package main`-adjacent `go:embed` declarations (package `main`, build-tag-free) |

Not owned: `settingswindow.go` (S03), `services.go`/`windowservice.go`/bindings (S04), anything under `internal/` or `apps/`.

## 2. Go signatures

```go
// main.go
func main()

// fatalStartup shows a native error dialog (SPEC §3) and exits 1.
// Uses the Wails dialog API when app != nil, else the platform fallback.
func fatalStartup(app *application.App, title, message string)

// emitBridge is the S00 §2.2 events bridge. Emit never blocks (SPEC §2.3).
type emitBridge struct {
    ch  chan emitMsg     // cap 64
    tap func(name string) // host-side listener (tray label refresh); called on the drain goroutine
}
type emitMsg struct {
    Name string
    Data any
}
func newEmitBridge(tap func(name string)) *emitBridge
func (b *emitBridge) SetApp(app *application.App) // starts the drain goroutine; queued events flush
func (b *emitBridge) Emit(name string, data any)  // passed to service.New as the EmitFunc
func (b *emitBridge) Close()                      // stops the goroutine; idempotent

// tray.go
// setupTray creates the tray, sets the template icon and initial label,
// wires OnClick to the popover, and returns the tray. refresh is invoked by
// the bridge tap for pick:recorded | config:changed | catalog:changed.
func setupTray(app *application.App, svc *service.Services, pop *application.WebviewWindow) (tray *application.SystemTray, refresh func())

// trayLabel ranks {ProfileSlug: "balanced_implementation", Holds: 1} and
// formats SPEC §2.4b. Never returns an error: failures log and yield "—".
func trayLabel(ctx context.Context, svc *service.Services) string

// popover.go
func newPopoverWindow(app *application.App) *application.WebviewWindow
func showPopover(w *application.WebviewWindow)   // Show + Focus
func hidePopover(w *application.WebviewWindow)   // Hide, never Close
func togglePopover(w *application.WebviewWindow) // used by fallback ladder steps 2/4
```

## 3. Popover `WebviewWindowOptions` (every field set)

| Field (verify name) | Value | Note |
|---|---|---|
| `Name` | `"popover"` | window lookup key |
| `Title` | `"which-model"` | not rendered (frameless) |
| `Width` / `Height` | `400` / `620` | SPEC §2.5; frontend scrolls |
| `Frameless` | `true` | mockup popover has no chrome |
| `Hidden` | `true` | shown only via tray |
| `AlwaysOnTop` | `true` | floats over full-screen-adjacent apps |
| `DisableResize` | `true` | fixed geometry |
| `URL` | `"/"` | popover entry (`index.html`) |
| `BackgroundType` / colour | transparent if the alpha supports it, else `--color-bg` `#0d0f12`-equivalent solid | the mockup's 12px radius + arrow are drawn by the frontend; transparency lets corners show through |
| `Mac.TitleBar` | fully hidden variant (frameless — set only if required by the alpha) | verify |

No other option fields are set; alpha defaults apply and any forced deviation is recorded in SPEC Deviations.

## 4. Tray icon asset

`tray-icon.svg` (source, authored verbatim from the mockup menu-bar glyph; stroke colour becomes `#000` in the template rasters — the mockup's `var(--color-accent-200)` is display-only):

```svg
<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#000"
     stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
  <path d="M2.2 4.6h11.6"/><path d="M2.2 8h7.4"/><path d="M2.2 11.4h4.2"/>
  <circle cx="12.4" cy="11.2" r="2.1"/>
</svg>
```

Rasters: black strokes, transparent background, no colour (macOS template-image rules); filenames end `Template`/`Template@2x` so AppKit auto-detects template mode. Embedded via `go:embed assets/tray-iconTemplate*.png`.

## 5. Fallback-ladder decision table (SPEC §2.6)

| Step | Condition to fall through | Mechanism |
|---|---|---|
| 1 `AttachWindow` | window appears detached/mispositioned or click does not toggle | `tray.AttachWindow(pop)` only |
| 2 manual position | blur-hide works but positioning wrong | `tray.OnClick` → `tray.Bounds()` → `SetPosition(centreX-200, menubarBottom)` → `togglePopover` |
| 3 frontend hide-assist | window stays open on focus loss | lost-focus host hook + frontend Escape/outside-click → hide |
| 4 menu fallback | tray click events unusable | tray menu item `Open which-model` → `togglePopover` |

Shipped step MUST be recorded in a `## Deviations` section of this feature's SPEC.

## 6. Config keys / events consumed

- Reads only via `service.Services` (no direct TOML access). Emits nothing itself; forwards the D00 §3 enum verbatim through the bridge.
- Tap-refresh trigger set (closed): `pick:recorded`, `config:changed`, `catalog:changed`.

## 7. Verification

Automated: `go build ./...` and `go vet ./...` green; `pnpm -r build` unaffected. No Go unit tests (host wiring is manual-verify per S00; `trayLabel` formatting MAY be unit-tested if factored free of Wails types).

Manual checklist (macOS primary; run before G-gate sign-off):
1. `task desktop:build` then launch: menu-bar icon appears, template-tinted in both light and dark menu bars.
2. Label shows an integer score next to the icon (fixture config), or `—` with an empty/missing catalog config.
3. Delete `<CacheDir>/catalog/available_model_scores.csv`, relaunch: native dialog with the exact SPEC §3 wording (path + `which-model catalog refresh` hint); process exits 1.
4. Tray click opens the popover under the icon (400×620, frameless, stub/popover entry rendered); second click hides it.
5. Click another app: popover hides (or fallback step 3 documented in Deviations).
6. Launch the app a second time while running: no second instance; popover shows.
7. Record a pick / edit config via CLI while running: label updates without restart (events bridge live).
8. Quit and relaunch repeatedly: no zombie processes, `launch.log`/stderr free of dropped-event spam at idle.

## Review correction — #175: completed empty frontend ranking

After a successful empty ranking, the popover calls `SetTrayPick("", "", "", "")`. This clears both recommendation lines and the provider icon while retaining frontend ownership. Host refresh must not substitute the default profile's recommendation. Initial pending queries and temporary query-key transitions publish nothing; a later successful nonempty rank restores the matching text and provider. Pinned regressions cover initial pending, valid → empty → valid, and frontend ownership after an empty push.
