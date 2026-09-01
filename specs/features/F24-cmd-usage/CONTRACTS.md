---
kind: feature-contracts
version: "1.0"
feature: F24-cmd-usage
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F24 — cmd-usage: Contracts

## 1. Owned files

- `pkg/whichmodel/usage_cmd.go` — cobra command, `//go:build !nousage` (command wiring per F22 contract; one file per command, self-registering `init()`).
- `pkg/whichmodel/usage.go` — pure logic: arg validation, fetch wiring, report assembly, text renderer. Compiles under both build tags (F21 stubs).
- Tests: `pkg/whichmodel/usage_cmd_test.go`, `pkg/whichmodel/usage_args_test.go`, `pkg/whichmodel/usage_fetch_test.go`, `pkg/whichmodel/usage_identity_test.go`, `pkg/whichmodel/usage_source_test.go`, `pkg/whichmodel/usage_exit_test.go`, `pkg/whichmodel/usage_json_test.go`, `pkg/whichmodel/usage_text_test.go`.

No config keys, no state files are owned by F24 (it reads `[usage] enabled` and `[providers.<id>].enabled`; it never writes).

## 2. Exported API (`pkg/whichmodel`)

```go
// UsageArgs is the fully-parsed, validated command input.
type UsageArgs struct {
    Providers    []string      // registry IDs, request order; empty when All
    All          bool          // --all
    Source       usage.Source  // "" = auto fallback chain; else one of oauth|api|cli|web|local|cache
    BandAtOrAbove string        // --band-at-or-above; "" = no band filter
    MaxAge       time.Duration // 0 = descriptor/config TTL
    ForceRefresh bool          // --refresh-usage
    Timeout      time.Duration // 0 = DefaultTimeoutSec
    Offline      bool          // --offline
    ShowIdentity bool          // --show-identity
    JSON         bool          // --json
    ConfigPath   string        // "" = resolved config
}

// UsageReport is the --json document root (F24 CONTRACTS §6).
type UsageReport struct {
    SchemaVersion   string                  `json:"schema_version"`              // "2.0"
    UsageEnabled    bool                    `json:"usage_enabled"`               // always true when emitted
    Snapshots       []usage.Snapshot        `json:"snapshots"`                   // request order
    LastVerified    map[string]time.Time    `json:"last_verified,omitempty"`     // provider → last successful live fetch
}

// RunUsage executes the whole command: disabled check → arg validation →
// registry lookup → FetchAll → exit classification → render (text or JSON).
// The returned error is nil, or an exit error (F24 CONTRACTS §8.1).
func RunUsage(args UsageArgs, stdout, stderr io.Writer) error

// FormatUsageText renders the text report (SPEC §2.7, §2.8).
// Pure function: same (report, showIdentity) → same bytes.
func FormatUsageText(report *UsageReport, showIdentity bool) string

// FetchAllOptions is the exact consumption shape passed to F14 (F24 CONTRACTS §8.2).
type FetchAllOptions struct {
    Providers      []string
    Source         usage.Source // "" = auto
    ForceRefresh   bool
    MaxAge         time.Duration
    Timeout        time.Duration
    Offline        bool
    IncludeIdentity bool
}
```

