---
kind: feature-spec
version: "1.0"
feature: B10-settings
project: which-model-desktop
---

# B10-settings — GUI Settings and Agent Snippets

## 1. Purpose

`SettingsService` owns the `[gui]` config section plus the settings-page projection of `[auth].use_keychain` behind the Settings "General" page (mockup `generalToggles`, `shortcutOpts`, `holdOpts`, `dispCarousel`/`dispList`, `slStep`/`slBar`/`slSlider` blocks) and the "Agent integration" page's shell-hook snippets (mockup `agentRows` + `agentSnippet`). It exposes whole-struct get/set of `GUISettings` and the pinned `ShellSnippets` strings the UI renders verbatim.

Depends on: B02 (Services core), B01 (`[gui]`/`[auth]` schema + defaults). The snippet preview additionally calls B04's `Rank` (read-only).

## 2. Behaviour

1. **Get.** Returns the current `GUISettings` assembled from `[gui]` and `[auth].use_keychain`, with B01 defaults filling absent keys, plus `ConfigPath = paths.UserConfigFile` (display-only; the mockup's sidebar footer path). Never emits.

2. **Set is whole-struct replace.** The UI always sends the full `GUISettings`; the service validates every enum field in the fixed order of §2.3, rewrites the entire `[gui]` section and `[auth].use_keychain` on one cloned config document, persists that document atomically, swaps in-memory state, then emits `settings:changed` with the new `GUISettings` (including `ConfigPath`) as payload. `ConfigPath` on the input is ignored — it is never read or persisted.

3. **Validation order (fixed; messages exact, CONTRACTS §4).** (1) `Layout` ∈ {`carousel`,`list`}; (2) `WeightControl` ∈ {`step`,`bar`,`slider`}; (3) `Holds` ∈ {3,5,10}; (4) `Shortcut` ∈ {`alt+space`,`ctrl+space`,`cmd+shift+m`} — the three canonical strings (the UI renders them as ⌥␣ / ⌃␣ / ⇧⌘M); (5) `AutoUpdateFrequency` ∈ {`hourly`,`daily`,`weekly`,`monthly`}. First failure wins; no write, no event. Booleans need no validation.

4. **ShellSnippets.** Returns the three strings of D00's `ShellSnippets`, pinned verbatim in CONTRACTS §3: `Alias` (the `wm` shell alias the "Shell alias wm" toggle installs), `ClaudeMD` (the 3-line markdown hint the "Write a CLAUDE.md hint" toggle writes), and `Preview` — the mockup's `agentSnippet` line computed live: `$ wm <slug>  →  <model_id>  (<provider>)` where `<slug>` is the default profile (§2.5) and model/provider come from the top candidate of a `Rank` call with `RankRequest{ProfileSlug: <slug>, Holds: 3}`. When Rank errors or returns zero candidates, `Preview` is `$ wm <slug>  →  no route`; `ShellSnippets` itself never fails for ranking reasons.

5. **Default profile slug.** `[strategy].default_profile`, falling back to `balanced_implementation` when unset — the same key/default the CLI uses (`docs/plan/annex-d-cli-reference.md` `[strategy]`).

6. **Toggles are persistence-only here.** `MCPServer`, `ClaudeMDHint`, `ShellAlias`, `LaunchAtLogin`, `ShowMenuBarIcon`, `AutoUpdate*` are stored and emitted; acting on them (hotkey registration, LaunchAgent, tray visibility) is S05's job, and the real MCP server is out of scope (plan §8).

## 3. Error behaviour

- Enum/holds violations → `errValidation` → `validation_failed`; the message names the field and the offending value (CONTRACTS §4); check order is fixed so messages are golden-testable.
- Persist failure → `io_error`; in-memory settings unchanged; no event.
- `Get`/`ShellSnippets` fail only on internal state errors (missing catalog never blocks them: ranking failures degrade to the `no route` preview).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Set granularity | Whole-struct replace of `[gui]` | One durable write + one event per user action; UI controls each send the full struct with one field changed (U12) |
| ConfigPath handling | Output-only, injected from `paths.UserConfigFile`, ignored on Set | Display convenience; paths never come from the frontend |
| Shortcut vocabulary | Exactly `alt+space`, `ctrl+space`, `cmd+shift+m` | Closed set S05 can map to `golang.design/x/hotkey`; UI owns glyph rendering |
| Preview rank source | B04 `Rank` with the `[strategy].default_profile` slug, Holds 3, top candidate only | Matches mockup `agentSnippet` (`'$ wm ' + slug + ' → ' + id + ' (' + route + ')'`); reuses the one ranking path |
| Preview degradation | `no route` suffix on rank error/empty | Snippets page must render even with an empty availability set |
| Snippet strings pinned | Exact strings live in CONTRACTS §3 | UI and future installer write them verbatim; golden-testable |
