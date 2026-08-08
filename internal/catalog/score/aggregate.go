package score

import (
	sdecimal "github.com/shopspring/decimal"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// WeightedArithmeticMean: round(Σ(v*w)/Σw, 0); weights may have any scale
// (renormalized by construction); returns false when len(values) == 0.
type WeightedArithmeticMean struct{}

// Aggregate computes the weighted arithmetic mean via internal/decimal
// WeightedMean (F02 pin) and rounds ROUND_HALF_UP to an integer.
func (WeightedArithmeticMean) Aggregate(values, weights []sdecimal.Decimal) (sdecimal.Decimal, bool) {
	mean, ok := wdecimal.WeightedMean(values, weights)
	if !ok {
		return sdecimal.Zero, false
	}
	return wdecimal.RoundHalfUp(mean, 0), true
}
