---
kind: feature-contracts
version: "1.0"
feature: B07-harnesses
project: which-model-desktop
---

# B07-harnesses — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/harness.go` | `HarnessService`, `Services.Harnesses()`, List/Save/Delete/SetProvider/SetAllProviders/BuildCommand/Launch, `userShell`, seed table |
| `internal/service/harness_sysproc_unix.go` | `//go:build !windows` — `launchSysProcAttr()` with `Setsid: true` |
| `internal/service/harness_sysproc_windows.go` | `//go:build windows` — `launchSysProcAttr()` with `CreationFlags: CREATE_NEW_PROCESS_GROUP` |
| `internal/service/harness_test.go` | fixtures per §7 |

Types used, not defined here: `HarnessInfo`, `LaunchResult`, `ParseRouteKey` (D00); sentinels `errValidation`/`errBuiltinReadonly`/`errNotFound`/`errLaunchFailed`, `toErrorDTO`, `newTestServices` (B02); `[harnesses.*]` TOML accessors (B01).

## 2. Exported API

```go
package service

// HarnessService owns harness registry, detection, substitution, and launch.
// Obtained via Services.Harnesses(); shares the Services mutex and emit.
type HarnessService struct { /* unexported: s *Services */ }

func (s *Services) Harnesses() *HarnessService

// List returns all harnesses, slug ascending. Seeds the four builtins into
// config first iff [harnesses] has no subtables (SPEC §2.2; one write, one
// config:changed event). Installed is recomputed each call (SPEC §2.4).
func (h *HarnessService) List(ctx context.Context) ([]HarnessInfo, error)

// Save upserts. Validation order: slug grammar -> name -> command -> builtin
// protection (SPEC §2.5). Builtin slugs: Name/Command must equal stored
// values or errBuiltinReadonly; only Providers may change. New slugs are
// created custom. Emits config:changed{section:"harnesses"}.
func (h *HarnessService) Save(ctx context.Context, in HarnessInfo) error

// Delete removes ANY harness, builtin or custom (SPEC Deviations).
// Unknown slug -> errNotFound. Emits config:changed{section:"harnesses"}.
func (h *HarnessService) Delete(ctx context.Context, slug string) error

// SetProvider toggles one provider in the harness allow-list (idempotent;
// list stored sorted, deduplicated). Unknown slug -> errNotFound; provider
// not configured under [providers.*] -> errValidation. Emits config:changed.
func (h *HarnessService) SetProvider(ctx context.Context, slug, provider string, on bool) error

// SetAllProviders: on=true -> list = every configured provider id;
// on=false -> empty list. Unknown slug -> errNotFound. Emits config:changed.
func (h *HarnessService) SetAllProviders(ctx context.Context, slug string, on bool) error

// BuildCommand substitutes {model_id} then {reasoning} (ReplaceAll); any
// remaining \{[a-z0-9_]+\} token -> errValidation naming the first one
// (§6 #4). Unknown slug -> errNotFound. Pure; no side effects.
func (h *HarnessService) BuildCommand(slug, modelID, reasoning string) (string, error)

// Launch parses routeKey, builds the command, then either returns it for
// copying ([gui].copy_command_instead) or spawns it detached via
// userShell() -lc with launchSysProcAttr(), stdout/stderr appended to
// <StateDir>/launch.log (0600). Start failure -> errLaunchFailed. On
// success (both modes) records the pick via Services.recordPick; a record
// failure is logged, not returned (SPEC §2.10).
func (h *HarnessService) Launch(ctx context.Context, slug, routeKey, profileSlug string) (LaunchResult, error)
```

Unexported, contract-relevant: `func userShell() string` ($SHELL, fallback `/bin/sh`); `func launchSysProcAttr() *syscall.SysProcAttr` (platform files); `Services.recordPick func(ctx context.Context, profileSlug, routeKey string) error` — field on `Services`, wired by `New` to B04's RecordPick, no-op stub permitted until IM-B04.

## 3. Builtin registry and local discovery

All paths are relative to the OS user home. All new builtin `providers` arrays are omitted to enable discovery.