## 3. Flags owned

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--all` | bool | `false` | Report every enabled provider instead of requiring positional names; mutually exclusive with positionals (exit 2) |
| `--source` | string | `""` | One of `oauth\|api\|cli\|web\|local\|cache`; `""` = auto fallback chain; explicit `"auto"` rejected (exit 2, SPEC §2.4). Command-local flag (F22's root flag set has no `--source`; annex-d §1.2's global `--source` is superseded) |
| `--band-at-or-above` | string | `""` | Keep only snapshots whose maximum known window pressure is in this configured band or a later one; hard-gated snapshots count as above every band; unknown band names are exit 2 |

Global flags (F22-owned, consumed by wiring `Global` into `UsageArgs`): `--json`, `--max-age` (`Global.MaxAge`), `--refresh-usage` (`Global.RefreshUsage`), `--show-identity` (`Global.ShowIdentity`), `--offline` (`Global.Offline`), `--no-usage` (`Global.NoUsage`), `--timeout` (`Global.Timeout`), `--config` (`Global.ConfigPath`).

## 4. Config keys read (F01-owned)

| Key | Read via | Used for |
|---|---|---|
| `usage.enabled` | `cfg.UnmarshalKey("usage.enabled", &v)` | L1 disabled refusal (SPEC §2.12) |
| `providers.<id>.enabled` | `cfg.UnmarshalKey("providers.<id>.enabled", &v)` | `--all` expansion (SPEC §2.3) |
| `bands` | `cfg.UnmarshalKey("bands", &raw)` + F19 `band.FromTOML` | Resolve and validate `--band-at-or-above` against the configured ordered tier ladder |

## 5. Error codes and exit codes

Command-level code strings on the failure line (`which-model usage: [<code>] <message>`, rendered by F22 via `output.WriteFailure`): `arguments` (all exit-2 argument errors — `UsageError`), `usage_disabled` (L0/L1 refusal — `CodedError`). Provider failures use the canonical `Failure.Code` values (global CONTRACTS §1.6) and appear inline in output or on stderr, never as new codes.

F24 RunE returns: `whichmodel.UsageError{Message}` for argument errors; `whichmodel.CodedError{Code, Message}` for everything else. Exit mapping is F22's (`ExitCodeFor`): `usage_disabled` → 2; auth-class codes (`unauthorized`, `login_required`, `expired_credential`, `credential_file`, `credential_json`, `unsafe_credential`, `access_denied`, `device_expired`, `cookie_unavailable`, `signing_failed`) → 5; any other code → 1.

| Exit | Condition |
|---|---|
| 0 | ≥1 requested provider succeeded (failures inline) |
| 1 | all providers failed, no auth-class failure among them |
| 2 | argument error; usage disabled (L0/L1) |
| 5 | all providers failed, ≥1 auth-class failure among them |

## 6. JSON shape (`which-model usage --json`)

```json
{
  "schema_version": "2.0",
  "usage_enabled": true,
  "snapshots": [],
  "last_verified": { "claude": "2026-08-07T17:03:11Z" }
}
```

- `snapshots`: one entry per requested provider in normal mode; with `--band-at-or-above`, only providers whose maximum known window pressure is in the requested configured tier or a later tier remain. A hard-gated provider always remains; unknown pressure does not. A failed provider carries `error: {"code": "...", "message": "..."}` (canonical `Failure`) and still participates in normal all-failed exit classification before filtering.
- `last_verified`: present only when F14 returned at least one included provider verification timestamp; keys are provider ids, values RFC3339.
- `schema_version` is `"2.0"` per global CONTRACTS §6; `usage_disabled_reason` never appears (usage refuses with exit 2 when disabled).
- On nonzero exit in text mode, stdout is empty (SPEC §2.13); in `--json` mode, stdout carries F22's error document `{"schema_version": "2.0", "error": {"code": ..., "message": ...}}` (rendered by F22, never by F24) instead of the report.

## 7. Text layout spec

Per provider, in request order, blank line between blocks:

```
<DisplayName> usage allowance
- <label>: <detail1>; <detail2>; ...
```

- `<DisplayName>` = registry `Descriptor.Name`, fallback provider id (Decision D-9).
- Detail order per window: `used` → `<n>% used`; `available` → `<n>% available` (only when `UsedPercent` set and `Remaining` nil); `remaining` → `<n> remaining`; `total` → `<n> total` (from `Limit`); `unlimited` (single detail, suppresses all percent/count); reset detail always last.
- Reset detail: `ResetsAt` set → `resets <RFC3339>`; else `ResetHint` non-empty → hint verbatim (prefixed `resets ` only if it does not already start with "resets"); else nothing.
- Numbers: `strconv.FormatFloat(v, 'f', -1, 64)` (trailing `.0` stripped).
- `--show-identity` only: append `- account: <account>` as the last line of the provider block.
- Example (golden, from annex-d §2.1):

```
Claude usage allowance
- five hour: 25% used; 75% available; resets 2026-08-07T18:00:00Z
- seven day: 41% used; 59% available

Codex usage allowance
- primary window: 12% used; 88% available; resets 2026-08-08T00:00:00Z
- credits: 340 remaining
```

## 8. Imported contracts (consumed upstream)

### 8.1 F22 `pkg/whichmodel` command wiring + exit mapping (pinned; canonical owner: `specs/features/F22-cli-skeleton/CONTRACTS.md`)

```go
package whichmodel

