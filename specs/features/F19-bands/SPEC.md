---
kind: feature-spec
version: "1.0"
feature: F19-bands
project: which-model
---

# F19-bands — Pressure, Bands, and Gating

## 1. Purpose

`internal/pick/band` turns a provider's usage `Snapshot` and a route's gating `WindowIDs` into the single scalar every selection strategy needs — pressure — then maps that pressure onto a configurable band ladder (`[bands]`) that assigns a `BandWeight` multiplier, and enforces the hard quota gate (`gate_above_used_percent`) that excludes a candidate entirely with reason code `band_gated`. It is the usage-aware weighting core of `which-model pick` (master plan §5).

Depends on: F01, F11

## 2. Behaviour

1. **Pressure is per-route, and it is the max.** `Pressure(snapshot, windowIDs)` returns the percent used for a route: `max(UsedPercent(w) for w in snapshot.Windows if w.ID in windowIDs)`. Max, not mean — the binding constraint is what stops you: a weekly lane at 90% with a 5-hour lane at 10% is a 90%-constrained route. (master plan §5.1)

2. **Per-window percent derivation.** `WindowPercent(w)` follows, in priority order: (a) `Synthetic` windows contribute nothing (a synthesized placeholder is not a real 0% lane); (b) `Unlimited` windows are 0% used (real knowledge, not an invented denominator); (c) `UsageKnown == false` windows are unknown (reset metadata without usage is not a number); (d) `UsedPercent` as reported, which may exceed 100; (e) `Used` + `Limit` → `Used / Limit * 100`; (f) `Remaining` + `Limit` → `(Limit - Remaining) / Limit * 100`; (g) balance only, or a non-positive `Limit` → unknown — a balance without an entitlement is not a proportion. (master plan §5.1 derivation table, §3.2)

3. **Unknown pressure.** Pressure is `Known == false` when: the snapshot carries `Failure` (fetch failed this run), or none of the route's gating windows yields a computable percent (including an empty `windowIDs` set). Windows named in `windowIDs` but absent from the snapshot contribute nothing — an optional window the provider did not report does not poison the pressure. (master plan §5.3; annex-a `WindowSpec.Optional`)

4. **Bands.** A band configuration is `[bands]` with `direction`, `gate_above_used_percent`, `unknown_pressure_weight`, and an ascending list of `[[bands.tier]]` entries (`name`, `upper_used_percent`, `weight`). The TOML shape is verbatim from the plan. A route's band is the first tier (ascending `upper_used_percent`) whose bound is `>=` pressure; pressure above the last bound clamps to the last tier. The upper bound is INCLUSIVE: exactly 25 maps to `low`, exactly 50 to `standard`, exactly 75 to `elevated`. (master plan §5.2)

5. **Weights and direction.** `direction = "spread"` (the default) assigns each tier its declared `weight` — high consumption lowers the weight, biasing picks toward providers with headroom. `direction = "drain"` reverses the weight assignment across tiers: tier *N* takes `weight[len-1-N]` — the same ladder yields `critical = 1.00, elevated = 0.85, standard = 0.60, low = 0.25`. Direction changes the weight a tier carries, never which tier a pressure maps to. Both directions ship; `spread` is the default. (master plan §5.2; `docs/plan/annex-d-cli-reference.md` §4.3)

6. **Gating.** When pressure is known and `percent >= gate_above_used_percent`, the candidate is GATED: excluded from `pick` output entirely, not merely down-weighted, with reason code `band_gated`. The threshold is inclusive ("at or above"). A gated result carries no band name and a zero weight. (master plan §5.2; annex-c §2.3 and §4.2 `reason_code` enum; global SPEC §5 exit 4)

7. **Unknown pressure policy.** Unknown pressure is never gated, never treated as 0%, and never treated as 100%. It earns `unknown_pressure_weight` (default 0.90) with a warning — neutral-with-a-warning rather than optimistic or exclusionary, because treating unknown usage as 0% would make every unmeasurable provider outrank every measured one. The pseudo-band name for unknown pressure is `"unknown"`, which is reserved and rejected in user tier names. (master plan §5.3)

8. **Validation.** `ValidateBands` rejects, in fixed order: an invalid `direction`, a negative `gate_above_used_percent`, a non-positive `unknown_pressure_weight`, an empty tier list, tier bounds that are not strictly ascending, duplicate tier names, a tier named `"unknown"`, and a non-positive tier weight. Config errors surface as exit 2 via the F01/F26 wiring. (master plan §5.2; annex-b §6.5 closed-schema ethos)

9. **Defaults.** When `[bands]` is absent (or a field is unset), `FromTOML` applies the plan defaults: `direction = "spread"`, `gate_above_used_percent = 98`, `unknown_pressure_weight = 0.90`, and the four-tier ladder `low/25/1.00`, `standard/50/0.85`, `elevated/75/0.60`, `critical/100/0.25`. (master plan §5.2 TOML verbatim)

