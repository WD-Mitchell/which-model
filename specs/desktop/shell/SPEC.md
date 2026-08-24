---
kind: feature-spec
version: "1.0"
feature: S00-shell
project: which-model-desktop
---

# S00-shell — Wails v3 Host

## 1. Purpose

`cmd/which-model-desktop` is the native shell: process lifecycle, tray icon + attached popover window, settings window, service registration/bindings, events bridge, global shortcut, login item, clipboard, launches. It is the ONLY Go package importing Wails (D00 §2.1b) and the only place platform-conditional code lives.

Inherits `specs/desktop/global/*`, `specs/desktop/backend/*` (as a consumer).

## 2. Behaviour

1. **Bootstrap order** (`main.go`): resolve `config.Paths` → `config.Load` → `service.New` (fatal dialog + exit on error, message includes remedial hint "run `which-model catalog refresh`" when the scores CSV is missing) → `application.New` with options (name "which-model", single-instance with second-launch → show popover) → create tray + windows → wire emit → `StartRefresher` → run.

2. **Events bridge.** `service.EmitFunc` is implemented as a non-blocking wrapper over `app.EmitEvent` (goroutine or buffered channel); event names/payloads pass through unmodified so the frontend's `EngineHost.on` receives exactly the D00 §3 enum.

3. **Windows.** Popover: frameless, 400×(content) hidden-by-default WebviewWindow attached to the tray; shows on tray click, hides on blur/Escape. Settings: separate 820×520 WebviewWindow, created lazily, hidden (not destroyed) on close, `MacTitleBarHiddenInset` on macOS with the web titlebar acting as the drag region. Both load from the same Vite bundle (`index.html` / `settings.html`).

4. **Window service.** The `window.*` group of `EngineHost` (D00 CONTRACTS §5) is served by a small host-side `WindowService` bound alongside the engine services: openSettings/closeSettings/hidePopover/quit/copyToClipboard (Wails clipboard API).

5. **Usage registration.** `main.go` blank-imports the three usage provider packages; builds never use `-tags nousage`.

6. **Alpha-risk register** (each has a specified fallback; implementers try primary first, document which shipped in the feature's Deviations):
   - `tray.AttachWindow` positioning → fallback: compute from tray bounds + `SetPosition`, show/hide manually.
   - Blur-to-hide flakiness → fallback: frontend outside-click/Escape handler calling `window.hidePopover`.
   - Binding generator churn → wails v3 version pinned in go.mod; generated bindings committed; regen via `task desktop:bindings` only.
   - Worst case (tray click events unusable) → tray context-menu with "Open which-model" item toggling the popover.

7. **Frontend/host contract.** In dev (`task desktop:dev`) Vite serves with HMR inside the webview; `import.meta.env.MODE === 'browser'` (plain `vite dev`) selects `MockEngineHost` so the whole UI runs in an ordinary browser for G3.

## 3. Error behaviour

- Fatal init errors show a native dialog then exit non-zero; never a blank window.
- Non-fatal host failures (shortcut registration, login-item write, clipboard) surface as toasts via an emitted `settings:changed`-adjacent mechanism defined in S05; they never crash the app.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Wails version pinning | exact `v3.0.0-alpha.X` in go.mod; bindings committed | Alpha churn must not break parallel work |
| Settings close = hide | window survives, state preserved | Mirrors macOS settings-window convention; cheap re-open |
| Browser mode for G3 | MODE-based host switch in apps, not build flags | One bundle, testable UI without native shell |
| Global hotkey library | `golang.design/x/hotkey` | Wails v3 lacks reliable global shortcuts |

## 5. Files (area-level)

| Path | Owner |
|---|---|
| `pnpm-workspace.yaml`, root `package.json`, `Taskfile.yml`, `apps/desktop/{package.json,vite.config.ts,index.html,settings.html,src/main-*.tsx}`, `.gitignore` additions, `cmd/which-model-desktop/build/**` | S01 |
| `cmd/which-model-desktop/{main.go,tray.go,popover.go}` + icon assets | S02 |
| `cmd/which-model-desktop/settingswindow.go` | S03 |
| `cmd/which-model-desktop/{services.go,windowservice.go}`, `apps/desktop/src/bindings/**`, `apps/desktop/src/host/wailsHost.ts`, host-switch in entries | S04 |
| `cmd/which-model-desktop/{hotkey.go,loginitem_darwin.go,loginitem_windows.go,loginitem_linux.go}` | S05 |
