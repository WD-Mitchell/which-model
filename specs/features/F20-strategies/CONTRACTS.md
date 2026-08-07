---
kind: feature-contracts
version: "1.0"
feature: F20-strategies
project: which-model
---

# F20 — Strategies: CONTRACTS

## 1. Package

`internal/pick/strategy` (Layer 3, `specs/global/SPEC.md` §3). Importable by `internal/pick` root and above; imports `internal/pick`, `internal/routing` (types), `internal/config`, `github.com/shopspring/decimal`, `github.com/gofrs/flock`, `math/rand/v2` — and nothing else in `internal/` (`specs/global/CONTRACTS.md` §8). The package has **no** `internal/usage` import and compiles under `-tags nousage`.

## 2. Files owned

| File | Contents |
|---|---|
| `internal/pick/strategy/strategy.go` | `Strategy` interface, `State`, `Config`, `RouteKey`, error values, `PriorityOrder` |
| `internal/pick/strategy/score.go` | `Score` implementation |
| `internal/pick/strategy/priority.go` | `Priority` implementation |
| `internal/pick/strategy/round_robin_state.go` | Cursor load/save/advance under `gofrs/flock` |
| `internal/pick/strategy/round_robin.go` | `RoundRobin` implementation |
| `internal/pick/strategy/least_used.go` | `LeastUsed` implementation |
| `internal/pick/strategy/weighted_random.go` | `WeightedRandom` implementation |
| `internal/pick/strategy/cost_optimal.go` | `CostOptimal` implementation |
| `internal/pick/strategy/registry.go` | `New`, `ParseStrategy` |

## 3. Types

```go
package strategy

import (
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/shopspring/decimal"
)

// State carries every piece of invocation context a strategy may read. It is
// built by the pick layer (F26) from config, the usage toggle (F21), and the
// band/pressure evaluation (F19). Zero value is valid for score and priority.
type State struct {
	Profile             string                       // scoring profile name; round-robin cursor scope input
	DataDir             string                       // state directory; round-robin file = <DataDir>/pick/round_robin.json
	ProviderPriority    []string                     // priority: config-ordered, most preferred first (see PriorityOrder)
	Config              Config                       // strategy config from [strategy] (see §4)
	Seed                int64                        // weighted-random PRNG seed
	HasSeed             bool                         // weighted-random REQUIRES true; false ⇒ ErrSeedRequired
	UsageEnabled        bool                         // resolved usage toggle (F21); false ⇒ least-used refuses
	UsageDisabledReason string                       // usage_disabled_reason value (specs/global/CONTRACTS.md §6); "" when enabled
	PressureByProvider  map[string]float64           // least-used: per-provider pressure (max window UsedPercent)
	CostScoreByRouteKey map[string]decimal.Decimal   // cost-optimal: catalog cost score per RouteKey (higher = cheaper)
	DryRun              bool                         // round-robin: compute but do not advance the cursor
}

// Config is the [strategy] TOML table owned by F20 (read via config.UnmarshalKey,
// specs/global/DEPENDENCY-GRAPH.md DECISION B). Unknown keys are F01's validation
// concern, not this struct's.
type Config struct {
	Default          string  `toml:"default"`             // pick.Strategy name; "" = "score"
	CostMaxScoreDrop float64 `toml:"cost_max_score_drop"` // FinalScore points; 0 = 5.0 (default)
}

// ResolvedCostMaxScoreDrop applies the default: zero ⇒ decimal 5.0.
func (c Config) ResolvedCostMaxScoreDrop() decimal.Decimal

// Strategy selects one candidate from the eligible set. candidates MUST be
// non-gated (upstream exclusions carry reason codes band_gated/no_score_row/
// auth_required/provider_error/not_in_availability_list per
// docs/plan/annex-c-agent-integration.md §4.2 and never reach a strategy).
//
// excluded holds every input candidate except the pick, in RouteKey ascending
// order. Implementations sort a copy of the input first, so results are
// independent of caller order.
type Strategy interface {
	Name() pick.Strategy
	Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error)
}

// Implementations (zero-value usable except RoundRobin, which reads state.DataDir):
type Score         struct{} // max FinalScore, tie-break FinalScore desc → RouteKey asc
type Priority      struct{} // first provider in state.ProviderPriority with any candidate; max FinalScore within it
type RoundRobin    struct{} // cursor rotation over sorted candidates, state file + flock
type LeastUsed     struct{} // min state.PressureByProvider; refuses when usage disabled
type WeightedRandom struct{} // weighted sample, weights = BandWeight × ProviderWeight, math/rand/v2 PCG(seed, seed)
type CostOptimal   struct{} // max cost score among candidates within CostMaxScoreDrop of max FinalScore
```

## 4. Functions

```go
// RouteKey is the canonical candidate key: provider + "/" + model_id + "/" + reasoning
// (F20 SPEC D6). Used for round-robin scope keys, cost lookups, and tie-breaks.
func RouteKey(c pick.Candidate) string

// RouteKeyFromRoute is RouteKey for a routing.Route (same rule).
func RouteKeyFromRoute(r routing.Route) string

// PriorityOrder sorts provider IDs by descending configured priority, ties by
// provider ID ascending (F20 SPEC D9). The pick layer builds
// State.ProviderPriority = PriorityOrder(provider→priority from
// config.Providers[].Priority, annex-d §4.2 "higher = preferred").
func PriorityOrder(priorities map[string]int) []string

// New returns the implementation for a pick.Strategy enum value
// (specs/global/CONTRACTS.md §4.2). Unknown values return an error wrapping
// ErrUnknownStrategy.
func New(s pick.Strategy) (Strategy, error)

// ParseStrategy maps the strategy.default config string to pick.Strategy;
// "" ⇒ StrategyScore. Unknown values return an error wrapping ErrUnknownStrategy.
func ParseStrategy(s string) (pick.Strategy, error)
```

