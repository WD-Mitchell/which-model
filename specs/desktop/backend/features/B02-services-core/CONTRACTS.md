---
kind: feature-contracts
version: "1.0"
feature: B02-services-core
project: which-model-desktop
---

# B02-services-core — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/service.go` | `EmitFunc`, `Services`, `New`, sentinels, `toErrorDTO`, weight helpers, `reloadCatalog`, `Warnings` |
| `internal/service/dto.go` | every D00 CONTRACTS §2 struct verbatim (shape owned by D00, not restated here) + `ParseRouteKey`/`FormatRouteKey` |
| `internal/service/events.go` | D00 CONTRACTS §3 event consts |
| `internal/service/service_test.go` | §9 tests for construction, route keys, weight conversion, write discipline |
| `internal/service/testhelp_test.go` | `newTestServices`, `TestOption`s, `emitRecorder`, fixture embedding |

Import boundary: B00 CONTRACTS §1 applies unchanged.

## 2. service.go — core

```go
package service

// EmitFunc delivers an event to the host. Must be non-blocking (host's duty).
// service.New replaces nil with a no-op (SPEC §2.1).
type EmitFunc func(event string, payload any)

// Services is the single instance the host owns. Zero value is unusable.
// Unexported fields: mu sync.RWMutex; paths config.Paths; cfg *config.Config;
// scores []catalog.ScoreRow; bench *score.BenchmarkConfig; routes routing.Table;
// warnings []string; emit EmitFunc; plus per-feature caches (B03..B11).
type Services struct { /* unexported */ }

// New eagerly loads scores CSV, benchmarks config, and routes table in that
// order, failing fast on the first error (SPEC §2.2–2.5). Missing routes
// table is non-fatal (empty availability + warning).
func New(paths config.Paths, cfg *config.Config, emit EmitFunc) (*Services, error)

// Warnings returns non-fatal construction warnings (currently only the
// missing-routes-table warning, §7), copied, in occurrence order.
func (s *Services) Warnings() []string

// reloadCatalog re-reads scores CSV + benchmarks config + routes table and
// swaps all three caches atomically under the write lock; on error the
// previous caches stay live (SPEC §2.10). Emits nothing.
func (s *Services) reloadCatalog() error
```

## 3. service.go — sentinels and boundary mapping

```go
// Sentinel errors; features wrap them (fmt.Errorf("%w: ...", errValidation))
// so toErrorDTO recovers the code via errors.Is.
var (
    errValidation       = errors.New("validation failed")
    errBuiltinReadonly  = errors.New("builtin is read-only")
    errNotFound         = errors.New("not found")
    errConflict         = errors.New("already exists")
    errUsageUnavailable = errors.New("usage unavailable")
    errLaunchFailed     = errors.New("launch failed")
)

// errScoresMissing is the typed fail-fast error for SPEC §2.3; it wraps
// fs.ErrNotExist and renders the §7 message. Maps to io_error.
type scoresMissingError struct{ Path string }
func (e *scoresMissingError) Error() string
func (e *scoresMissingError) Unwrap() error // fs.ErrNotExist

// toErrorDTO maps any internal error to the boundary shape (SPEC §2.7):
// an ErrorDTO (or *ErrorDTO) passes through; then errors.Is against the
// sentinels per §5; then usage fetch ctx errors -> usage_unavailable;
// everything else -> io_error with sanitised message.
func toErrorDTO(err error) ErrorDTO

// Error makes ErrorDTO returnable directly from bound methods.
func (e ErrorDTO) Error() string // "<code>: <message>"
```

## 4. service.go — weight conversion (B00 CONTRACTS §4, exact semantics)

```go
// dtoWeights converts engine decimals to DTO ints: each value Round(0)
// half-up to int; keys rounding to <=0 are dropped (SPEC §2.9).
func dtoWeights(m map[string]decimal.Decimal) map[string]int

// engineWeights converts DTO ints to engine decimals: keys with v<=0
// removed; any v>5 -> fmt.Errorf("%w: weight %q is %d, must be 0..5",
// errValidation, key, v) checking keys in sorted order (deterministic).
func engineWeights(m map[string]int) (map[string]decimal.Decimal, error)

// engineProfile builds a catalog.Profile from a ProfileDetail:
// Tier1Share = CoreShare/100 (decimal, no rounding), Tier2Share = 1-Tier1Share,
// Name = Slug, weights via engineWeights (tier1 error checked first).
func engineProfile(d ProfileDetail) (catalog.Profile, error)

// round2 is the ONLY decimal->float64 crossing: d.Round(2).InexactFloat64().
func round2(d decimal.Decimal) float64
```

## 5. Error mapping table (implements B00 CONTRACTS §3)

| Matched (in this order) | Code |
|---|---|
| `ErrorDTO` / `*ErrorDTO` in chain | passed through unchanged |
| `errValidation` | `validation_failed` |
| `errBuiltinReadonly` | `builtin_readonly` |
| `errNotFound` | `not_found` |
| `errConflict` | `conflict` |
| `errUsageUnavailable`, ctx errors from usage fetch | `usage_unavailable` |
| `errLaunchFailed` | `launch_failed` |
| anything else (incl. `*scoresMissingError`, fs, TOML, non-usage ctx) | `io_error` |

