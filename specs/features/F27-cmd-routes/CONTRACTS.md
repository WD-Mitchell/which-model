---
kind: feature-contracts
version: "1.0"
feature: F27-cmd-routes
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F27 — cmd-routes: Contracts

## 1. Owned files

- `pkg/whichmodel/routes_cmd.go` — `routes` command with the five subcommands `list|add|remove|refresh|verify`, NO build tag (registered in all builds; SPEC §2.6). `func init() { register(NewRoutesCmd) }` + `func NewRoutesCmd() *cobra.Command`.
- `pkg/whichmodel/routes.go` — logic: add/remove/list/refresh/verify implementations, all reading/writing routes via F18 seams.
- Tests: `pkg/whichmodel/routes_cmd_test.go`, `routes_add_test.go`, `routes_remove_test.go`, `routes_list_test.go`, `routes_refresh_test.go`, `routes_verify_test.go`, `routes_json_test.go`.

No config keys, no flags beyond the tables below are owned; F27 never writes routes.json directly (F18 `SaveRoutes` is the only writer) and owns no state files of its own.

## 2. Exported API (`pkg/whichmodel`)

```go
// RouteAddArgs / RouteRemoveArgs / RouteListArgs / RouteRefreshArgs /
// RouteVerifyArgs are the validated per-subcommand inputs.
type RouteAddArgs struct {
    Provider   string   // registry id
    ModelID    string   // provider-side model id
    Model      string   // scored catalog model name
    Reasoning  string   // default "default"
    Windows    []string // --window, repeatable
    ConfigPath string
}
type RouteRemoveArgs struct {
    Provider, ModelID, ConfigPath string
}
type RouteListArgs struct {
    Provider   string // "" = all
    JSON       bool
    ConfigPath string
}
type RouteRefreshArgs struct {
    Auto       string // "" = none
    ConfigPath string
}
type RouteVerifyArgs struct {
    JSON       bool
    ConfigPath string
}

// RouteList is the list --json document root.
type RouteList struct {
    SchemaVersion string          `json:"schema_version"` // "2.0"
    Routes        []routing.Route `json:"routes"`         // canonical routing.Route JSON tags
}

// VerifyReport is the verify --json document root (SPEC §2.7).
type VerifyReport struct {
    SchemaVersion        string         `json:"schema_version"`      // "2.0"
    StaleRoutes          []string       `json:"stale_routes"`        // "<provider>:<model-id>"
    Unrouted             []ScoreRef     `json:"unrouted"`            // score rows without routes
    ProvenanceCounts     map[string]int `json:"provenance_counts"`   // user_declared|provider_live|models_dev
    ScoresSHA256Matches  bool           `json:"scores_sha256_matches"`
}
type ScoreRef struct {
    Model     string `json:"model"`
    Reasoning string `json:"reasoning"`
}

func RunRouteAdd(args RouteAddArgs, stdout, stderr io.Writer) error
func RunRouteRemove(args RouteRemoveArgs, stdout, stderr io.Writer) error
func RunRouteList(args RouteListArgs, stdout, stderr io.Writer) error
func RunRouteRefresh(args RouteRefreshArgs, stdout, stderr io.Writer) error
func RunRouteVerify(args RouteVerifyArgs, stdout, stderr io.Writer) error
```

## 3. Flags owned

| Subcommand | Flag | Type | Default | Meaning |
|---|---|---|---|---|
| `add` | `--provider` | string | `""` | registry id; required, validated |
| `add` | `--model-id` | string | `""` | required, non-empty |
| `add` | `--model` | string | `""` | required, non-empty |
| `add` | `--reasoning` | string | `"default"` | score-row reasoning key |
| `add` | `--window` | stringSlice | `[]` | repeatable window id |
| `remove` | `--provider` | string | `""` | required |
| `remove` | `--model-id` | string | `""` | required |
| `list` | `--provider` | string | `""` | filter; unknown → exit 2 |
| `refresh` | `--auto` | string | `""` | fuzzy model name → adds user_declared route |
| `verify` | — | — | — | no own flags |

Consumed globals: `--json` (`Global.JSON`), `--config` (`Global.ConfigPath`), `--no-usage` (`Global.NoUsage` — refresh warning only, SPEC §2.6).

## 4. Exit codes

No new registrations (F27's codes are all covered by F22's table; `no_route`, `stale_routes`, `runtime` → unknown → 1; `arguments` → 2 via `UsageError`).

| Subcommand | Exit | Condition |
|---|---|---|
| `add` | 0 | route written |
| `add` | 2 | unknown provider; empty `--model-id`/`--model`; duplicate route |
| `add` | 1 | IO error on load/save |
| `remove` | 0 | removed (or was removable) |
| `remove` | 1 | no such route (`no_route`); IO error |
| `remove` | 2 | missing/invalid flags |
| `list` | 0 | table/JSON emitted (missing routes.json = empty) |
| `list` | 2 | unknown `--provider` |
| `list` | 1 | IO error |
| `refresh` | 0 | produced + saved (or no-op); usage-disabled warning is not an error |
| `refresh` | 2 | `--auto` zero/ambiguous match; bad flags |
| `refresh` | 1 | IO error |
| `verify` | 0 | clean (report still emitted) |
| `verify` | 1 | ≥1 stale route (`stale_routes`, report on stdout via `ReportedError`); IO error |
| `verify` | 2 | bad flags |

