---
kind: feature-contracts
feature: F01-config
version: "1.0"
project: which-model
---

# F01 — config: CONTRACTS

Package `internal/config` (Layer 0). Imports: Go stdlib, `github.com/BurntSushi/toml`, `github.com/shopspring/decimal`. MUST NOT import anything in `internal/` (`specs/global/CONTRACTS.md` §8). Files: `internal/config/usage.go`, `types.go`, `unmarshal.go`, `load.go`, `env.go`, `paths.go`, `discover.go`, `validate.go`, `marshal.go`. Feature `depends_on: —`, blocks F19, F21, F22, F30 (`specs/DEPENDENCY-GRAPH.md` §2).

## 1. Exported API

### 1.1 Usage toggle (`internal/config/usage.go`)

The canonical type, copied verbatim from `specs/global/CONTRACTS.md` §5.1:

```go
type UsageEnabled string

const (
    UsageAuto  UsageEnabled = "auto"  // enabled iff ≥1 provider enabled
    UsageTrue  UsageEnabled = "true"
    UsageFalse UsageEnabled = "false"
)

// F01 additions (same package, file internal/config/usage.go):
// Parse accepts exactly "auto", "true", "false" (case-sensitive); anything
// else is a validation error (KindInvalidValue, Key "usage.enabled").
func ParseUsageEnabled(s string) (UsageEnabled, error)

// UnmarshalTOML accepts a TOML boolean (true/false) or the string "auto";
// any other TOML value (or string) is an error. BurntSushi/toml calls this
// for `[usage] enabled = …` because UsageEnabled implements Unmarshaler.
func (u *UsageEnabled) UnmarshalTOML(v interface{}) error

func ParseUsageBackend(s string) (UsageBackend, error)
func (b *UsageBackend) UnmarshalTOML(v interface{}) error
```

### 1.2 Config types (`internal/config/types.go`)

```go
type Config struct {
    Usage     UsageConfig
    Providers map[string]ProviderConfig
    // unexported: raw merged TOML document + WHICH_MODEL_* overlay
}

type UsageBackend string

const (
    UsageBackendOff      UsageBackend = "off"
    UsageBackendNative   UsageBackend = "native"
    UsageBackendCodexBar UsageBackend = "codexbar"
)

type UsageConfig struct {
    Enabled UsageEnabled // default UsageAuto
    Backend UsageBackend // default UsageBackendOff
}

type ProviderConfig struct {
    Enabled               bool
    Priority              int
    Weight                decimal.Decimal // default 1.0; 0/absent → 1.0, negative → invalid
    CacheTTL              time.Duration
    SourcePreference      []string // NOT env-addressable (annex-d §4.4)
    CredentialPath        string
    TrustedFallbackOrigin string
}

func Default() *Config // Usage.Enabled = UsageAuto; Usage.Backend = UsageBackendOff; Providers = empty map
```

### 1.3 Errors (`internal/config/types.go`)

```go
type ErrorKind int

const (
    KindNotFound ErrorKind = iota // explicitly requested file does not exist
    KindUnreadable                // file exists but cannot be read
    KindInvalidTOML               // TOML does not parse
    KindInvalidValue              // unknown key / bad value / bad env override
)

type ConfigError struct {
    Kind ErrorKind
    Path string // file path (load errors) or "" (value errors)
    Key  string // dotted config key or env var name, when applicable
    Err  error  // wrapped cause (may be nil)
}

func (e *ConfigError) Error() string
func (e *ConfigError) Unwrap() error
func (e *ConfigError) ExitCode() int // always 2 (global SPEC §5: argument/config error)
```

### 1.4 Loading (`internal/config/load.go`)

```go
type LoadOptions struct {
    Path   string            // --config value (flag declared by F22); "" = discover
    Getenv func(string) string // nil = os.Getenv
    CWD    string            // "" = os.Getwd
    Home   string            // "" = os.UserHomeDir
    GOOS   string            // "" = runtime.GOOS
}

// Load resolves in order: Path → $WHICH_MODEL_CONFIG → project walk → user
// file → Default(), then WHICH_MODEL_* overrides, then Validate.
func Load(opts LoadOptions) (*Config, error)

// LoadFile decodes one file over Default(), validates, and returns.
// No discovery, no env overrides. Missing file → KindNotFound.
func LoadFile(path string) (*Config, error)
```

