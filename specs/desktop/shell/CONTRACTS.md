---
kind: feature-contracts
version: "1.0"
feature: S00-shell
project: which-model-desktop
---

# S00-shell — Contracts

## 1. Package and files

Package `main` at `cmd/which-model-desktop` (files per S00 SPEC §5). MAY import: Wails v3, `internal/service`, `internal/config`, `internal/usage/provider/*` (blank), `internal/usage/toggle`, `golang.design/x/hotkey`, stdlib. MUST NOT import `pkg/whichmodel` or reach around `internal/service` into other internals (exception: `config.Paths`/`config.Load` for bootstrap).

## 2. Task interface (root `Taskfile.yml`, S01 owns)

| Task | Effect |
|---|---|
| `task desktop:dev` | `pnpm --filter desktop dev` (Vite) + `wails3 dev` against it |
| `task desktop:build` | `pnpm -r build` then `wails3 build` |
| `task desktop:package` | production .app (mac) / installer-less binaries elsewhere |
| `task desktop:bindings` | `wails3 generate bindings` into `apps/desktop/src/bindings` |

## 3. Service registration (S04 owns)

All engine services + `WindowService` registered via `application.Options.Services`. Bound method set = exactly the D00 CONTRACTS §5 `EngineHost` surface; the CI check `scripts/check-host-surface.mjs` (S04) parses `host.ts` and the generated bindings and fails on any mismatch (missing or extra method).

```go
// windowservice.go
type WindowService struct{ /* refs to app + windows */ }
func (w *WindowService) OpenSettings() error
func (w *WindowService) CloseSettings() error
func (w *WindowService) HidePopover() error
func (w *WindowService) Quit() error
func (w *WindowService) CopyToClipboard(text string) error
```

## 4. Frontend host switch (S04 owns; entries import it)

```ts
// apps/desktop/src/host/index.ts
export function getHost(): EngineHost   // MODE==='browser' -> createMockEngineHost(), else wailsHost
```

## 5. Shortcut string mapping (S05 owns)

| GUISettings.Shortcut | hotkey mods+key |
|---|---|
| `alt+space` | Option/Alt + Space |
| `ctrl+space` | Ctrl + Space |
| `cmd+shift+m` | Cmd(⌘)/Win + Shift + M |

Registration failure → emit `settings:changed` with current settings + host log; UI shows toast "shortcut unavailable".

## 6. Login item (S05 owns)

macOS: LaunchAgent plist `~/Library/LaunchAgents/com.wdmitchell.which-model.plist` (Label, ProgramArguments=[binary path], RunAtLoad=true); Windows: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` value `which-model`; Linux: `~/.config/autostart/which-model.desktop`. Toggling `launch_at_login` writes/removes; failures are non-fatal toasts.
