---
kind: feature-contracts
version: "1.0"
feature: B01-config-schema
project: which-model-desktop
---

# B01-config-schema — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/config/gui.go` | §2 types, `Load*`/`Set*`/`Delete*` accessors, `DefaultGUIConfig`, validators, re-declared route-key regexps |
| `internal/config/gui_test.go` | accessor/validator table tests + golden round-trip (§6) |
| `internal/config/auth.go`, `auth_test.go` | `[auth]` storage preference, defaulting, and round-trip |
| `internal/config/write.go` | `AtomicWriteFile` |
| `internal/config/write_test.go` | atomic-write tests (§6) |
| `internal/config/marshal.go` (change site) | render list extended per SPEC §2.6; trailing sorted unknown-section loop |
| `pkg/whichmodel/config_cmd.go` (change site) | `atomicWrite(path, out)` call → `config.AtomicWriteFile(path, out)`; local `atomicWrite` deleted |

Import boundary: `internal/config` gains no new imports beyond stdlib (`os`, `path/filepath`, `regexp`, `sort`, `fmt`, `strings`). MUST NOT import `internal/pick`, `internal/service`, `internal/routing`.

## 2. Exported API — `internal/config/gui.go`

```go
package config

// ProfileTOML mirrors one [profiles.<slug>] table. Weights are ints 1..5;
// zero-weight keys are never stored (SPEC §4 Decisions).
type ProfileTOML struct {
    CoreShare int            `toml:"core_share"` // 10..90, step 5
    Tier1     map[string]int `toml:"tier1"`      // keys exactly intelligence/cost/speed
    Tier2     map[string]int `toml:"tier2"`      // keys ⊆ categories ∪ [groups.*] slugs
}
type ProfilesTOML map[string]ProfileTOML

// HarnessTOML mirrors one [harnesses.<slug>] table (seeded by B07).
type HarnessTOML struct {
    Name      string   `toml:"name"`
    Command   string   `toml:"command"`   // template; token semantics are B07's
    Providers []string `toml:"providers"` // provider slugs
    Builtin   bool     `toml:"builtin"`
}
type HarnessesTOML map[string]HarnessTOML

// FavouritesTOML mirrors [favourites]. Pins are D00 §1 route keys.
type FavouritesTOML struct {
    Pins []string `toml:"pins"`
}

// RoutesDisabledTOML mirrors [routes.disabled]: provider -> "model_id@reasoning".
type RoutesDisabledTOML map[string][]string

// GroupTOML mirrors one [groups.<slug>] custom benchmark group.
type GroupTOML struct {
    Benchmarks []string `toml:"benchmarks"`
}
type GroupsTOML map[string]GroupTOML

// GUIConfig mirrors [gui]; field meanings and value sets are D00 GUISettings
// (which adds the transport-only ConfigPath — deliberately absent here).
type GUIConfig struct {
    Layout                  string `toml:"layout"`
    WeightControl           string `toml:"weight_control"`
    Holds                   int    `toml:"holds"`
    Shortcut                string `toml:"shortcut"`
    ShowMenuBarIcon         bool   `toml:"show_menu_bar_icon"`
    LaunchAtLogin           bool   `toml:"launch_at_login"`
    CopyCommandInstead      bool   `toml:"copy_command_instead"`
    ClosePopoverAfterLaunch bool   `toml:"close_popover_after_launch"`
    AutoUpdate              bool   `toml:"auto_update"`
    AutoUpdateFrequency     string `toml:"auto_update_frequency"`
    MCPServer               bool   `toml:"mcp_server"`
    ClaudeMDHint            bool   `toml:"claude_md_hint"`
    ShellAlias              bool   `toml:"shell_alias"`
    CatalogRepo             string `toml:"catalog_repo"`
    UseLocalAA              bool   `toml:"use_local_aa"`
    OnlyEnabledProviders    bool   `toml:"only_enabled_providers"`
}

// AuthConfig mirrors [auth]. It controls only credentials written by
// which-model; provider-native keychain sources are unaffected.
type AuthConfig struct {
    UseKeychain bool `toml:"use_keychain" json:"use_keychain"`
}

func DefaultAuthConfig() AuthConfig // UseKeychain: true

// DefaultGUIConfig returns the §4 defaults.
func DefaultGUIConfig() GUIConfig

// Load accessors: decode via UnmarshalKey, apply defaults (GUI only), then
// validate in the §5 order. Absent section ⇒ (defaults|empty, nil).
// categories is the canonical tier2 vocabulary (callers pass
// pick.CategoryNames); it is unioned with the [groups.*] slugs internally.
func (c *Config) LoadGUI() (GUIConfig, error)
func (c *Config) LoadAuth() (AuthConfig, error)
func (c *Config) LoadProfiles(categories []string) (ProfilesTOML, error)
func (c *Config) LoadHarnesses() (HarnessesTOML, error)
func (c *Config) LoadFavourites() (FavouritesTOML, error)
func (c *Config) LoadRoutesDisabled() (RoutesDisabledTOML, error)
func (c *Config) LoadGroups() (GroupsTOML, error)

// Setters: validate (§5 order, same messages), then write plain values into
// the raw document so MarshalTOML round-trips them (SPEC §2.5). No disk I/O.
func (c *Config) SetGUI(g GUIConfig) error
func (c *Config) SetAuth(a AuthConfig) error
func (c *Config) SetProfile(slug string, p ProfileTOML, categories []string) error
func (c *Config) SetHarness(slug string, h HarnessTOML) error
func (c *Config) SetFavourites(f FavouritesTOML) error
func (c *Config) SetRoutesDisabled(r RoutesDisabledTOML) error
func (c *Config) SetGroup(slug string, g GroupTOML) error

// Deletes: remove the slug's raw table; idempotent, never error.
func (c *Config) DeleteProfile(slug string)
func (c *Config) DeleteHarness(slug string)
func (c *Config) DeleteGroup(slug string)
```

