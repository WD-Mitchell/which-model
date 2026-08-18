---
kind: feature-contracts
version: "1.0"
feature: B00-backend
project: which-model-desktop
---

# B00-backend — Contracts

## 1. Package and files

Package `internal/service` (feature files per SPEC §5) plus B01's additions to `internal/config`. Import boundary: MAY import `internal/{pick,pick/strategy,pick/band,routing,catalog,catalog/score,catalog/csvstore,usage,usage/fetch,usage/cache,usage/toggle,config}`, `shopspring/decimal`, stdlib. MUST NOT import `pkg/whichmodel`, `github.com/wailsapp/*`, cobra.

## 2. Core construction

```go
package service

// EmitFunc delivers an event to the host. Must be non-blocking (host's duty).
type EmitFunc func(event string, payload any)

// Services is the single instance the host owns. Zero value is unusable.
type Services struct { /* unexported: mu, paths, cfg, catalog caches, emit, stats */ }

// New loads config-adjacent state eagerly and fails fast (B00 SPEC §2.1).
func New(paths config.Paths, cfg *config.Config, emit EmitFunc) (*Services, error)
```

## 3. Error mapping (implemented in B02's `service.go`; features declare sentinels)

| Sentinel / condition | Code |
|---|---|
| `errValidation` (wrapped, message = field detail) | `validation_failed` |
| `errBuiltinReadonly` | `builtin_readonly` |
| `errNotFound` | `not_found` |
| `errConflict` | `conflict` |
| `fs errors, ctx cancellation (non-usage)` | `io_error` |
| `errUsageUnavailable`, fetch ctx errors | `usage_unavailable` |
| `errLaunchFailed` | `launch_failed` |

`func toErrorDTO(err error) ErrorDTO` — boundary conversion; also `func (e ErrorDTO) Error() string` so hosts can return it directly.

## 4. Weight conversion helpers (B02 owns; all features use)

```go
// dtoWeights: decimal map (engine) -> int map (DTO); drops zero after rounding.
func dtoWeights(m map[string]decimal.Decimal) map[string]int
// engineWeights: int map (DTO) -> decimal map; keys with v<=0 removed; v>5 -> errValidation.
func engineWeights(m map[string]int) (map[string]decimal.Decimal, error)
// engineProfile builds a catalog.Profile from a ProfileDetail (shares from CoreShare).
func engineProfile(d ProfileDetail) (catalog.Profile, error)
// round2 rounds a decimal score for the boundary.
func round2(d decimal.Decimal) float64
```

## 5. Test helper (B02 owns)

```go
type TestOption func(*testFixture)
// newTestServices builds a full temp tree + Services + recording emitter.
func newTestServices(t *testing.T, opts ...TestOption) (*Services, *emitRecorder)
func WithConfigTOML(s string) TestOption      // replaces config.toml content
func WithScoresCSV(csv string) TestOption     // replaces fixture scores
func WithRoutes(rt routing.Table) TestOption  // replaces synthetic routes table
type emitRecorder struct{ /* Events() []recordedEvent */ }
```
Default fixture: scores from `internal/catalog/score/testdata/scores_golden.csv`; `benchmarks.toml` + `providers.toml` copied from `available-model-data-export/`; routes table with providers `claude`,`codex` over the fixture models; empty config.toml.

## 6. Cross-feature invariants

1. Provider order everywhere = ascending `providers.<id>.priority`, ties broken by id asc.
2. "Enabled providers" = `providers.<id>.enabled == true` (config absent ⇒ disabled — default-deny, matching the CLI).
3. Availability set (used by B04, B06, B09): routes table entries whose provider is enabled AND whose `model_id@reasoning` is not listed under `[routes.disabled].<provider>`.
4. Builtin merge rule (profiles B03, groups B05, harnesses B07): builtins come from code/benchmarks.toml; customs from config; a custom slug colliding with a builtin slug is `conflict` at save time; builtins are never written to config except the harness seed (B07 Deviations).
5. Every `*_test.go` asserts events via the recorder: one event per mutation, zero on read/validation-failure paths.
