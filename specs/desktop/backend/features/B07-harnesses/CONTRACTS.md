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

## 3. Builtin seed table (exact; written on first List)

| Slug | Name | Command template | Seed providers |
|---|---|---|---|
| `claude` | `Claude Code` | `claude --model {model_id} --reasoning {reasoning}` | `["claude","codex","copilot"]` |
| `codex` | `Codex CLI` | `codex -m {model_id} -c reasoning={reasoning}` | `["codex","copilot"]` |
| `copilot` | `Copilot CLI` | `copilot --model {model_id}` | `["copilot","cursor"]` |
| `cursor` | `Cursor` | `cursor --model {model_id}` | `["cursor"]` |

All seeded with `builtin = true`. Seed provider lists mirror the mockup's `hp` initial state; ids not configured under `[providers.*]` at List time stay in config but are omitted from `HarnessInfo.Providers` (SPEC §2.3).

## 4. Config keys consumed (schema owned by B01)

| Key | Type | Meaning |
|---|---|---|
| `harnesses.<slug>.name` | string | display name |
| `harnesses.<slug>.command` | string | template with `{model_id}`/`{reasoning}` |
| `harnesses.<slug>.providers` | []string | allow-list; membership = enabled for this harness |
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

1. **Seed on first List**: empty-config `newTestServices`; first `List` returns exactly the §3 four (slug asc), config file now contains all four subtables with `builtin = true`, recorder shows one `config:changed{section:"harnesses"}`; second `List` emits nothing; `WithConfigTOML` containing one custom harness ⇒ no seeding ever.
2. **Substitution table**: (template, modelID, reasoning) → expected command for all four seed templates; `{reasoning}`-free templates ignore reasoning; template `x {model_id} {custom_flag}` → errValidation with message #4 naming `{custom_flag}`; unknown slug → errNotFound.
3. **Detection with fake PATH**: `t.Setenv("PATH", dir)` with an executable stub named `claude` ⇒ `claude.Installed == true`, others false; empty-command harness ⇒ false.
4. **Save semantics**: custom round-trip Save→List; builtin provider-map-only Save succeeds; builtin with changed Command → errBuiltinReadonly (message #5); validation order asserted (bad slug + empty name reports #1).
5. **Delete any**: deleting `claude` succeeds, emits one event, does not re-seed on next List; unknown slug → errNotFound.
6. **SetProvider/SetAllProviders**: toggle round-trip, idempotence (repeat = same config bytes, still one event per call), unknown provider → #7, all-on/all-off lists.
7. **Copy mode**: `copy_command_instead = true` ⇒ Launch returns `{Copied:true, Command:<substituted>}`, spawns nothing (no `launch.log` created), recordPick fake called once with (profileSlug, routeKey).
8. **Spawn failure maps to launch_failed**: harness whose argv0/shell cannot start (point `$SHELL` at a non-existent path) ⇒ error with `toErrorDTO(...).Code == "launch_failed"`, recordPick NOT called, no event emitted.
9. **Spawn success**: `$SHELL=/bin/sh`, command `echo ok`; Launch returns `{Copied:false}`, `<StateDir>/launch.log` eventually contains `ok`, recordPick called once (unix-only test; skipped on Windows).

Every mutation test asserts exactly one event via the B02 recorder; read/validation-failure paths assert zero (B00 §6.5).
