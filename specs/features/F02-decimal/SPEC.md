---
kind: feature-spec
feature: F02-decimal
version: "1.0"
project: which-model
---

# F02 — decimal: SPEC

## Purpose

`internal/decimal` is the single numeric wrapper package of the `which-model` binary: every score, weight, and mean is a `github.com/shopspring/decimal` `decimal.Decimal`, and F02 is the only package that decides how strings, rounding, display, and weighted means behave on that type. It exists because the plan mandates **no `float64` anywhere on the numeric path** (annex-b §1: floating point makes tie handling and display nondeterministic across architectures), and because every scoring feature (F06–F10) must round identically — half away from zero — and display identically. F02 is Layer 0: it imports nothing but `github.com/shopspring/decimal` and the standard library (dependency rule, `specs/global/CONTRACTS.md` §8; F02 has `depends_on: —` in `specs/DEPENDENCY-GRAPH.md`).

Source: `docs/plan/annex-b-catalog-port.md` §1 (no-float64 rule, `Round(0)` = ROUND_HALF_UP); `docs/plan/annex-b-catalog-port.md` §1.1 (tie handling: 0.5 → 1, not 0); `docs/plan/annex-d-cli-reference.md` §3.2 (score display: rounded to the nearest integer, "0".."100").

## Behaviour

1. **Parse.** `Parse(s string) (decimal.Decimal, error)` wraps `decimal.NewFromString` unchanged: accepted inputs are decimal literals (`"0"`, `"0.85"`, `"100"`, `"-1.5"`, `"1e3"`); garbage (`""`, `"abc"`, `"1..2"`, `"1.2.3"`, `"NaN"`) returns a non-nil error. No float64 conversion ever happens (annex-b §1).
2. **RoundHalfUp.** `RoundHalfUp(d, places)` is `d.Round(places)` — shopspring's `Round` rounds half away from zero, i.e. Python's `ROUND_HALF_UP` (annex-b §1.1): `0.5 → 1`, `1.5 → 2`, `2.5 → 3`, `-0.5 → -1`, `-1.5 → -2`. `places` may be negative (`RoundHalfUp(125, -1) → 130`); the value passes through shopspring's rounding, no extra behavior.
3. **ScoreString.** `ScoreString(d) string` = `RoundHalfUp(d, 0).StringFixed(0)` — the canonical display form for scores: the nearest integer, no sign on zero, `"0"`..`"100"` in practice (annex-d §3.2). Because `Round` rounds half away from zero, `ScoreString(0.5) == "1"` (annex-b §1.1); `ScoreString(-0.4) == "0"`, `ScoreString(-0.5) == "-1"`.
4. **WeightedMean.** `WeightedMean(components, weights) (decimal.Decimal, bool)` computes `Σ(cᵢ·wᵢ) / Σ(wᵢ)` over the entries whose weight is strictly positive, at full decimal precision — the caller rounds (`RoundHalfUp(mean, 0)` for a final score). `components` and `weights` are parallel slices, entry `i` of one matching entry `i` of the other. Entries with `weight ≤ 0` (zero or negative) are skipped, not errors — a provider with zero weight contributes nothing (consistent with F01's weight normalization: 0 = unset = multiplicative identity, `specs/features/F01-config/SPEC.md` §7). The second return value is `false` exactly when there is no valid component: length mismatch, both empty, or every weight `≤ 0`; then the first value is `decimal.Zero`. Degenerate inputs are not an error channel — they produce `(Zero, false)` (D3).
5. **No Failure codes, no config keys, no flags.** F02 exports only the four functions of §1–§4. Scoring failure taxonomy (`Failure` + codes) is F06's (F06-F11 layer; `specs/global/CONTRACTS.md` §1.3); F02 never returns a `*Failure`.

## Error behaviour

- `Parse` returns errors verbatim from `decimal.NewFromString` — no wrapping, no taxonomy. Callers (F06+ scoring pipeline) map a parse failure to their own failure codes.
- `RoundHalfUp` and `ScoreString` are total: they cannot fail on any `decimal.Decimal`.
- `WeightedMean` never returns an error; the `bool` result signals "no valid weighted components" and the caller decides the response (F06's contract: a `benchmark` with no valid components is a broken benchmark, surfaced per the scoring feature's own error taxonomy).
- `internal/decimal` introduces no exit codes, no `Failure.Code` values, and no config validation errors.

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Wrapper package | `internal/decimal` with `decimal.Decimal` as the exposed type, not a new named type | shopspring is the plan's canonical decimal (annex-b §1); a named wrapper type would force conversions at every callsite in F06–F10 for no benefit |
| D2 | Rounding mode | `d.Round(places)` — half away from zero | annex-b §1.1 verbatim (Python `ROUND_HALF_UP`); `RoundBank` (tie-to-even) is explicitly NOT used — 0.5 must become 1 |
| D3 | WeightedMean degenerate inputs | `(Zero, false)`, no error channel | a mean over no valid weights is not a config/runtime failure; the bool lets callers branch without error plumbing (F06 weights the response) |
| D4 | WeightedMean skip rule | skip entries with `weight ≤ 0`, renormalize over included entries | zero weight = "no contribution" (F01 normalizes 0 → 1.0 for single-provider weighting; in a mean, skipping is the equivalent); negatives are user error but harmless to skip — F06 validates weights before scoring |
| D5 | ScoreString form | `RoundHalfUp(d,0).StringFixed(0)`, no sign on zero | annex-d §3.2 display ("0".."100"); `StringFixed` renders exactly `n` fraction digits with `Round` semantics pre-applied |
| D6 | Parse surface | wrap `decimal.NewFromString` exactly; no float64, no int→decimal helpers | annex-b §1 no-float64 rule; every other feature parses its own config/CSV strings through `Parse` (F01 `weight`, F06 CSV `score`, F09 aggregators) |

## Out of scope

- Scoring pipeline, `Failure` taxonomy and codes, benchmark/score CSV formats → F06-scoring, F07-aggregation, F08-scoring-transport, F09-scoring-config, F10-scoring-report (F02 is consumed by F06–F10 per `specs/DEPENDENCY-GRAPH.md`; it knows nothing about benchmarks or providers).
- Provider/band weight semantics (what a weight means, normalization to 1.0) → F01-config (`internal/config`, `specs/features/F01-config/SPEC.md` §7) and F19-bands/F20-strategies.
- Config parsing of decimal-typed keys (TOML float → text → `UnmarshalText`) → F01-config (`internal/config/unmarshal.go`, D12).
- `decimal.Decimal` JSON/CSV marshaling beyond what shopspring provides; if F06–F10 need custom encoders they implement them in their own packages.