## 3. Exported API — `internal/config/write.go`

```go
// AtomicWriteFile durably replaces path with data: MkdirAll(dir, 0o755),
// temp file in dir chmodded 0o600, write, fsync, close, rename over path,
// fsync dir. On error the temp file is removed and path is untouched.
// Promoted from pkg/whichmodel/config_cmd.go atomicWrite (SPEC §2.8); the
// single write path for CLI `config set` and all B02+ service mutations.
func AtomicWriteFile(path string, data []byte) error
```

## 4. Config keys owned (full schema, defaults in comments)

```toml
[profiles.<slug>]                 # slug: [a-z0-9_]+
core_share = 60                   # int 10..90 step 5; REQUIRED
[profiles.<slug>.tier1]           # REQUIRED; keys exactly these three
intelligence = 4                  # int 1..5
cost = 3
speed = 3
[profiles.<slug>.tier2]           # optional; keys ⊆ categories ∪ group slugs
software_engineering = 5          # int 1..5

[harnesses.<slug>]
name = "Claude Code"              # REQUIRED, non-empty
command = "claude --model {model_id} --reasoning {reasoning}"  # REQUIRED
providers = ["claude", "codex"]   # provider slugs; may be empty
builtin = true                    # default false

[favourites]
pins = ["codex/gpt-5.6-luna@max"] # D00 §1 route keys; default []

[routes.disabled]                 # provider -> disabled "model_id@reasoning"
claude = ["claude-opus-5@low"]    # default: no keys

[groups.<slug>]
benchmarks = ["SWE-Bench Verified"] # REQUIRED, non-empty, unique

[gui]                             # all keys optional; per-key defaults:
layout = "carousel"               # "carousel"|"list"
weight_control = "slider"         # "step"|"bar"|"slider"
holds = 5                         # 3|5|10
shortcut = "alt+space"            # "alt+space"|"ctrl+space"|"cmd+shift+m"
show_menu_bar_icon = true
launch_at_login = false
copy_command_instead = false
close_popover_after_launch = true
auto_update = true
auto_update_frequency = "daily"   # "hourly"|"daily"|"weekly"|"monthly"
mcp_server = false
claude_md_hint = false
shell_alias = false

[auth]                            # all keys optional; per-key defaults:
use_keychain = true               # prefer OS keychain; false forces state-file storage
```

None of these keys is env-addressable (SPEC §2.7).

## 5. Validation errors (exact, checked in this order)