### 1.5 Discovery and paths (`internal/config/discover.go`, `internal/config/paths.go`)

```go
// Walk from cwd upward: dir/.which-model/config.toml (first hit wins),
// bounded at the nearest git root (inclusive), then $HOME (exclusive).
func ProjectConfigFile(cwd, home string) (string, bool)

// User config file path: $XDG_CONFIG_HOME/which-model/config.toml
// (default ~/.config/…) on non-darwin; ~/Library/Application Support/
// which-model/config.toml on darwin. XDG_* ignored on darwin.
func UserConfigFile(goos, home string, getenv func(string) string) string

type Paths struct {
    UserConfigFile string // user-layer config.toml
    ConfigDir      string
    CacheDir       string
    StateDir       string // darwin: ConfigDir + "/state"
}

// annex-d §4.5 table; goos == "darwin" → macOS column unconditionally.
func ResolvePaths(goos, home string, getenv func(string) string) Paths
```

### 1.6 Env overrides (`internal/config/env.go`)

```go
// Applies WHICH_MODEL_* overrides. environ lists the process environment
// for enumeration (nil = os.Environ); getenv resolves values (nil =
// os.Getenv). Every var except WHICH_MODEL_CONFIG must match a key in the
// closed env-key vocabulary (see §3); no match → KindInvalidValue naming
// the var. F01-typed keys (usage.enabled, providers.<id>.{enabled,
// priority, weight, cache_ttl, credential_path, trusted_fallback_origin})
// set the typed fields eagerly; all other matches are stored as
// dotted-key → raw value (c.env) and applied by UnmarshalKey. Parse
// failures → KindInvalidValue naming the var. WHICH_MODEL_CONFIG is a path
// override consumed by Load, never stored.
func ApplyEnv(c *Config, getenv func(string) string, environ []string) error
```

Env-name → dotted-key resolution: lowercase; find the LONGEST suffix of
the name that equals a vocabulary key (underscores included, e.g. `DEFAULT_PROFILE`,
`WARN_ON_STALE_SCORES`); the remainder is the section path (dots from `_`,
`providers.<id>` restored to kebab-case). Exported as the table `EnvKeys`
(lowercased key names; used by the vocabulary test).

### 1.7 Generic accessor (`internal/config/unmarshal.go`)

```go
// Decodes the merged TOML subtree at the dotted key into out (struct
// pointer, pre-populated with the caller's defaults), then overlays stored
// env overrides whose dotted path falls under the subtree even when no file
// declares that table (matched by toml tag path; scalar kinds only: string,
// bool, int, time.Duration, encoding.TextUnmarshaler, or a pointer to any of
// these — nil pointers are allocated, so callers keep unset-vs-zero
// semantics), and rejects unknown keys inside the subtree
// (KindInvalidValue). A missing TOML key with no matching env override leaves
// out untouched and returns nil. A present key must address a TOML table.
func (c *Config) UnmarshalKey(key string, out any) error

// Decodes one TOML file as a layer into c: parse, merge into the raw
// document, typed-decode [usage]/[providers]. Missing → KindNotFound,
// unreadable → KindUnreadable, unparseable → KindInvalidTOML, unknown
// owned key → KindInvalidValue. Used by Load/LoadFile and by tests.
func (c *Config) DecodeFile(path string) error
```

### 1.8 Resolved rendering (`internal/config/marshal.go`)

```go
// Renders the fully resolved config for `config show` (annex-d §2.7):
// [usage] + [providers.<id>] from typed values (env applied, weights
// normalized), generic sections from the merged file document, and env
// overrides merged into their sections (dotted keys). UsageAuto → "auto",
// UsageTrue/False → TOML bools; decimals/durations → TOML strings; env
// values rendered by documented key type. Section order: usage, scoring,
// strategy, bands, catalog, output, providers (annex-d §4.2); providers
// sorted by id. Generic sections appear only when present in a file layer
// or via env.
func (c *Config) MarshalTOML() ([]byte, error)
```

