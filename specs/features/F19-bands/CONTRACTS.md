---
kind: feature-contracts
version: "1.0"
feature: F19-bands
project: which-model
---

# F19-bands — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/pick/band/pressure.go` | `Pressure`, `WindowPercent`, `Pressure` constructor |
| `internal/pick/band/bands.go` | `Direction`, `BandSpec`, `Config`, `Result`, `TOMLConfig`, `TOMLTier`, `EvaluateBand`, `ValidateBands`, `FromTOML`, `DefaultConfig`, `ReasonCodeBandGated` |

Import boundary (global CONTRACTS §8): `internal/pick/band` MAY import `internal/usage` (types only) and `github.com/shopspring/decimal`; MUST NOT import `internal/config`, `internal/catalog/*`, `internal/routing`, or `cmd/`. All numeric types are `shopspring/decimal.Decimal`; `float64` appears only at the conversion boundary from the canonical `usage.Window` `*float64` fields and the TOML floats.

## 2. Exported API — `internal/pick/band/pressure.go`

```go
package band

import (
    "github.com/shopspring/decimal"
    "github.com/WD-Mitchell/which-model/internal/usage"
)

// Pressure is the single scalar describing how constrained a route's
// provider is. Known == false means the usage is unmeasurable (SPEC §2.3).
type Pressure struct {
    Known   bool
    Percent decimal.Decimal // meaningful only when Known; may exceed 100
}

// WindowPercent derives the percent used for ONE window, in the priority
// chain (SPEC §2.2):
//   1. Synthetic        -> unknown (placeholder, not a real lane)
//   2. Unlimited        -> 0, known
//   3. UsageKnown false -> unknown (reset metadata, no usage number)
//   4. UsedPercent set  -> as reported (may exceed 100)
//   5. Used + Limit>0   -> Used / Limit * 100
//   6. Remaining+Limit>0-> (Limit - Remaining) / Limit * 100
//   7. otherwise        -> unknown (balance only, or non-positive Limit)
// Float64 conversion uses decimal.NewFromFloat (shortest-representation,
// exact for 25/50/75/100 and the default weights). No rounding here.
func WindowPercent(w usage.Window) (decimal.Decimal, bool)

// Pressure reduces a usage Snapshot to one scalar for a route (SPEC §2.1):
// max over the snapshot windows whose ID is in windowIDs, skipping windows
// whose WindowPercent is unknown. Windows named in windowIDs but absent from
// the snapshot contribute nothing. Returns Known == false when the snapshot
// carries Failure, when windowIDs is empty, or when no gating window yields
// a computable percent.
func Pressure(snapshot usage.Snapshot, windowIDs []string) Pressure
```

## 3. Exported API — `internal/pick/band/bands.go`