| Slug | Name | Command | Configuration |
|---|---|---|---|
| `aider` | Aider | `aider --model {model_id}` | `.aider.conf.yml` |
| `amp` | Amp | `amp` | `.config/amp/settings.json` |
| `antigravity` | Antigravity | `agy --model {model_id}` | `.gemini/antigravity` directory |
| `claude` | Claude Code | `claude --model {model_id}` | `.claude/settings.json`, `.claude/settings.local.json`, `.claude/.credentials.json`, `.claude.json` OAuth account metadata |
| `cline` | Cline | `cline --model {model_id}` | `.cline/data/settings/providers.json`, `.cline/data/globalState.json` |
| `codex` | Codex CLI | `codex -m {model_id}` | `.codex/config.toml`, `.codex/auth.json` |
| `continue` | Continue | `cn` | `.continue/config.yaml` or `.continue/config.json` |
| `copilot` | Copilot CLI | `copilot --model {model_id}` | `.copilot/config.json` |
| `crush` | Crush | `crush` | `.config/crush/crush.json` |
| `cursor` | Cursor Agent | `cursor-agent --model {model_id}` | `.cursor/cli-config.json`, `.config/cursor/auth.json` |
| `droid` | Factory Droid | `droid --model {model_id}` | `.factory/settings.json`, `.factory/settings.local.json` |
| `gemini` | Gemini CLI | `gemini --model {model_id}` | `.gemini/settings.json`, `.gemini/oauth_creds.json` |
| `goose` | Goose | `goose session --model {model_id}` | `.config/goose/config.yaml` |
| `kilo` | Kilo Code | `kilo --model {model_id}` | `.config/kilo/kilo.json[ c ]` (json/jsonc), `.local/share/kilo/auth.json` |
| `kiro` | Kiro CLI | `kiro-cli chat` | No provider adapter; manual switches remain available |
| `opencode` | OpenCode | `opencode --model {model_id}` | `.config/opencode/opencode.json[ c ]` (json/jsonc), `.local/share/opencode/auth.json` |
| `qwen` | Qwen Code | `qwen --model {model_id}` | `.qwen/settings.json`, `.qwen/oauth_creds.json` |
| `windsurf` | Windsurf | `windsurf` | `.codeium/windsurf` directory |

Read regular files up to 2 MiB. JSON, JSONC (comments/trailing commas), TOML and YAML are supported. Extract declared provider identifiers only; retain no secret fields or source document in DTOs/config. OpenCode/Kilo enabled/disabled lists restrict discovery; disabled Crush providers are excluded. Unavailable or malformed documents degrade to empty discovery. Provider aliases include anthropic→claude, openai→codex, github-copilot→copilot and gemini→google. Per-provider switches override discovery and survive reload.

## 4. Config keys consumed (schema owned by B01)

| Key | Type | Meaning |
|---|---|---|
| `harnesses.<slug>.name` | string | display name |
| `harnesses.<slug>.command` | string | template with `{model_id}`/`{reasoning}` |
| `harnesses.<slug>.providers` | optional []string | omitted = discovery; explicit array = manual allow-list |
| `harnesses.<slug>.provider_overrides` | map[string]bool | individual switches applied after discovery or manual list |
| `harnesses.<slug>.builtin` | bool | seeded builtins true; customs false |
| `gui.copy_command_instead` | bool | Launch copy mode (SPEC §2.9.3) |

## 5. Substitution and detection rules

1. Tokens recognised: `{model_id}`, `{reasoning}` — replaced everywhere they appear, in that order, via `strings.ReplaceAll`. Absence of either token is valid.
2. Leftover scan: regexp `\{[a-z0-9_]+\}` over the substituted string; first match errors. Braces not matching the pattern (e.g. `{}`, `{Foo}`) pass through untouched.
3. `reasoning` substitutes verbatim, including `"default"`.
4. Detection: `argv0 = strings.Fields(Command)[0]`; `Installed = exec.LookPath(argv0) == nil`; empty `Fields` ⇒ false. No caching, no persistence.

## 6. Error messages (exact; sentinel → code via B02 §3)