### 1.9 Validation (`internal/config/validate.go`)

```go
// Validates usage.enabled ∈ {auto,true,false} and usage.backend ∈ {off,native,codexbar};
// provider ids non-empty; provider weight ≥ 0 (0 normalized to 1.0);
// provider cache_ttl ≥ 0. Returns *ConfigError (KindInvalidValue) on first violation.
func (c *Config) Validate() error
```

## 2. Config keys owned

| Key | Type | Default | Notes |
|---|---|---|---|
| `usage.enabled` | `UsageEnabled` | `"auto"` | TOML bool or string `"auto"`; three-state; resolution → F21 (README §6.1, annex-d §4.2) |
| `usage.backend` | `UsageBackend` | `"off"` | `"off"` disables usage; `"native"` selects the Claude/Codex/Copilot adapters; `"codexbar"` selects the CodexBar CLI |
| `providers.<id>.enabled` | bool | `false` | default-deny (README §6.2); `enabled` is the ONLY key F21/F15-F17 read for gating |
| `providers.<id>.priority` | int | `0` | priority-strategy ordering → F20 |
| `providers.<id>.weight` | `decimal.Decimal` | `1.0` | FinalScore multiplier → F19/F20 (annex-d §4.2) |
| `providers.<id>.cache_ttl` | `time.Duration` | unset | usage snapshot TTL → F15-F17/F13 |
| `providers.<id>.source_preference` | `[]string` | `[]` | ordered `AuthSource` override → F15-F17 (annex-d §4.2) |
| `providers.<id>.credential_path` | string | `""` | override provider-native path → F12/F15-F17 |
| `providers.<id>.trusted_fallback_origin` | string | `""` | pre-fill only, never auto-trust → F16 (annex-d §2.2) |

Unknown keys under `usage.*` or `providers.<id>.*` → `KindInvalidValue` (strict allow-list, annex-d §1.4). `<id>` is any non-empty table name; registry membership is NOT validated by F01 (→ F15-F17, Annex A).

## 3. Env vars owned

| Env var | Effect |
|---|---|
| `WHICH_MODEL_CONFIG` | config file path override; consumed by `Load` between `--config` and discovery; missing file → `KindNotFound` |
| `WHICH_MODEL_USAGE_ENABLED` | `usage.enabled` (`"auto"`/`"true"`/`"false"`) |
| `WHICH_MODEL_USAGE_BACKEND` | `usage.backend` (`"off"`/`"native"`/`"codexbar"`) |
| `WHICH_MODEL_PROVIDERS_<ID>_ENABLED` | `providers.<id>.enabled` (bool) |
| `WHICH_MODEL_PROVIDERS_<ID>_PRIORITY` | `providers.<id>.priority` (int) |
| `WHICH_MODEL_PROVIDERS_<ID>_WEIGHT` | `providers.<id>.weight` (decimal) |
| `WHICH_MODEL_PROVIDERS_<ID>_CACHE_TTL` | `providers.<id>.cache_ttl` (duration) |
| `WHICH_MODEL_PROVIDERS_<ID>_CREDENTIAL_PATH` | `providers.<id>.credential_path` (string) |
| `WHICH_MODEL_PROVIDERS_<ID>_TRUSTED_FALLBACK_ORIGIN` | `providers.<id>.trusted_fallback_origin` (string) |

`<ID>` = provider id uppercased with `-` → `_`. Unknown provider keys (`WHICH_MODEL_PROVIDERS_<ID>_<other>`) → `KindInvalidValue`.

**Closed env-key vocabulary** (D14): a `WHICH_MODEL_*` var is valid iff its longest-suffix match lands on one of these lowercased key names AND the resolved section owns that key per the table (arrays excluded — annex-d §4.4):

