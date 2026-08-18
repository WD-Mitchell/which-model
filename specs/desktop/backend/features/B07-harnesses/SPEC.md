---
kind: feature-spec
version: "1.0"
feature: B07-harnesses
project: which-model-desktop
---

# B07-harnesses — Harness Registry, Command Templates, Launch

## 1. Purpose

`HarnessService` (in `internal/service/harness.go`) owns the harness concept the CLI never had: named launch targets ("Claude Code", "Codex CLI", …) with a command template, a per-harness provider allow-map, PATH-based install detection, `{model_id}`/`{reasoning}` substitution, and the Launch action behind the popover's "Launch in {harness}" split button. Config lives under `[harnesses.<slug>]` (schema owned by B01); DTOs are D00's `HarnessInfo` and `LaunchResult`.

Depends on: B02 (Services core, sentinels, test helper). Consumes B04's `RecordPick` through an internal seam (§2.10).

## 2. Behaviour

1. **Access.** `Services.Harnesses() *HarnessService` returns the sub-service (back-pointer to `Services`; shares its mutex, config, and emit). Methods map 1:1 to the D00 `EngineHost.harnesses` group plus the internal `BuildCommand`.

2. **Builtin seeds.** On the FIRST `List` call that finds `[harnesses]` empty (no subtables at all), the service writes the four builtin seeds to config atomically (one write, one `config:changed{section:"harnesses"}` event) before answering. The seeds, verbatim (slug / name / command template):
   - `claude` / `Claude Code` / `claude --model {model_id} --reasoning {reasoning}`
   - `codex` / `Codex CLI` / `codex -m {model_id} -c reasoning={reasoning}`
   - `copilot` / `Copilot CLI` / `copilot --model {model_id}`
   - `cursor` / `Cursor` / `cursor --model {model_id}`
   Each seed is written with `builtin = true` and `providers` = the seed default in CONTRACTS §4. Seeding happens at most once: any non-empty `[harnesses]` section (even a single custom) suppresses it forever.

3. **List.** Returns every configured harness in slug-ascending order as `HarnessInfo`. `Providers` is a map over ALL provider ids currently configured under `[providers.*]` (B06's set), `true` iff the id appears in the harness's `providers` list; ids in the list but absent from `[providers.*]` are preserved in config but omitted from the DTO map. `Installed` is computed per §2.4 on every List, never persisted.

4. **Install detection.** `Installed = (exec.LookPath(argv0) == nil)` where `argv0` is the first whitespace-separated token of `Command` (`strings.Fields(cmd)[0]`). Empty/whitespace-only command ⇒ `Installed = false`. Detection reads the live `PATH`; tests override `PATH` via `t.Setenv`.

5. **Save.** Upserts a harness from a `HarnessInfo`. Validation, in fixed order: slug grammar `[a-z0-9_]+` → `Name` non-empty → `Command` non-empty → builtin protection. For an EXISTING builtin slug, Save is permitted only when the submitted `Name` and `Command` are byte-identical to the stored values (i.e. only the provider map may change) — any change to a builtin's name or command → `errBuiltinReadonly`. `Command` is therefore editable only for customs; the UI shows builtin commands read-only (mockup exposes no command editor on the list, and the detail's command input persists only for customs). Save on a NEW slug creates a custom (`builtin = false` regardless of the DTO's `Builtin` field). `Installed` is ignored on input. Persists, emits `config:changed{section:"harnesses"}`.

6. **Delete.** Removes `[harnesses.<slug>]` for ANY harness, builtin or custom (see Deviations). Unknown slug → `errNotFound`. Persists, emits `config:changed{section:"harnesses"}`. A deleted builtin does NOT re-seed (§2.2 requires a fully empty section); it reappears only via manual config edit.

7. **SetProvider / SetAllProviders.** `SetProvider(slug, provider, on)` adds/removes `provider` in the harness's `providers` list (idempotent; list kept sorted ascending, no duplicates). `provider` must exist under `[providers.*]` → else `errValidation`. `SetAllProviders(slug, on)`: `on=true` sets the list to every configured provider id; `on=false` sets it to empty (mockup's Enable/Disable all). Both persist and emit `config:changed{section:"harnesses"}`; unknown slug → `errNotFound`.

8. **BuildCommand.** `BuildCommand(slug, modelID, reasoning)` substitutes the template: `strings.ReplaceAll` of `{model_id}` → modelID then `{reasoning}` → reasoning (a template without `{reasoning}` is valid — the replace no-ops). Afterwards any remaining token matching `\{[a-z0-9_]+\}` → `errValidation` naming the first offending token (CONTRACTS §6). `reasoning == "default"` substitutes verbatim. Unknown slug → `errNotFound`.

