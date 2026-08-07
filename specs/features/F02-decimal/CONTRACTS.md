---
kind: feature-contracts
feature: F02-decimal
version: "1.0"
project: which-model
---

# F02 — decimal: CONTRACTS

Package `internal/decimal` (Layer 0). Imports: `github.com/shopspring/decimal` + Go stdlib. MUST NOT import anything in `internal/` and MUST NOT use `float64` anywhere (`specs/global/CONTRACTS.md` §8; `docs/plan/annex-b-catalog-port.md` §1). Files: `internal/decimal/decimal.go` (+ test files per task). Feature `depends_on: —`, blocks F06, F07, F08, F09, F10 (`specs/DEPENDENCY-GRAPH.md`).

## 1. Exported API

All four functions live in `internal/decimal/decimal.go`, package `decimal`.

```go
package decimal

import "github.com/shopspring/decimal"

// Parse wraps decimal.NewFromString unchanged. Accepts decimal literals
// ("0", "0.85", "100", "-1.5", "1e3"); anything else returns a non-nil
// error verbatim from NewFromString. No float64 conversion anywhere.
func Parse(s string) (decimal.Decimal, error)

// RoundHalfUp rounds d to places digits after the decimal point, half away
// from zero (Python ROUND_HALF_UP, annex-b §1.1): 0.5 → 1, -0.5 → -1.
// Equivalent to d.Round(places); places may be negative (125 @ -1 → 130).
func RoundHalfUp(d decimal.Decimal, places int32) decimal.Decimal

// ScoreString renders d as the canonical integer score string:
// RoundHalfUp(d, 0).StringFixed(0) — "0".."100" in practice (annex-d §3.2);
// no sign on zero ("0", not "+0"/"-0").
func ScoreString(d decimal.Decimal) string

// WeightedMean computes Σ(cᵢ·wᵢ)/Σ(wᵢ) over entries with weight > 0, at
// full decimal precision (caller rounds). components and weights are
// parallel slices. Entries with weight <= 0 are skipped, not errors. The
// bool is false exactly when no valid component exists (length mismatch,
// both empty, or all weights <= 0), in which case the value is
// decimal.Zero. Never returns an error.
func WeightedMean(components, weights []decimal.Decimal) (decimal.Decimal, bool)
```

## 2. Config keys owned

None. (Decimal-typed config values — `providers.<id>.weight`, band tier weights — are owned by F01-config/F19; they reach `decimal.Decimal` through `internal/config`'s decode path, `specs/features/F01-config/CONTRACTS.md` §2.)

## 3. Env vars owned

None.

## 4. Flags owned

None.

## 5. Error codes added

No `Failure.Code` values, no exit codes. `Parse` returns `decimal.NewFromString`'s error; `WeightedMean` signals degeneracy with its `bool`. Scoring failure codes (`Failure`, `specs/global/CONTRACTS.md` §1.3) are F06's.

## 6. JSON shapes emitted

None. (Score/result JSON is F06–F10's composition; if they emit decimals they use shopspring's marshaling, which F02 does not alter.)

## 7. Consumers

- F06-scoring: `Parse` for score CSV cells and weight strings; `RoundHalfUp(mean, 0)` for final scores; `WeightedMean` for combining band scores.
- F07-aggregation: `WeightedMean` for `weighted-arithmetic-mean`; `ScoreString` for display of aggregate results.
- F09-scoring-config: `Parse` for `weight`-typed config values surfaced via `internal/config` `UnmarshalKey` (the decimal field types decode through F01's `encoding.TextUnmarshaler` path; F09 uses `decimal.Decimal` fields directly).
- F08-scoring-transport / F10-scoring-report: `RoundHalfUp` + `ScoreString` for report formatting (0/1/2 decimal places via `RoundHalfUp(d, n).StringFixed(n)`).
- F01-config: imports shopspring directly for its `ProviderConfig.Weight` field — it MUST NOT import `internal/decimal` (F01 keeps its dependency row empty, `specs/DEPENDENCY-GRAPH.md`); `internal/decimal` likewise never imports `internal/config`.
