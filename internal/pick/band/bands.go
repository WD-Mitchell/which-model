package band

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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

// ValidateBands checks, in this exact order (SPEC §2.8, Decisions):
// direction value; gate >= 0; unknown_pressure_weight > 0; tiers non-empty;
// UpperUsedPercent strictly ascending; tier names unique; no tier named
// "unknown"; every tier weight > 0. Error strings are exact (§5).
func ValidateBands(cfg Config) error {
	if cfg.Direction != DirectionSpread && cfg.Direction != DirectionDrain {
		return fmt.Errorf(`bands: direction must be "spread" or "drain"`)
	}
	if cfg.GateAboveUsedPercent.LessThan(decimal.Zero) {
		return fmt.Errorf("bands: gate_above_used_percent must not be negative")
	}
	if cfg.UnknownPressureWeight.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("bands: unknown_pressure_weight must be positive")
	}
	if len(cfg.Tiers) == 0 {
		return fmt.Errorf("bands: tiers must not be empty")
	}
	for i := 1; i < len(cfg.Tiers); i++ {
		if cfg.Tiers[i].UpperUsedPercent.LessThanOrEqual(cfg.Tiers[i-1].UpperUsedPercent) {
			return fmt.Errorf("bands: tier %d upper_used_percent %s must be greater than the previous bound %s",
				i+1, cfg.Tiers[i].UpperUsedPercent, cfg.Tiers[i-1].UpperUsedPercent)
		}
	}
	seen := make(map[string]struct{}, len(cfg.Tiers))
	for _, tier := range cfg.Tiers {
		if _, dup := seen[tier.Name]; dup {
			return fmt.Errorf("bands: tier names must be unique")
		}
		seen[tier.Name] = struct{}{}
	}
	for _, tier := range cfg.Tiers {
		if tier.Name == "unknown" {
			return fmt.Errorf(`bands: tier name "unknown" is reserved`)
		}
	}
	for _, tier := range cfg.Tiers {
		if tier.Weight.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("bands: tier %q weight must be positive", tier.Name)
		}
	}
	return nil
}

// DefaultConfig returns the plan defaults (SPEC §2.9): direction "spread",
// gate 98, unknown_pressure_weight 0.90, and the ladder
// low/25/1.00, standard/50/0.85, elevated/75/0.60, critical/100/0.25.
func DefaultConfig() Config {
	return Config{
		Direction:             DirectionSpread,
		GateAboveUsedPercent:  decimal.NewFromFloat(98),
		UnknownPressureWeight: decimal.NewFromFloat(0.90),
		Tiers: []BandSpec{
			{Name: "low", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1.00)},
			{Name: "standard", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(0.85)},
			{Name: "elevated", UpperUsedPercent: decimal.NewFromFloat(75), Weight: decimal.NewFromFloat(0.60)},
			{Name: "critical", UpperUsedPercent: decimal.NewFromFloat(100), Weight: decimal.NewFromFloat(0.25)},
		},
	}
}

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
func FromTOML(t TOMLConfig) (Config, error) {
	cfg := DefaultConfig()
	if t.Direction != nil {
		cfg.Direction = Direction(*t.Direction)
	}
	if t.GateAboveUsedPercent != nil {
		cfg.GateAboveUsedPercent = *t.GateAboveUsedPercent
	}
	if t.UnknownPressureWeight != nil {
		cfg.UnknownPressureWeight = *t.UnknownPressureWeight
	}
	if t.Tiers != nil {
		tiers := make([]BandSpec, len(t.Tiers))
		for i := range t.Tiers {
			tiers[i] = BandSpec{
				Name:             t.Tiers[i].Name,
				UpperUsedPercent: t.Tiers[i].UpperUsedPercent,
				Weight:           t.Tiers[i].Weight,
			}
		}
		cfg.Tiers = tiers
	}
	return cfg, ValidateBands(cfg)
}

// EvaluateBand maps one pressure to a band result (total function, never
// errors; SPEC §3):
//   - p.Known && p.Percent >= cfg.GateAboveUsedPercent  -> Gated (SPEC §2.6)
//   - !p.Known                                           -> Name "unknown",
//     Weight = cfg.UnknownPressureWeight, warning emitted (SPEC §2.7)
//   - otherwise -> first tier (ascending) with UpperUsedPercent >= p.Percent;
//     pressure above the last bound clamps to the last tier (SPEC §2.4;
//     upper bound INCLUSIVE per DEFERRED D8)
// Precondition: cfg is validated (ValidateBands). Band NAMING follows the
// pressure tier; direction only chooses the weight: spread -> declared
// weight, drain -> tier N takes weight[len-1-N] (SPEC §2.5).
func EvaluateBand(p Pressure, cfg Config) Result {
	if p.Known && p.Percent.GreaterThanOrEqual(cfg.GateAboveUsedPercent) {
		return Result{Name: "", Weight: decimal.Zero, Gated: true}
	}
	if !p.Known {
		return Result{
			Name:    "unknown",
			Weight:  cfg.UnknownPressureWeight,
			Gated:   false,
			Warning: "pressure unknown for route; using unknown_pressure_weight",
		}
	}
	// First tier whose bound is >= p; p below the first bound lands in
	// tier 0; p above the last bound clamps to the last tier.
	i := 0
	for ; i < len(cfg.Tiers); i++ {
		if cfg.Tiers[i].UpperUsedPercent.GreaterThanOrEqual(p.Percent) {
			break
		}
	}
	if i == len(cfg.Tiers) {
		i = len(cfg.Tiers) - 1 // clamp above the last bound
	}
	name := cfg.Tiers[i].Name
	weight := cfg.Tiers[i].Weight
	if cfg.Direction == DirectionDrain {
		weight = cfg.Tiers[len(cfg.Tiers)-1-i].Weight
	}
	return Result{Name: name, Weight: weight, Gated: false, Warning: ""}
}