10. **Decimal discipline.** Every numeric quantity F19 computes or compares — pressure, per-window percent, tier bounds, weights — is `github.com/shopspring/decimal`. Float64 appears only at the boundary where the canonical `usage.Window` fields (`*float64`) are converted via `decimal.NewFromFloat`; the `[bands]` TOML floats decode straight into `decimal.Decimal` (BurntSushi's TextUnmarshaler path; F01 SPEC B12). No float64 exists on the numeric path (annex-b §1). No rounding is applied at pressure or band time; rounding happens in strategy scoring (F20). (global CONTRACTS §1.4; master plan §2)

11. **Usage-disabled behaviour.** F19's package is usage-active band evaluation only; when usage is disabled at any level, the degraded assembly (band empty, `BandWeight = 1.0`, `[bands]` and `gate_above_used_percent` inert) is F21's `internal/pick/degraded.go`, not F19's. F19 compiles unchanged under `-tags nousage` because it consumes only `usage.Snapshot`/`usage.Window` (types), which F21's stub surface keeps compiled in. (master plan §6.3)

## 3. Error behaviour

- `Pressure` and `EvaluateBand` are total functions: they never return errors. Failure conditions (fetch failure, unknown units, unknown pressure) are encoded in `Pressure.Known` and `Result.Warning`.
- `ValidateBands` and `FromTOML` return errors with the exact messages in `specs/features/F19-bands/CONTRACTS.md` §5; callers map them to exit 2.
- `EvaluateBand` assumes a validated `Config` (documented precondition); callers MUST run `ValidateBands`/`FromTOML` before use.
- Gating is reported structurally (`Result.Gated`), and the exclusion reason code is the constant `ReasonCodeBandGated` — consumers must never construct the string by hand (annex-c §4.2 enum stability).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Multi-window pressure reduction | Max over the route's gating windows (`windowIDs`), skipping synthetic/uncomputable windows; unknown only when no gating window computes | master plan §5.1: "max, not mean: the binding constraint is what stops you" — a weighted average would let a quiet lane hide an exhausted lane |
| Band edge inclusivity | Upper bound inclusive: first tier with `upper_used_percent >= pressure`; exactly 25/50/75 hit low/standard/elevated | master plan §5.2: "first tier whose bound is >= pressure" |
| Gating threshold inclusivity | `percent >= gate_above_used_percent` gates ("at or above") | annex-c §2.3: "at or above gate_above_used_percent MUST be excluded" |
| Unknown-pressure policy | Never gated, never 0%, never 100%; weight = `unknown_pressure_weight` (default 0.90) + warning; pseudo-band name `"unknown"` reserved | master plan §5.3: "neutral-with-a-warning"; 0% would make unmeasurable providers outrank measured ones |
| Band boundary schema | One bound per tier, `upper_used_percent` only (min is implicit: the previous tier's bound) | plan TOML verbatim (§5.2, annex-d §4.2); keeps the ladder first-tier-match semantics exact |
| Direction config key | `[bands].direction`, values `"spread"` (default) and `"drain"`; drain assigns tier *N* weight `weight[len-1-N]` | master plan §5.2: "settled: both ship, spread is the default" |
| WindowPercent priority chain | Synthetic → skip; Unlimited → 0; UsageKnown false → unknown; UsedPercent → as-is; Used+Limit; Remaining+Limit; else unknown | master plan §5.1 table; `Synthetic`/`UsageKnown` semantics from §3.2 |
| Absent gating windows | A `windowIDs` id missing from the snapshot contributes nothing (not unknown-everything) | optional windows are legitimately absent (annex-a `WindowSpec.Optional`); only when NO gating window computes is pressure unknown |
| Defaults source | `FromTOML` fills unset fields with the plan's TOML defaults | master plan §5.2; F01's `UnmarshalKey` yields zero values for missing keys (Main DECISION B) |
| Validation order | direction → gate → unknown weight → tiers non-empty → ascending bounds → unique names → reserved name → positive weights | fixed order keeps error messages deterministic and golden-testable |
| Decimal conversion | `decimal.NewFromFloat` at the canonical `*float64` boundary; no rounding until F20 | global CONTRACTS §1.4; 25/50/75/100 and the default weights are exactly representable, so boundary comparisons are exact |

## 5. Out of scope

- Strategy scoring (`FinalScore = ModelScore x BandWeight x ProviderWeight`), provider weight, strategies, state files — F20.
- `which-model pick` command wiring, exit codes 3/4, `--max-used-percent`, `--require-live`, candidate exclusion assembly — F26.
- Degraded-mode assembly (band empty, `BandWeight = 1.0`, `[bands]` inert) — F21 (`internal/pick/degraded.go`).
- Usage fetching, caching, confidence/staleness flags — F14/F13; F19 consumes the resulting `Snapshot` as-is.
- `[bands]` TOML decoding itself — F01's generic `Config.UnmarshalKey`; F19 owns the schema and the semantic validation.
- The `which-model usage --fail-on-gated` flag (exit 4) — F24, which may reuse `EvaluateBand`/`ReasonCodeBandGated`.
- Snapshot confidence handling (`--require-live` exclusion of cached/estimated) — F26.
