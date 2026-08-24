---
kind: feature-spec
version: "1.0"
feature: S05-integrations
project: which-model-desktop
---

# S05-integrations — Hotkey, Login Item, Clipboard, Tray Visibility

## 1. Purpose

S05 delivers the OS integrations of the shell: the global shortcut (`golang.design/x/hotkey`) that toggles the popover, launch-at-login per platform (`loginitem_{darwin,windows,linux}.go`), the clipboard flow for copy-mode launches, `show_menu_bar_icon=false` handling, and the second-launch behaviour test. All failures here are non-fatal (S00 SPEC §3): they log and surface as toasts via the shell-local `host:notice` event defined in §2.3.

Depends on: S04, B07 (settings), B10 (launch).

## 2. Behaviour

1. **Hotkey lifecycle** (`hotkey.go`). On app start, read `GUISettings.Shortcut` and register the mapped hotkey (S00 CONTRACTS §5 table; parser in CONTRACTS §3). Registration is independent of `ShowMenuBarIcon` — the shortcut works even with the tray hidden. Subscribe to `settings:changed`: when the payload's `shortcut` differs from the currently registered one, unregister then register the new mapping; when equal, do nothing. Unregister on quit. The hotkey firing toggles the popover (show if hidden, hide if visible) — same code path as a tray click (S02).

2. **Hotkey failure.** If registration fails (hotkey taken, platform denial): log the error, leave the app running, emit `settings:changed` carrying the current `GUISettings` (so UI state reconciles, per S00 CONTRACTS §5), and emit `host:notice` with message `shortcut unavailable` so the UI toasts it. No retry loop; the next `settings:changed` with a different shortcut retries naturally.

3. **`host:notice` (Deviation from D00 §3 — see CONTRACTS §2).** D00's event enum is service-emitted and closed; S05 adds ONE shell-local event, `host:notice`, payload `{"message": string}`, emitted only by the shell process (never by `internal/service`, never added to `internal/service/events.go`). `wailsHost.on()` forwards it like any other event (S04 CONTRACTS §4). It is the transient-toast mechanism for non-fatal host failures (S00 SPEC §3). Follow-up wiring point for U05: the popover shell registers `host.on('host:notice', ...)` → `useToast` with the payload message.

4. **Login item** (`loginitem_*.go`, per S00 CONTRACTS §6). One file per GOOS, each exporting `setLoginItem(enabled bool, execPath string) error` behind the shared surface in CONTRACTS §4. `execPath` is `os.Executable()` resolved through symlinks at call time. On start AND on every `settings:changed`, reconcile: `LaunchAtLogin` true → write the artifact (CONTRACTS §5 templates verbatim, idempotent overwrite); false → remove it (missing artifact is success). Failure → log + `host:notice` `could not update launch at login`.

5. **Clipboard for copy-mode launches.** When `copy_command_instead` is on, `harnesses.launch` returns `LaunchResult{Copied: true, Command: ...}` (B10); the FRONTEND then calls `window.copyToClipboard(Command)` — already part of D00 §5 served by S04's `WindowService`. S05 adds no new surface; it verifies the host clipboard implementation end-to-end (see CONTRACTS §6 checklist).

6. **Tray hidden.** When `GUISettings.ShowMenuBarIcon` is false at start, the tray icon is not created (or is destroyed on the settings flip); popover remains reachable via the global shortcut and second-launch. On the transition to hidden, emit `host:notice` with exactly: `menu bar icon hidden — relaunch the app or edit config.toml to restore`.

7. **Single-instance second launch.** Already implemented in S02 (launching the binary while running shows the popover); S05 owns the manual verification of it alongside the other integrations (CONTRACTS §6).

## 3. Error behaviour

- Every S05 failure is non-fatal: log via the app logger + `host:notice` toast; never a dialog, never an exit.
- `parseShortcut` on an unknown string (config hand-edited) → falls back to `alt+space` and emits `host:notice` `unknown shortcut in config; using alt+space`. It never rejects at this layer (B07 validates writes).
- Login-item removal when the artifact is absent is success, not an error.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Toast transport | new shell-local `host:notice` event, NOT a service enum addition | Keeps D00 §3 closed for the engine; the shell is the only emitter and wailsHost already forwards events (S00 SPEC §3 asked S05 to define this) |
| Hotkey re-register trigger | compare `shortcut` field on `settings:changed` | Payload is full `GUISettings` (D00 §3); diffing avoids churn on unrelated setting flips |
| Login-item strategy | file/registry artifacts, reconciled (not toggled blindly) on start + settings change | Idempotent; heals hand-deleted artifacts and stale executable paths after app moves |
| Executable path | `os.Executable()` + `filepath.EvalSymlinks`, resolved per write | Templates must point at the real binary, incl. inside the .app bundle |
| Hotkey failure policy | non-fatal, no retry loop | S00 CONTRACTS §5; a taken hotkey is an environment fact, not an app error |

## 5. Out of scope

- Shortcut VALIDATION on write (B07 — settings service rejects values outside the 3-string enum). Toast component and the `host:notice` listener wiring (U02/U05 follow-up). Tray creation/click handling (S02). `LaunchResult` semantics (B10). Auto-update, MCP server, shell-alias settings (other features).