| # | Condition | Sentinel | Message |
|---|---|---|---|
| 1 | slug fails `[a-z0-9_]+` | errValidation | `harness slug %q must match [a-z0-9_]+` |
| 2 | empty name | errValidation | `harness name must not be empty` |
| 3 | empty command | errValidation | `harness command must not be empty` |
| 4 | unresolved token | errValidation | `harness %q: unresolved template token %q` (slug, token incl. braces) |
| 5 | builtin name/command edit | errBuiltinReadonly | `harness %q is builtin: name and command are read-only` |
| 6 | unknown slug (Delete/SetProvider/SetAllProviders/BuildCommand/Launch) | errNotFound | `harness %q not found` |
| 7 | unknown provider id | errValidation | `unknown provider %q` |
| 8 | bad route key | errValidation | D00 ParseRouteKey message |
| 9 | spawn failure | errLaunchFailed | `launch %q: %v` (slug, OS error) |

## 7. Test fixtures (`harness_test.go`; TDD-first)

1. **Seed on first List**: empty-config `newTestServices`; first `List` returns exactly the §3 eighteen (slug asc), config file now contains all eighteen subtables with `builtin = true`, recorder shows one `config:changed{section:"harnesses"}`; second `List` emits nothing; a custom harness remains intact while missing builtins are added.
2. **Substitution table**: (template, modelID, reasoning) → expected command for representative model-selecting seed templates; `{reasoning}`-free templates ignore reasoning; template `x {model_id} {custom_flag}` → errValidation with message #4 naming `{custom_flag}`; unknown slug → errNotFound.
3. **Detection with fake PATH**: `t.Setenv("PATH", dir)` with an executable stub named `claude` ⇒ `claude.Installed == true`, others false; empty-command harness ⇒ false.
4. **Save semantics**: custom round-trip Save→List; builtin provider-map-only Save succeeds; builtin with changed Command → errBuiltinReadonly (message #5); validation order asserted (bad slug + empty name reports #1).
5. **Delete any**: deleting `claude` returns builtin_readonly and emits nothing; deleting a custom succeeds and it does not return; unknown slug → errNotFound.
6. **SetProvider/SetAllProviders**: toggle round-trip, idempotence (repeat = same config bytes, still one event per call), unknown provider → #7, all-on/all-off lists.
7. **Copy mode**: `copy_command_instead = true` ⇒ Launch returns `{Copied:true, Command:<substituted>}`, spawns nothing (no `launch.log` created), recordPick fake called once with (profileSlug, routeKey).
8. **Spawn failure maps to launch_failed**: harness whose argv0/shell cannot start (point `$SHELL` at a non-existent path) ⇒ error with `toErrorDTO(...).Code == "launch_failed"`, recordPick NOT called, no event emitted.
9. **Spawn success**: `$SHELL=/bin/sh`, command `echo ok`; Launch returns `{Copied:false}`, `<StateDir>/launch.log` eventually contains `ok`, recordPick called once (unix-only test; skipped on Windows).

Every mutation test asserts exactly one event via the B02 recorder; read/validation-failure paths assert zero (B00 §6.5).

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

Pinned regressions: `TestDiscoverHarnessProviders` covers documented formats and aliases, provider allow/deny lists, malformed/oversized input, and unchanged source bytes. `TestHarnessDiscoveryPreservesExplicitSwitches` covers new discovery after an off override, enable-toggle preservation and bulk off. `TestHarnessLegacyMigrationPreservesOverrides` covers old Cursor command correction without enabling it. `TestHarnessQualifiedLaunchCommands` covers OpenCode/Kilo provider/model arguments and Cline provider selection.

Discovered gateways absent from the global catalog (for example Cline's gateway) remain in the harness provider map and numeric count. Detail shows a switch and `Configured in this harness`. This metadata does not add/enable a global provider or trigger usage reads. Explicit switches and bulk changes include these ids.

Launch uses the current native effort controls: non-default Claude effort uses `--effort` (minimal is omitted because Claude does not accept it); Codex uses `-c model_reasoning_effort=…`. A default reasoning pick adds no effort override. Cline retains its configured OAuth adapter id when it differs from the canonical provider alias.
