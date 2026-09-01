---
kind: feature-contracts
version: "1.0"
feature: B10-settings
project: which-model-desktop
---

# B10-settings — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/settings.go` | `SettingsService`, `(*Services).Settings()`, Get/Set/ShellSnippets, snippet constants |
| `internal/service/settings_test.go` | table tests per §5 |

Package `internal/service` (B00 import boundary applies). DTOs are D00 canon (`GUISettings`, `ShellSnippets`). `[gui]` and `[auth].use_keychain` key names, types, and defaults are owned by B01 (`internal/config/gui.go`, `auth.go`); this feature owns their settings-service semantics and GUI validation messages.

## 2. Exported API — `internal/service/settings.go`

```go
package service

// SettingsService is the settings facet of Services; the host registers it
// as a Wails service. Zero value unusable; obtain via Settings().
type SettingsService struct { s *Services }

// Settings returns the settings facet (shares the Services lock and config).
func (s *Services) Settings() *SettingsService

// Get assembles GUISettings from [gui] and [auth] + B01 defaults;
// ConfigPath is always paths.UserConfigFile (SPEC §2.1). Read-only; never emits.
func (g *SettingsService) Get(ctx context.Context) (GUISettings, error)

// Set validates in the fixed order of SPEC §2.3 (messages §4), replaces the
// whole [gui] section plus [auth].use_keychain in one atomic config write, and
// emits settings:changed with the new GUISettings (ConfigPath filled) as
// payload. in.ConfigPath ignored.
func (g *SettingsService) Set(ctx context.Context, in GUISettings) error

// ShellSnippets returns the pinned Alias and ClaudeMD strings (§3) and the
// live Preview line (SPEC §2.4–2.5). Ranking failure degrades the Preview
// to the "no route" form; it never propagates.
func (g *SettingsService) ShellSnippets(ctx context.Context) (ShellSnippets, error)
```

## 3. Pinned snippet strings (exact; Go raw-string constants in `settings.go`)

`ShellSnippets.Alias` — one line, single quotes as shown:

```
alias wm='which-model pick --profile'
```

`ShellSnippets.ClaudeMD` — exactly these 3 lines, no trailing newline:

```
## Model selection
Before delegating work, run `wm <profile>` to print the best model id for that task profile.
`wm` is an alias for `which-model pick --profile`; profiles live in which-model's config.toml.
```

`ShellSnippets.Preview` — computed, format exact (two spaces around `→`, two before `(`):

| Case | Format |
|---|---|
| Rank returned ≥1 candidate | `$ wm <slug>  →  <model_id>  (<provider>)` |
| Rank error or 0 candidates | `$ wm <slug>  →  no route` |

`<slug>` = `[strategy].default_profile` or `balanced_implementation`; `<model_id>`/`<provider>` from `Rank(ctx, RankRequest{ProfileSlug: slug, Holds: 3})` candidate 1 (B04).

## 4. Validation error strings (exact, checked in this order; all → `validation_failed`)

| # | String |
|---|---|
| 1 | `gui: layout must be "carousel" or "list", got %q` |
| 2 | `gui: weight_control must be "step", "bar", or "slider", got %q` |
| 3 | `gui: holds must be 3, 5, or 10, got %d` |
| 4 | `gui: shortcut must be "alt+space", "ctrl+space", or "cmd+shift+m", got %q` |
| 5 | `gui: auto_update_frequency must be "hourly", "daily", "weekly", or "monthly", got %q` |

## 5. Test fixtures (`settings_test.go`)

Built on `newTestServices(t, ...)` (B02). Required cases:

1. **Defaults**: empty config → Get returns B01's `[gui]` defaults, `UseKeychain = true`, and `ConfigPath` = the temp `UserConfigFile`.
2. **Round-trip**: Set a fully non-default struct → Get returns it field-for-field; config.toml on disk contains the `[gui]` keys and `[auth].use_keychain`; unknown keys elsewhere in the file preserved.
3. **Event**: successful Set → recorder shows exactly one `settings:changed` whose payload equals the new `GUISettings` (ConfigPath filled); failed Set → zero events.
4. **Enum rejection table**: one case per §4 row (bad layout/weight_control/holds/shortcut/frequency) asserting exact message and `validation_failed`; a struct with two bad fields fails on the earlier-ordered one.
5. **ConfigPath ignored**: Set with `ConfigPath: "/evil"` succeeds; disk has no such value; Get still reports the real path.
6. **Snippets pinned**: `Alias` and `ClaudeMD` equal §3 byte-for-byte (ClaudeMD has exactly 3 lines).
7. **Preview**: with the fixture catalog, Preview matches `$ wm <slug>  →  <model_id>  (<provider>)` for the known top candidate; with all providers disabled (empty availability), Preview is the `no route` form and the call still succeeds.