All are `&ConfigError{Kind: KindInvalidValue, Key: <Key>, Err: errors.New(<Detail>)}`, rendering `config: invalid value for <Key>: <Detail>`. Slugs/keys iterate sorted ascending; list entries in list order; validation stops at the first failure.

| # | Section | Key | Detail |
|---|---|---|---|
| P1 | profiles | `profiles.<slug>` | `slug must match [a-z0-9_]+` |
| P2 | | `profiles.<slug>.core_share` | `must be between 10 and 90` |
| P3 | | `profiles.<slug>.core_share` | `must be a multiple of 5` |
| P4 | | `profiles.<slug>.tier1` | `keys must be exactly intelligence, cost, speed` |
| P5 | | `profiles.<slug>.tier1.<axis>` (intelligence, cost, speed order) | `must be between 1 and 5` |
| P6 | | `profiles.<slug>.tier2.<key>` | `unknown tier2 category` |
| P7 | | `profiles.<slug>.tier2.<key>` | `must be between 1 and 5` |
| H1 | harnesses | `harnesses.<slug>` | `slug must match [a-z0-9_]+` |
| H2 | | `harnesses.<slug>.name` | `must not be empty` |
| H3 | | `harnesses.<slug>.command` | `must not be empty` |
| H4 | | `harnesses.<slug>.providers` | `provider %q must match [a-z0-9_]+` |
| F1 | favourites | `favourites.pins` | `invalid route key %q` |
| F2 | | `favourites.pins` | `duplicate pin %q` |
| R1 | routes.disabled | `routes.disabled` | `provider %q must match [a-z0-9_]+` |
| R2 | | `routes.disabled.<provider>` | `invalid route %q` (must match `model_id "@" reasoning`) |
| R3 | | `routes.disabled.<provider>` | `duplicate route %q` |
| G1 | groups | `groups.<slug>` | `slug must match [a-z0-9_]+` |
| G2 | | `groups.<slug>.benchmarks` | `must not be empty` |
| G3 | | `groups.<slug>.benchmarks` | `benchmark name must not be empty` |
| G4 | | `groups.<slug>.benchmarks` | `duplicate benchmark %q` |
| U1 | gui | `gui.layout` | `must be "carousel" or "list"` |
| U2 | | `gui.weight_control` | `must be "step", "bar" or "slider"` |
| U3 | | `gui.holds` | `must be 3, 5 or 10` |
| U4 | | `gui.shortcut` | `must be "alt+space", "ctrl+space" or "cmd+shift+m"` |
| U5 | | `gui.auto_update_frequency` | `must be "hourly", "daily", "weekly" or "monthly"` |

Unknown keys inside owned sections keep the existing `UnmarshalKey` undecoded error (`config: invalid value for <section>.<key>: <key>`).

## 6. Test fixtures

| File | Requirement |
|---|---|
| `internal/config/gui_test.go` | Table tests: every §5 error (exact string asserted); defaults-on-absent for all six accessors (`LoadGUI` == `DefaultGUIConfig()`); set→load round-trip per section; delete idempotence. **Golden round-trip (REQUIRED):** a fixture TOML containing all six sections, unknown keys in non-owned sections (e.g. `[scoring]`, an unrecognised `[future]` table), and existing `[usage]`/`[providers.*]` content is loaded, `MarshalTOML`'d, reloaded, and re-marshalled — the two marshal outputs are byte-identical and TOML-decode to semantically equal documents with zero keys lost (SPEC §2.6). |
| `internal/config/write_test.go` | `AtomicWriteFile`: creates missing parent dirs; resulting file mode 0600; overwrite replaces content; failure mid-write (unwritable dir) leaves an existing destination byte-identical and no temp litter. |
| `pkg/whichmodel` (existing tests) | `config set` tests pass unchanged after the re-point — no new fixtures. |

## 7. External symbols referenced

| Symbol | Source | Used by |
|---|---|---|
| `Config.UnmarshalKey`, `Config.MarshalTOML`, `setKey`, `ConfigError` | `internal/config` (existing) | all accessors/setters |
| `pick.CategoryNames` | `internal/pick/axes.go` | callers only (B03/B05) — passed as `categories`, never imported here |
| Route-key grammar | D00 CONTRACTS §1 | re-declared regexp in `gui.go` (SPEC Decisions) |