type GlobalFlags struct {
    JSON, Text, Quiet, NoColor, Offline, RefreshUsage, RefreshBenchmarks,
    RefreshScores, Refresh, NoUsage, ShowIdentity, Schema, Version bool
    MaxAge, Timeout time.Duration // Timeout default 10s
    Verbose int
    ConfigPath, Normalizer, Aggregator string
}
var Global GlobalFlags

func (g *GlobalFlags) Bind(cmd *cobra.Command) error
func (g *GlobalFlags) Normalize() error
func (g *GlobalFlags) Validate() error

type UsageError struct{ Message string } // exit 2, code "arguments"
type CodedError struct{ Code, Message string } // code→exit via global §1.6 table; unknown code → 1
func ExitCodeFor(err error) int
func CodeFor(err error) string
func RegisterExitCode(code string, exit int) // concurrency-safe, additive; called from F24's init() only if needed

func Execute() int
func ExecuteArgs(args []string) int
func NewRootCmd() *cobra.Command // Use fixed "which-model"

func RegisterSchema(cmdPath string, doc map[string]any)
func SchemaIndex() []string

// unexported — F24 calls it from init() only:
func register(factory func() *cobra.Command)
func registeredCommands() []*cobra.Command
```

- `pkg/whichmodel/usage_cmd.go` = `func init() { register(NewUsageCmd) }` + `func NewUsageCmd() *cobra.Command`. F24 never calls `AddCommand` and never calls `os.Exit`; F22's `ExecuteArgs` maps the returned error to the process exit via `ExitCodeFor` and renders exactly one failure line on stderr via F03 `output.WriteFailure(w, "usage", code, message)`; in `--json` mode F22 instead writes `{"schema_version": "2.0", "error": {"code": ..., "message": ...}}` to stdout. F24 RunE returns errors only and never prints failure lines or JSON error documents itself (SPEC Decision D-10).
- F24 init() registers no extra exit codes (its codes `arguments`→2, `usage_disabled`→2, auth-class codes→5, unknown→1 are all covered by F22's global table).

### 8.2 F14 `internal/usage/fetch` (canonical owner: `specs/features/F14-usage-fetch/CONTRACTS.md`)

```go
package fetch

type FetchAllOptions struct {
    Providers       []string
    All             bool   // expand to every enabled provider
    Source          usage.Source // "" = auto
    ForceRefresh    bool
    MaxAge          time.Duration
    Timeout         time.Duration
    Offline         bool
    IncludeIdentity bool
}

type FetchResult struct {
    Snapshots   []usage.Snapshot
    LastVerified map[string]time.Time // provider → last successful live verification
}

func FetchAll(ctx context.Context, opts FetchAllOptions) (*FetchResult, error)
```

Expected file: `internal/usage/fetch/fetch.go`. F24 passes its `FetchAllOptions` verbatim (F24 CONTRACTS §2) and renders `FetchResult`; F14 owns cache-TTL, fan-out, and partial-failure semantics.

### 8.3 F11 `internal/usage` registry (canonical owner: `specs/features/F11-usage-types/CONTRACTS.md`)

```go
type Descriptor struct {
    ID          string        // e.g. "claude"
    Name        string        // display name, e.g. "Claude"
    Kind        Kind
    AuthSources []Source      // ordered fallback chain; valid --source values for this provider
}

func All() []Descriptor            // registry order
func Get(id string) (Descriptor, bool)
```

Expected file: `internal/usage/registry.go`. Used for arg validation (SPEC §2.6) and text header display names (Decision D-9).

### 8.4 F01 `internal/config`

- `func Load(path string) (*config.Config, error)` (F01 canonical owner) and `func (c *Config) UnmarshalKey(key string, out any) error` (pinned by the F01/F30/F19/F20 ownership decision — F24 uses it for `usage.enabled` and `providers.<id>.enabled`).
- `func StateDir() (string, error)` — not used by F24 (documented for the cmd family).

## 9. Security invariants (this feature)

- Never render `Snapshot.Account` without `--show-identity` (SPEC §2.9; global SPEC §6.7).
- Never render credential/token material under any flag; every output path is canary-tested (global SPEC §6.5).
- stdout carries report data only; all diagnostics to stderr (annex-d §1.3).
- No network, credential, or cache access is performed by F24 itself — everything goes through F14 (with F12/F13 underneath).