9. **Launch.** `Launch(ctx, slug, routeKey, profileSlug)`:
   1. `ParseRouteKey(routeKey)` — invalid grammar → `errValidation`.
   2. `BuildCommand(slug, modelID, reasoning)`.
   3. If `[gui].copy_command_instead` is true → return `LaunchResult{Copied: true, Command: cmd}`; nothing is spawned (the frontend copies to clipboard).
   4. Otherwise spawn `exec.Command(userShell(), "-lc", cmd)` detached: new session (unix `Setsid`; Windows: `CREATE_NEW_PROCESS_GROUP` — the sole platform difference, isolated in the two `sysproc` files), stdin nil, stdout+stderr appended to `<StateDir>/launch.log` (`O_APPEND|O_CREATE|O_WRONLY`, 0600). `cmd.Start()` error → `errLaunchFailed`; the process is released, never waited on. `userShell()` = `$SHELL`, fallback `/bin/sh`.
   5. On success (either the copy return or a successful spawn) record the pick via the §2.10 seam, then return `LaunchResult{Copied: false, Command: cmd}` (spawn path).
   A copy-mode launch is still a launch: the pick is recorded (matches the mockup, which increments the profile's pick count on every launch).

10. **RecordPick seam.** The pick is recorded by calling B04's `RecordPick(ctx, profileSlug, routeKey)`, reached through the unexported func field `Services.recordPick` which `New` wires to the B04 implementation. Until IM-B04 lands, IM-B07 may ship a no-op stub assigned to that field; tests inject a recording fake. A `RecordPick` failure after a successful spawn is logged via the seam's error return but does NOT fail the Launch (the process is already running); the `pick:recorded` event is emitted by `RecordPick` itself, never by this file.

11. **Events.** Mutations (Save, Delete, SetProvider, SetAllProviders, first-List seeding) emit exactly one `config:changed{section:"harnesses"}` each. Launch emits nothing directly (`pick:recorded` arrives via RecordPick). Read paths after seeding emit nothing.

## 3. Error behaviour

- All errors cross the boundary via B02's `toErrorDTO`: `errValidation` → `validation_failed`, `errBuiltinReadonly` → `builtin_readonly`, `errNotFound` → `not_found`, `errLaunchFailed` → `launch_failed`, file failures → `io_error`.
- Validation check order is fixed (§2.5, §2.9) so messages are golden-testable; exact strings in CONTRACTS §6.
- A failed config write leaves in-memory state untouched and emits no event (B00 §2.2).
- `errLaunchFailed` messages include the OS error but never the user's home path beyond `<StateDir>` (D00 §2.8).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Seed persistence | Builtins written to config on first List | Users edit builtin provider allow-maps; the map must live in TOML like any other harness state (see Deviations) |
| Builtin editability | Name/Command read-only; provider map editable; deletable | Mockup: no command editing surfaced for builtins, but every row has a remove control and the detail page toggles providers |
| Delete scope | Any harness, including builtins | Mockup `harnessRows[].onRemove` exists on every row; a removed builtin reappears only via manual config edit (§2.2 seeds only into an empty section) |
| Detection | `exec.LookPath` on argv0 at List time, never stored | PATH changes between launches; persisted flags go stale |
| Launch shell | `$SHELL` fallback `/bin/sh`, `-lc` | Login-shell semantics pick up the user's PATH/aliases where harness CLIs typically live |
| Copy-mode records pick | Yes | Mockup increments picks on every launch; copying IS the launch when `copy_command_instead` is on |
| No terminal window | v1 spawns headless, output to `<StateDir>/launch.log` | Spawning a visible terminal is platform-specific; documented follow-up, not v1 |
| RecordPick coupling | `Services.recordPick` func field wired in `New` | Keeps `harness.go` compilable and testable before/without IM-B04 |

## 5. Deviations

- **B00 CONTRACTS §6.4 (builtins never written to config):** harness builtin SEEDS are written to config on first List, exactly once. Rationale: unlike profiles/groups, a harness builtin carries per-user mutable state (the provider allow-map) with no separate storage; persisting the whole seed lets users edit that map with the same read-modify-write path as customs. Pre-authorised by B00 §6.4's own carve-out ("except the harness seed (B07 Deviations)").
- **B00 CONTRACTS §6.4 / D00 §4 `builtin_readonly` (builtins not deletable):** any harness, builtin included, is deletable (§2.6). Rationale: the mockup places a remove control on every harness row; `builtin_readonly` still protects builtin Name/Command edits.

## 6. Out of scope

- Config schema/accessors for `[harnesses.*]` — B01. Route-key parsing, DTO shapes, event names — D00. Error-code mapping, locking, test helper — B02.
- `RecordPick` implementation, history file — B04/B11.
- Clipboard write for copy mode, toast, popover close — frontend (U05) and host (S05).
- Harness auto-detection from each harness's own config files (mockup footnote "read from each harness' own config on launch") — follow-up; v1 detection is PATH-only.
- Terminal-window spawning; Windows/Linux launch polish beyond compiling and basic spawn.
