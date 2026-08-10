---
kind: feature-contracts
version: "1.1"
feature: F20-strategies
project: which-model
---

# F20 — Strategies: CONTRACTS

## 1. Package

`internal/pick/strategy` imports `internal/pick`, `internal/routing`, `github.com/gofrs/flock`, and the standard library. It has no `internal/usage` import and compiles under `-tags nousage`.

## 2. Files owned

| File | Contents |
|---|---|
| `strategy.go` | shared interface, state, helpers, and errors |
| `priority.go` | default priority selection |
| `round_robin.go`, `round_robin_state.go` | cursor-based rotation |
| `least_used.go` | lowest-pressure selection |
| `most_used.go` | highest-pressure selection |
| `closest_to_reset.go` | earliest-reset selection |
| `registry.go` | validation and construction |

## 3. Types

```go
type State struct {
    Profile             string
    DataDir             string
    ProviderPriority    []string
    Config              Config
    UsageEnabled        bool
    UsageDisabledReason string
    PressureByProvider  map[string]float64
    ResetAtByProvider   map[string]time.Time
    DryRun              bool
}

type Config struct {
    Default string `toml:"default"`
}

type Strategy interface {
    Name() pick.Strategy
    Pick([]pick.Candidate, *State) (pick.Candidate, []pick.Candidate, error)
}
```

## 4. Functions

```go
func RouteKey(pick.Candidate) string
func RouteKeyFromRoute(routing.Route) string
func PriorityOrder(map[string]int) []string
func ParseStrategy(string) (pick.Strategy, error) // empty => StrategyPriority
func New(pick.Strategy) (Strategy, error)
```

## 5. Errors

```go
var ErrNoCandidates error
var ErrUnknownStrategy error

type ErrLeastUsedRequiresUsage struct{ Reason string }
type ErrMostUsedRequiresUsage struct{ Reason string }
type ErrClosestToResetRequiresUsage struct{ Reason string }
type ErrMissingPressure struct{ Provider string }
type ErrMissingReset struct{ Provider string }
```

## 6. Config and flags

| Surface | Default | Contract |
|---|---|---|
| `strategy.default` | `"priority"` | canonical strategy name |
| `providers.<id>.priority` | `0` | higher values are preferred |
| `--strategy` | `priority` | one of the five canonical names |
| `--dry-run` | `false` | round-robin does not advance |

`--seed` and `strategy.cost_max_score_drop` are removed.

## 7. Round-robin state

Path: `<State.DataDir>/pick/round_robin.json`. The state contains `scope_key` and `index`. Scope key is `hex(sha256(State.Profile + "|" + strings.Join(sortedRouteKeys, "|")))[:16]`.

## 8. Build and dependency constraints

The package must compile and test in default and `nousage` builds. It consumes `pick.Candidate`, `pick.Strategy`, and the five canonical strategy constants from `specs/global/CONTRACTS.md` §4.