## 5. JSON shapes

`list --json` (F22 envelope applies — this document IS the envelope):
```json
{
  "schema_version": "2.0",
  "routes": [
    {"provider": "claude", "model_id": "claude-sonnet-4-5", "model": "claude-sonnet-4-5", "reasoning": "default", "windows": ["5h", "7d"], "provenance": "user_declared"}
  ]
}
```
(tags per canonical `routing.Route` — F18 is the format owner; F27 echoes it.)

`verify --json`:
```json
{
  "schema_version": "2.0",
  "stale_routes": ["codex:gpt-5-codex"],
  "unrouted": [{"model": "gpt-5-codex", "reasoning": "default"}],
  "provenance_counts": {"user_declared": 2, "provider_live": 1, "models_dev": 0},
  "scores_sha256_matches": false
}
```
Arrays always `[]` (never `null`); `provenance_counts` always contains all three keys.

## 6. Text layout spec

`list` (text): `text/tabwriter`, padding 2, header `provider  model_id  model  reasoning  windows  provenance`; `windows` comma-joined or `-`; one row per route in file order.

`verify` (text, stdout):
```
stale route <provider>:<model-id> (<model>/<reasoning>)
```
one line per stale route; stderr: `routes: <n> total (<x> user_declared, <y> provider_live, <z> models_dev)`.

## 7. Imported contracts (consumed upstream)

### 7.1 F18 `internal/usage/routing` (canonical owner: `specs/features/F18-usage-routing/CONTRACTS.md`)

```go
package routing

type Route struct {
    Provider   string   `json:"provider"`
    ModelID    string   `json:"model_id"`
    Model      string   `json:"model"`
    Reasoning  string   `json:"reasoning"`
    Windows    []string `json:"windows"`
    Provenance string   `json:"provenance"` // provider_live|models_dev|user_declared
}

func LoadRoutes(path string) ([]Route, error)   // missing file → empty list, nil error
func SaveRoutes(path string, routes []Route) error
func ProduceRoutes(cfg *config.Config) ([]Route, error) // includes merge: user_declared wins
func RoutesPath(cfg *config.Config) (string, error)
func ScoresSHA256(cfg *config.Config) (string, error) // hash of current scores CSV
```

F27's seams: `loadRoutesFunc`, `saveRoutesFunc`, `produceRoutesFunc`, `routesPathFunc` (defaults = the F18 funcs; injectable in tests). F27 persists exactly what ProduceRoutes returns and never re-implements merge.

### 7.2 F06 `internal/catalog/csvstore` (canonical owner: F06 CONTRACTS)

```go
package csvstore

type ScoreRow struct { Model, Reasoning string; /* score fields */ }
func ReadScores(path string) ([]ScoreRow, error)
```

F27 consumes `ReadScores` (via seam `readScoresFunc`) for verify; the CSV path comes from `cfg.UnmarshalKey("catalog.scores_csv_path", &p)`.

### 7.3 F11 `internal/usage` registry

`func Get(id string) (Descriptor, bool)` — `add`/`list --provider` validation.

### 7.4 F01 `internal/config`

`func Load(path string) (*config.Config, error)`, `func (c *Config) UnmarshalKey(key string, out any) error`, `func (c *Config) StateDir() (string, error)` — routes path resolution.

### 7.5 F22 `pkg/whichmodel` (pinned; canonical owner: `specs/features/F22-cli-skeleton/CONTRACTS.md`)

`GlobalFlags`/`Global`, `UsageError`, `CodedError`, `ReportedError{Err error}` (deliverable already on stdout: stderr failure line + exit code still apply, JSON error doc suppressed), `ExitCodeFor`, `CodeFor`, `RegisterExitCode`, unexported `register`/`registeredCommands`, `RegisterSchema`. F27 RunE returns errors only; never `AddCommand`, never `os.Exit`.

### 7.6 F03 `internal/output`

`WriteWarning(w io.Writer, message string)` — all F27 stderr warnings (refresh disabled warning, verify warnings, summary lines).

## 8. Security invariants (this feature)

- Routes contain no credential material (provider ids, model ids, and window ids only); nothing to redact — but `add`/`remove` never echo flag values into output beyond the route id.
- The scores CSV hash is read-only metadata; verify never writes or modifies the CSV.
- `refresh` never deletes user_declared routes (F18's merge guarantees it; F27's compare-and-skip preserves the file when nothing changed).

### Shared catalog schema correction (#166)

All catalog consumers decode the complete `catalog` table through F01's strict,
table-only `UnmarshalKey`. `internal/catalog.Config` owns all existing catalog
fields plus `Publish PublishConfig` (`toml:"publish"`); `catalog.PublishConfig`
owns the existing nested publishing fields. `whichmodel.CatalogConfig` and
`publish.PublishConfig` remain public aliases. Unknown catalog and publish keys
are errors; valid nested publishing settings coexist with configured raw/scores
paths, including environment-only overrides. Publishing seeds nested defaults,
then reads raw artifact paths from the decoded root. Pick validates catalog
configuration before loading scores and propagates config errors as exit 2.
Empty consumer paths retain their previous defaults. This corrects scalar
accessor guidance that contradicted F01's table-only contract.
