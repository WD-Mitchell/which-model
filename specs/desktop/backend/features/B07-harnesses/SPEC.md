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

2. **Builtin reconciliation.** Every List adds missing builtins from CONTRACTS §3 in one atomic write/event, even with existing custom harnesses. Existing custom entries and explicit enable/provider switches survive. Exact old factory provider lists migrate to automatic discovery, and known obsolete factory commands migrate to current CLI commands. Subsequent reads are write-free until another migration is needed.

3. **List and configured providers.** Return harnesses by slug. The provider map spans B06's full known universe (config, routes, registered providers and cached catalog). Missing `providers` means automatic discovery for installed builtins; an explicit list, including empty, is a complete manual selection. `provider_overrides` applies individual switches last, so discovering a new provider never restores one explicitly switched off. Discovery is local and read-only, with no credential copy, network request, subprocess, or config-expression evaluation. Malformed, missing, oversized or non-regular config files contribute no providers. Recognized aliases map into which-model's provider ids.

4. **Installation and discovery sources.** Check command argv0 on PATH each List. Native desktop detection also checks Homebrew, /usr/local/bin and standard user CLI install directories when the GUI inherits a minimal PATH. Read only known per-user configuration paths from CONTRACTS §3; no project scan or guessed model-family-to-provider mapping. A provider listed here is configured, not freshly authenticated. Amp, Continue, Crush, Kiro and Windsurf select models through their own settings; their launch commands intentionally contain no model placeholder and the UI explains this.

5. **Save.** Upserts a harness from a `HarnessInfo`. Validation, in fixed order: slug grammar `[a-z0-9_]+` → `Name` non-empty → `Command` non-empty → builtin protection. For an EXISTING builtin slug, Save is permitted only when the submitted `Name` and `Command` are byte-identical to the stored values (i.e. only the provider map may change) — any change to a builtin's name or command → `errBuiltinReadonly`. `Command` is therefore editable only for customs; the UI shows builtin commands read-only (mockup exposes no command editor on the list, and the detail's command input persists only for customs). Save on a NEW slug creates a custom (`builtin = false` regardless of the DTO's `Builtin` field). `Installed` is ignored on input. Persists, emits `config:changed{section:"harnesses"}`.

6. **Delete.** Remove custom harnesses only. Builtin removal returns `errBuiltinReadonly`, matching the disabled Remove control and keeping reconciliation deterministic. Unknown slug returns `errNotFound`.

7. **Provider switches.** SetProvider writes one `provider_overrides` boolean while retaining automatic discovery and other switches. Save's full provider map and SetAllProviders are explicit manual selections. Bulk off remains off after discovery. Provider validation uses B06's full universe. SetEnabled preserves all provider settings. Mutations remain atomic and emit one config-changed event.

8. **BuildCommand.** `BuildCommand(slug, modelID, reasoning)` substitutes the template: `strings.ReplaceAll` of `{model_id}` → modelID then `{reasoning}` → reasoning (a template without `{reasoning}` is valid — the replace no-ops). Afterwards any remaining token matching `\{[a-z0-9_]+\}` → `errValidation` naming the first offending token (CONTRACTS §6). `reasoning == "default"` substitutes verbatim. Unknown slug → `errNotFound`.

9. **Launch.** `Launch(ctx, slug, routeKey, profileSlug)`:
   1. `ParseRouteKey(routeKey)` — invalid grammar → `errValidation`.
   2. `BuildCommand(slug, modelID, reasoning)`. Builtin OpenCode and Kilo qualify model ids with the picked provider's catalog slug; builtin Cline adds its provider flag. Custom commands retain their exact template semantics.
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
- **Builtin deletion correction:** the owner requested a complete evolving registry. Builtins are now retained, matching the existing UI; custom harnesses remain removable.

## 6. Out of scope

- Config schema/accessors for `[harnesses.*]` — B01. Route-key parsing, DTO shapes, event names — D00. Error-code mapping, locking, test helper — B02.
- `RecordPick` implementation, history file — B04/B11.
- Clipboard write for copy mode, toast, popover close — frontend (U05) and host (S05).
- Project-specific configuration, environment-only provider settings, and live authentication checks remain outside global harness discovery.
- Terminal-window spawning; Windows/Linux launch polish beyond compiling and basic spawn.

## Review correction — #178: preserve enabled overrides

Save of an existing harness, SetProvider, and SetAllProviders preserve its stored `Enabled` pointer exactly: nil means installation detection, false explicitly disables, true explicitly enables. A new custom harness starts with nil; only SetEnabled changes the override. The derived boolean in HarnessInfo is not a persistence override.

Pinned regression: `TestHarnessEditsPreserveEnabledOverride` covers all nine edit × nil/false/true combinations through persisted reload.

## Review correction — #179: atomic harness mutations

Every mutation, including initial seeding, Save, Delete, SetProvider, SetAllProviders, and SetEnabled, clones the complete configuration through `cloneConfig` while holding the write lock. It mutates only that independent document and publishes it after the atomic write succeeds. Clone cleanup and lock release occur on every error. Failed writes leave both live and persisted state unchanged and emit no event.

Pinned regression: `TestHarnessFailedMutationLeavesLiveConfigUnchanged` forces all six mutations to fail at an invalid destination, verifies byte-identical live TOML and no event, then verifies a later successful write cannot leak the rejected change.


## Transaction boundary correction — #179 review

Config cloning preserves raw values, typed provider values, and deferred env
overrides directly. A pre-rename failure leaves both disk and memory unchanged.
If rename succeeds but directory sync fails, publish the now-visible config and
emit one config-changed event, then return a classified committed-write error
so durability failure remains visible. The caller must not treat this error as
a rollback. Pin post-commit filesystem and harness state/event regressions.

### September 2026 owner-requested registry and discovery

This supersedes the initial four-entry spec and seven-entry implementation, first-run-only seeding, fixed provider guesses, and pip-based counts. Existing factory values migrate without changing customized commands or explicit switches. The user asked for common harnesses including Cline and OpenCode, automatic provider detection, and numeric provider counts.

Discovered gateways absent from the global catalog (for example Cline's gateway) remain in the harness provider map and numeric count. Detail shows a switch and `Configured in this harness`. This metadata does not add/enable a global provider or trigger usage reads. Explicit switches and bulk changes include these ids.

Launch uses the current native effort controls: non-default Claude effort uses `--effort` (minimal is omitted because Claude does not accept it); Codex uses `-c model_reasoning_effort=…`. A default reasoning pick adds no effort override. Cline retains its configured OAuth adapter id when it differs from the canonical provider alias.