| Key names | Sections |
|---|---|
| `enabled`, `priority`, `weight`, `cache_ttl`, `credential_path`, `trusted_fallback_origin` | `providers.<id>` (`source_preference` array excluded) |
| `enabled` | `usage`, `catalog.publish` |
| `default`, `default_profile`, `tier1_share`, `tier2_share` | `strategy` |
| `direction`, `gate_above_used_percent` | `bands` (`tier` array excluded) |
| `normalizer`, `aggregator` | `scoring` |
| `raw_csv_path`, `scores_csv_path`, `provider_config_path`, `benchmark_config_path`, `warn_on_stale_scores` | `catalog` (`cache_ttl` too) |
| `schedule`, `timezone`, `mode`, `auto_merge`, `merge_method`, `commit_message`, `pr_title`, `run_tests` | `catalog.publish` (`branches`, `pr_labels` arrays excluded) |
| `color`, `timestamps`, `identity_default` | `output` |

Any other `WHICH_MODEL_*` var — no suffix match, an unknown section, or a key the resolved section does not own (e.g. `WHICH_MODEL_USAGE_DEFAULT`: `default` exists but `strategy` owns it) — → `KindInvalidValue` at `ApplyEnv` time (eager). Matches are stored as `dotted-key → raw value` and applied by `UnmarshalKey` when the owning feature decodes its section (e.g. `WHICH_MODEL_BANDS_GATE_ABOVE_USED_PERCENT` → `bands.gate_above_used_percent`; `WHICH_MODEL_CATALOG_PUBLISH_SCHEDULE` → `catalog.publish.schedule`). Arrays are never env-addressable. Adding a config key later updates this table + the vocabulary test.

## 4. Flags owned

None. `--config <path>` is a global flag declared and wired by F22-cli-skeleton (annex-d §1.2); F22 passes its value as `LoadOptions.Path`. The `--no-usage` flag is F21's (F01 only parses `usage.enabled`).

## 5. Error codes added

No new `Failure.Code` values. `ConfigError.ExitCode() == 2` for every kind; F22 maps `*config.ConfigError` → exit 2 ("Argument/config error", `specs/global/SPEC.md` §5; deterministic config errors, annex-d §3.4). Kind taxonomy: `KindNotFound` (missing explicit file), `KindUnreadable`, `KindInvalidTOML`, `KindInvalidValue` — the missing-vs-malformed distinction callers rely on.

## 6. JSON shapes emitted

None. `which-model config show --json` (including its `_sources` map) is F22's composition over F01 primitives (annex-d §4.1).

## 7. Consumers

- F21-usage-toggle: `cfg.Usage.Enabled` (`UsageEnabled`, `specs/global/CONTRACTS.md` §5.1) + enumeration of `cfg.Providers` with `Enabled == true` for `auto` resolution (README §6.1); F21 resolves, F01 never does.
- F22-cli-skeleton: `Load`, `LoadFile`, `Validate`, `ResolvePaths`, `ConfigError`/`ExitCode`; `--config` → `LoadOptions.Path`; `config show` renders via `MarshalTOML` (annex-d §2.7); `config show --json` derives from the same render (toml → map, F22 composition; `_sources` is F22's).
- F19-bands / F20-strategies / F09-scoring / F23-cmd-catalog / F30-publishing: `cfg.UnmarshalKey("bands"|"strategy"|"scoring"|"catalog"|"catalog.publish", &ownStruct)` with own defaults + semantic validation.
- F30-publishing: `Load(cfg)` + `UnmarshalKey("catalog.publish", …)` per DECISION B; F30 SPEC owns the full `[catalog.publish]` key table (annex-b §8).

### Environment rendering correction (#167)

Generic environment overrides retain the owning field type when rendered or saved:
`strategy.tier1_share` and `tier2_share` are integers (including 0/1);
`catalog.warn_on_stale_scores`, `catalog.publish.enabled/auto_merge/run_tests`, and
`output.identity_default` are booleans (including ParseBool 0/1 spelling).
All remaining generic overrides are strings, including decimal gate values,
durations, and titles such as "true", "0", and "001". Invalid booleans or integers
return `ConfigError{Kind: KindInvalidValue}` naming the dotted key. This supersedes
the lossy bool-first inference in D16; rendering never mutates the source config.
