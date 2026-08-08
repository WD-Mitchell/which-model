package score

import (
	sdecimal "github.com/shopspring/decimal"
)

// Normalizer maps a raw metric value onto a relative 0-100 score given the
// column's min/max (specs/global/CONTRACTS.md §2.2, verbatim). The interface
// carries no direction parameter: lower-is-better columns are reflected by
// directionAdjust before Normalize.
type Normalizer interface {
	Normalize(raw sdecimal.Decimal, min, max sdecimal.Decimal) sdecimal.Decimal
}

// Aggregator combines weighted component scores into one (specs/global/
// CONTRACTS.md §2.2, verbatim). The bool is false when no components exist.
type Aggregator interface {
	Aggregate(values []sdecimal.Decimal, weights []sdecimal.Decimal) (sdecimal.Decimal, bool)
}

// Name constants for the config vocabulary (CONTRACTS §1.1).
const (
	NormalizerNameMinMaxLinear           string = "minmax-linear"
	AggregatorNameWeightedArithmeticMean string = "weighted-arithmetic-mean"
)

// Compile-time conformance: the concrete implementations satisfy the
// interfaces used by the derive/composite layers.
var (
	_ Normalizer = MinMaxLinear{}
	_ Aggregator = WeightedArithmeticMean{}
)

// DefaultNormalizer returns the canonical normalizer instance.
func DefaultNormalizer() Normalizer { return MinMaxLinear{} }

// DefaultAggregator returns the canonical aggregator instance.
func DefaultAggregator() Aggregator { return WeightedArithmeticMean{} }

// ResolveNormalizer maps a config name to a normalizer; unknown names return
// *Error{Code: ErrUnknownNormalizer} with message `unknown normalizer: <name>`.
func ResolveNormalizer(name string) (Normalizer, error) {
	if name == NormalizerNameMinMaxLinear {
		return MinMaxLinear{}, nil
	}
	return nil, &Error{Code: ErrUnknownNormalizer, Message: "unknown normalizer: " + name}
}

// ResolveAggregator maps a config name to an aggregator; unknown names return
// *Error{Code: ErrUnknownAggregator} with message `unknown aggregator: <name>`.
func ResolveAggregator(name string) (Aggregator, error) {
	if name == AggregatorNameWeightedArithmeticMean {
		return WeightedArithmeticMean{}, nil
	}
	return nil, &Error{Code: ErrUnknownAggregator, Message: "unknown aggregator: " + name}
}

// ScoringConfig is the [scoring] TOML section owned by F09 (read by F23 via
// cfg.UnmarshalKey("scoring", &c) with c pre-set to DefaultScoringConfig()).
type ScoringConfig struct {
	Normalizer string `toml:"normalizer"`
	Aggregator string `toml:"aggregator"`
}

// DefaultScoringConfig returns the [scoring] defaults (CONTRACTS §2.4).
func DefaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		Normalizer: NormalizerNameMinMaxLinear,
		Aggregator: AggregatorNameWeightedArithmeticMean,
	}
}