```go
// Direction selects how band weights are assigned (SPEC §2.5).
type Direction string

const (
    DirectionSpread Direction = "spread" // default; high consumption LOWERS weight
    DirectionDrain  Direction = "drain"  // high consumption RAISES weight
)

// BandSpec is one tier of the ladder. The min of a tier is implicit: the
// previous tier's UpperUsedPercent (SPEC §2.4, Decisions).
type BandSpec struct {
    Name             string
    UpperUsedPercent decimal.Decimal
    Weight           decimal.Decimal
}

// Config is the validated [bands] configuration.
type Config struct {
    Direction             Direction
    GateAboveUsedPercent  decimal.Decimal
    UnknownPressureWeight decimal.Decimal
    Tiers                 []BandSpec // strictly ascending UpperUsedPercent
}

// Result of one band evaluation. Name is "" when Gated, "unknown" when the
// pressure is unknown (SPEC §2.7).
type Result struct {
    Name    string
    Weight  decimal.Decimal // 0 when Gated; UnknownPressureWeight when unknown
    Gated   bool
    Warning string // exact strings in §4; "" when none
}

// ReasonCodeBandGated is the exclusion reason code for a gated candidate
// (docs/plan/annex-c-agent-integration.md §4.2 enum). Consumers MUST use
// this constant, never a hand-written string.
const ReasonCodeBandGated = "band_gated"

// EvaluateBand maps one pressure to a band result (total function, never
// errors; SPEC §3):
//   - p.Known && p.Percent >= cfg.GateAboveUsedPercent  -> Gated (SPEC §2.6)
//   - !p.Known                                           -> Name "unknown",
//     Weight = cfg.UnknownPressureWeight, warning emitted (SPEC §2.7)
//   - otherwise -> first tier (ascending) with UpperUsedPercent >= p.Percent;
//     pressure above the last bound clamps to the last tier (SPEC §2.4)
// Precondition: cfg is validated (ValidateBands). Band NAMING follows the
// pressure tier; direction only chooses the weight: spread -> declared
// weight, drain -> tier N takes weight[len-1-N] (SPEC §2.5).
func EvaluateBand(p Pressure, cfg Config) Result

// ValidateBands checks, in this exact order (SPEC §2.8, Decisions):
// direction value; gate >= 0; unknown_pressure_weight > 0; tiers non-empty;
// UpperUsedPercent strictly ascending; tier names unique; no tier named
// "unknown"; every tier weight > 0. Error strings are exact (§5).
func ValidateBands(cfg Config) error

// DefaultConfig returns the plan defaults (SPEC §2.9): direction "spread",
// gate 98, unknown_pressure_weight 0.90, and the ladder
// low/25/1.00, standard/50/0.85, elevated/75/0.60, critical/100/0.25.
func DefaultConfig() Config

// TOMLConfig mirrors the [bands] TOML section; F01's generic
// Config.UnmarshalKey("bands", &raw) decodes into it (Main DECISION B;
// F01 §1.7 overlay). Pointer scalars distinguish "unset" (default applies)
// from zero. direction is *string; the two numeric scalars are
// *decimal.Decimal — shopspring implements encoding.TextUnmarshaler, which
// BurntSushi/toml uses to decode TOML floats and F01's env overlay uses to
// apply WHICH_MODEL_BANDS_* overrides (F01 SPEC B12: no float64 on the
// numeric path). TOMLTier values are plain decimal.Decimal: the tier array
// is not env-addressable and each entry is fully specified.
type TOMLConfig struct {
    Direction             *string          `toml:"direction"`
    GateAboveUsedPercent  *decimal.Decimal `toml:"gate_above_used_percent"`
    UnknownPressureWeight *decimal.Decimal `toml:"unknown_pressure_weight"`
    Tiers                 []TOMLTier       `toml:"tier"`
}

type TOMLTier struct {
    Name             string          `toml:"name"`
    UpperUsedPercent decimal.Decimal `toml:"upper_used_percent"`
    Weight           decimal.Decimal `toml:"weight"`
}

// FromTOML converts the decoded TOML section into a validated Config: unset
// scalar fields take DefaultConfig values; a nil Tiers takes the default
// ladder; a present-but-empty Tiers is a validation error. The numeric
// fields arrive as decimal.Decimal already (TOML decode via TextUnmarshaler,
// F01 env overlay via TextUnmarshaler); FromTOML only dereferences pointers
// and never introduces float64. Returns the ValidateBands error, if any.
func FromTOML(t TOMLConfig) (Config, error)
```

## 4. Warning string (exact)

| Where | String |
|---|---|
| `Result.Warning` when pressure is unknown | `pressure unknown for route; using unknown_pressure_weight` |

## 5. Validation error strings (exact, checked in this order)

| # | String |
|---|---|
| 1 | `bands: direction must be "spread" or "drain"` |
| 2 | `bands: gate_above_used_percent must not be negative` |
| 3 | `bands: unknown_pressure_weight must be positive` |
| 4 | `bands: tiers must not be empty` |
| 5 | `bands: tier %d upper_used_percent %s must be greater than the previous bound %s` (1-indexed tier position; decimal strings) |
| 6 | `bands: tier names must be unique` |
| 7 | `bands: tier name "unknown" is reserved` |
| 8 | `bands: tier %q weight must be positive` (tier name) |

## 6. Config keys owned