## 5. Errors

```go
var (
	// ErrNoCandidates: "no candidates to pick from". Every strategy; F26 maps to exit 3.
	ErrNoCandidates = errors.New("no candidates to pick from")

	// ErrSeedRequired: "weighted-random requires --seed for reproducibility".
	// WeightedRandom with State.HasSeed false; F26 maps to exit 2.
	ErrSeedRequired = errors.New("weighted-random requires --seed for reproducibility")

	// ErrUnknownStrategy: "unknown strategy (valid: score, priority, round-robin,
	// least-used, weighted-random, cost-optimal)". New/ParseStrategy wrap it:
	// fmt.Errorf("%w: %q", ErrUnknownStrategy, s); F26 maps to exit 2.
	ErrUnknownStrategy = errors.New("unknown strategy (valid: score, priority, round-robin, least-used, weighted-random, cost-optimal)")
)

// ErrLeastUsedRequiresUsage — least-used with usage disabled at any level
// (flag | config | compiled_out | no_providers_enabled). The message names the
// disabling source (F20 SPEC D11); F26 maps to exit 2. Match with errors.As.
type ErrLeastUsedRequiresUsage struct{ Reason string }

func (e *ErrLeastUsedRequiresUsage) Error() string // "least-used requires usage data; usage is disabled by <source>"

// ErrMissingPressure — least-used with usage enabled but no pressure entry for
// a candidate's provider; F26 maps to exit 1. Match with errors.As.
type ErrMissingPressure struct{ Provider string }

func (e *ErrMissingPressure) Error() string // `no usage pressure data for provider "<provider>"`
```

## 6. Config keys owned

| Key | Type | Default | Meaning |
|---|---|---|---|
| `strategy.default` | string | `"score"` | Default `--strategy` value (`docs/plan/annex-d-cli-reference.md` §4.2); parsed via `ParseStrategy` |
| `strategy.cost_max_score_drop` | float | `5.0` | `cost-optimal` FinalScore threshold (F20 SPEC D4) |
| `providers.<id>.priority` | int | `0` | Ordering semantics for `priority` (higher = preferred); consumed as `State.ProviderPriority` via `PriorityOrder` |

Read via F01's `(*config.Config).UnmarshalKey("strategy", &strategy.Config)` (`specs/global/DEPENDENCY-GRAPH.md` DECISION B). `providers.<id>` is F01-typed config; F20 owns only the ordering semantics.

## 7. Flags owned (semantics; flag definitions live in F26)

| Flag | Contract |
|---|---|
| `--strategy` | Values are exactly the six `pick.Strategy` enum strings (`specs/global/CONTRACTS.md` §4.2); default from `strategy.default` |
| `--seed <int64>` | REQUIRED for `weighted-random`; missing ⇒ `ErrSeedRequired` ⇒ exit 2 |
| `--dry-run` | `State.DryRun`; round-robin cursor not advanced; no effect for weighted-random (`docs/plan/annex-d-cli-reference.md` §3.2) |

## 8. Error codes added

None. Exit-code mapping is F26's (`specs/global/SPEC.md` §5): `ErrSeedRequired`/`ErrUnknownStrategy`/`ErrLeastUsedRequiresUsage` → 2; `ErrNoCandidates` → 3; `ErrMissingPressure` → 1.

## 9. State file JSON (round-robin)

Path: `<State.DataDir>/pick/round_robin.json` (dirs `0700`, file `0600`; `docs/plan/annex-d-cli-reference.md` §4.5 resolves the state dir). Shape:

```json
{
  "<scope_key>": { "index": 3, "updated_at": "2026-08-07T17:03:12Z" }
}
```

- `scope_key` = `hex(sha256(State.Profile + "|" + strings.Join(sortedRouteKeys, "|")))[:16]`, route keys via `RouteKey`, sorted ascending.
- `index` = zero-based index of the NEXT pick (`candidates[index % len]`); default `0` when absent; advanced to `index+1` under the exclusive flock after each non-dry-run pick; `updated_at` = RFC 3339 UTC.
- Corrupt/unreadable file ⇒ treated as empty cursor set (all scopes index 0), never an error.
- Lock: `github.com/gofrs/flock` (`flock.New(path)` + blocking `Lock()`/`Unlock()`); file opened `O_RDWR|O_CREATE`; write followed by fsync before unlock (F20 SPEC D2, D5, D7, D8).

## 10. Cross-feature references

- `pick.Candidate`, `pick.Strategy` + six enum constants — `specs/global/CONTRACTS.md` §4 (files `internal/pick/…`, F10/F19/F26).
- `usage_disabled_reason` values (`flag|config|compiled_out|no_providers_enabled`) — `specs/global/CONTRACTS.md` §6; resolved by F21 `internal/usage/toggle`; F20 reads them as plain strings on `State`.
- `config.Config`/`config.UnmarshalKey` — F01 `internal/config`.
- Degraded candidates (`Band = ""`, `BandWeight = 1.0`) are produced by F21's `internal/pick/degraded.go` before strategies run; strategies consume the resulting `FinalScore` unchanged.
- Band/pressure/gating evaluation — F19 `internal/pick/band` (input side, never imported by F20).