## 6. dto.go — route keys (grammar: D00 CONTRACTS §1)

```go
// ParseRouteKey splits s at the FIRST '/' and the LAST '@', then validates
// components. Errors (all wrapping errValidation), checked in this order:
//   no '/'                    -> route key %q: missing "/"
//   no '@' after the '/'      -> route key %q: missing "@"
//   empty component           -> route key %q: empty <provider|model_id|reasoning>
//   provider !~ [a-z0-9_]+    -> route key %q: invalid provider %q
//   model_id !~ [A-Za-z0-9._-]+ -> route key %q: invalid model_id %q
//   reasoning not in enum     -> route key %q: invalid reasoning %q
// (enum: minimal|low|medium|high|xhigh|max|default)
func ParseRouteKey(s string) (provider, modelID, reasoning string, err error)

// FormatRouteKey returns provider + "/" + modelID + "@" + reasoning.
// No validation (SPEC §2.8).
func FormatRouteKey(provider, modelID, reasoning string) string
```

## 7. Fixed strings (exact, golden-tested)

| Where | String |
|---|---|
| `scoresMissingError.Error()` | `scores CSV not found at %s; run: which-model catalog refresh` (absolute path) |
| routes warning in `Warnings()` | `routes table not found at %s; availability is empty until: which-model routes refresh` (primary path) |

## 8. events.go

```go
const (
    EventConfigChanged   = "config:changed"
    EventCatalogChanged  = "catalog:changed"
    EventUsageUpdated    = "usage:updated"
    EventSettingsChanged = "settings:changed"
    EventPickRecorded    = "pick:recorded"
)
```

## 9. Test helper and fixtures (B00 CONTRACTS §5)

```go
type TestOption func(*testFixture)

// newTestServices builds a t.TempDir() config/cache/state tree, writes the
// default fixtures, applies opts, loads config, and calls New. Fatal on any
// setup error.
func newTestServices(t *testing.T, opts ...TestOption) (*Services, *emitRecorder)

func WithConfigTOML(s string) TestOption     // replaces config.toml content
func WithScoresCSV(csv string) TestOption    // replaces fixture scores; "" = omit the file
func WithRoutes(rt routing.Table) TestOption // replaces synthetic routes table

type recordedEvent struct{ Event string; Payload any }
type emitRecorder struct{ /* mu + slice */ }
func (r *emitRecorder) Events() []recordedEvent // copy, mutex-guarded
```

**Default fixture tree** (under the temp dir):
- `cache/catalog/available_model_scores.csv` — byte-copy of `internal/catalog/score/testdata/scores_golden.csv` (go:embed or read at test time via relative path).
- `cache/catalog/benchmarks.toml`, `config/providers.toml` — copies of `available-model-data-export/benchmarks.toml` and `available-model-data-export/providers.toml`.
- `cache/catalog/routes.json` — synthetic `routing.Table{SchemaVersion: routing.TableSchemaVersion}` covering providers `claude`,`codex` × ≥2 fixture models × ≥2 reasoning levels, written via `routing.SaveTable`.
- `config/config.toml` — empty file.

**Required tests** (`service_test.go`):

| Test | Asserts |
|---|---|
| `TestNew_LoadsFixtures` | `New` succeeds on the default tree; scores/bench/routes caches populated; `Warnings()` empty; zero events emitted |
| `TestNew_MissingScoresCSV` | `WithScoresCSV("")` ⇒ `New` errors; `toErrorDTO` code `io_error`; message contains the §7 string with the fixture path |
| `TestNew_MissingRoutesTable` | routes file removed ⇒ `New` succeeds, empty table, `Warnings()` has exactly the §7 warning |
| `TestRouteKey_RoundTrip` | for every fixture route: `FormatRouteKey` → `ParseRouteKey` round-trips; each §6 error case yields `validation_failed` with the exact message |
| `TestWeightConversion` | golden cases: `{a:3,b:0}` → engine drops `b`; `{a:6}` → `errValidation` (exact §4 message); decimal `2.5` → dto `3`; decimal `0.4` → dropped (0-drop); `round2(decimal("1.005"))` documented result asserted |
| `TestWriteDiscipline_FailureLeavesStateAndNoEvent` | a mutation forced to fail at AtomicWriteFile (read-only dir) leaves config bytes + in-memory state unchanged and the recorder empty |

## 10. External symbols referenced

`score.ParseScoresCSV`, `score.ParseBenchmarkConfig`, `score.BenchmarkConfig` (`internal/catalog/score`); `catalog.ScoreRow`, `catalog.Profile` (`internal/catalog`); `routing.LoadTable`, `routing.SaveTable`, `routing.Table`, `routing.TableSchemaVersion` (`internal/routing`); `config.Paths`, `config.Config`, `config.MarshalTOML` (`internal/config`); `config.AtomicWriteFile` (B01); `decimal.Decimal.Round/InexactFloat64/NewFromInt` (`shopspring/decimal`).
