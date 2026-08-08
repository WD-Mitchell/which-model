package score

import (
	sdecimal "github.com/shopspring/decimal"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

var hundred = sdecimal.NewFromInt(100)

// MinMaxLinear: score = round(((raw - min) / (max - min)) * 100, 0); caller
// guarantees min != max (degenerate ranges are handled at column level) and
// min <= raw <= max. The global Normalizer interface carries no direction
// parameter, so lower-is-better metrics (time, cost, median) are
// direction-adjusted by the derive layer BEFORE Normalize via the exact
// reflection v' = min + max - v, which yields (max - v)/(max - min)
// (generate_scores.py normalized_score).
type MinMaxLinear struct{}

// Normalize computes the min-max linear score with ROUND_HALF_UP at 0 places.
func (MinMaxLinear) Normalize(raw, min, max sdecimal.Decimal) sdecimal.Decimal {
	return wdecimal.RoundHalfUp(raw.Sub(min).Div(max.Sub(min)).Mul(hundred), 0)
}

// directionAdjust reflects a lower-is-better raw value so MinMaxLinear can
// treat every column as higher-is-better: v' = min + max - v for
// lower-is-better, v unchanged for higher-is-better.
func directionAdjust(raw, min, max sdecimal.Decimal, higherIsBetter bool) sdecimal.Decimal {
	if higherIsBetter {
		return raw
	}
	return min.Add(max).Sub(raw)
}