Section `[bands]` (TOML, plan §5.2 verbatim; decoded by F01 `Config.UnmarshalKey("bands", &raw)`):

| Key | Default | Meaning |
|---|---|---|
| `bands.direction` | `"spread"` | `"spread"` high consumption lowers weight; `"drain"` raises it |
| `bands.gate_above_used_percent` | `98` | hard exclusion: known pressure `>=` this value gates the candidate (SPEC §2.6) |
| `bands.unknown_pressure_weight` | `0.90` | weight applied when pressure is unknown (SPEC §2.7) |
| `bands.tier[]` (array of tables) | four-tier ladder (SPEC §2.9) | each entry: `name` (string), `upper_used_percent` (float), `weight` (float) |

Environment overrides (F01 §3 closed env-key vocabulary; annex-d §4.4): `WHICH_MODEL_BANDS_DIRECTION` → `bands.direction`; `WHICH_MODEL_BANDS_GATE_ABOVE_USED_PERCENT` → `bands.gate_above_used_percent`. F01's overlay allocates a nil `*decimal.Decimal` on override and parses the value via TextUnmarshaler, so the unset-vs-zero semantics survive. `unknown_pressure_weight` and `[[bands.tier]]` entries are not env-addressable; override those via `--band-config <path>` (annex-d §2.5, F26). Unknown `WHICH_MODEL_*` vars fail eagerly at F01 `ApplyEnv` (KindInvalidValue).

## 7. Reason codes and error codes

- **Reason code owned:** `ReasonCodeBandGated = "band_gated"` (§3). Other `ExcludedCandidate.reason_code` values (`no_score_row`, `auth_required`, `provider_error`, `not_in_availability_list`) belong to F26.
- **Exit codes added:** none. `ValidateBands`/`FromTOML` errors are configuration errors surfaced as exit 2 by the caller (F01/F26 wiring); all-gated results surface as exit 4 via F26 (global SPEC §5).
- **JSON shapes emitted:** none — F19 returns Go values only; JSON rendering is F26's output layer.

## 8. External symbols referenced (cross-feature)

| Symbol | Source | Used by |
|---|---|---|
| `usage.Snapshot{Provider, Windows []Window, Failure *Failure, ...}` | `specs/global/CONTRACTS.md §1.5`, file `internal/usage/types.go` (F11) | `Pressure` |
| `usage.Window{ID, UsedPercent *float64, Used *float64, Limit *float64, Remaining *float64, Unlimited bool, ModelScope []string, Synthetic bool, UsageKnown bool}` | `specs/global/CONTRACTS.md §1.4`, file `internal/usage/types.go` (F11) | `WindowPercent` |
| `config.Config.UnmarshalKey(key string, out any) error` | `specs/features/F01-config/CONTRACTS.md` (Main DECISION B) | wiring only — F19 never imports `internal/config`; the caller decodes into `TOMLConfig` and calls `FromTOML` |
| `decimal.NewFromFloat`, `decimal.Decimal.Cmp/Round`, `decimal.Decimal.UnmarshalText` (TextUnmarshaler) | `github.com/shopspring/decimal` (global contract: shopspring everywhere scores are computed) | all numeric paths; no rounding in F19 (rounding is F20's); `UnmarshalText` is the TOML/env decode hook (F01 SPEC B12) |

## 9. Notes for consumers

- F20 (`FinalScore = ModelScore x BandWeight x ProviderWeight`) consumes `EvaluateBand(...).Weight`; F26 assembles `pick.Candidate` (global CONTRACTS §4.1: `Band`, `BandWeight`) and maps `Result.Gated` to an exclusion with `ReasonCodeBandGated` (exit 4 when all candidates are gated).
- F24's `which-model usage --fail-on-gated` may reuse `EvaluateBand`/`DefaultConfig` (annex-d §2.4).
- F21 owns degraded assembly (band empty, `BandWeight = 1.0`, `[bands]` inert) in `internal/pick/degraded.go`; F19 stays out of it.
- F19 compiles under `-tags nousage` unchanged (SPEC §5).
