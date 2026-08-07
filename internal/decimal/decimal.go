package decimal

import "github.com/shopspring/decimal"

// Parse wraps decimal.NewFromString unchanged. No float64 conversion.
func Parse(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}
// RoundHalfUp rounds half away from zero (Python ROUND_HALF_UP,
// annex-b §1.1); places may be negative. Equivalent to d.Round(places).
func RoundHalfUp(d decimal.Decimal, places int32) decimal.Decimal {
	return d.Round(places)
}

// ScoreString renders d as the canonical integer score: nearest integer,
// half away from zero, no sign on zero.
func ScoreString(d decimal.Decimal) string {
	return RoundHalfUp(d, 0).StringFixed(0)
}
// WeightedMean computes Σ(cᵢ·wᵢ)/Σ(wᵢ) over entries with weight > 0 at
// full precision (caller rounds). components and weights are parallel
// slices; entries with weight <= 0 are skipped. The bool is false when
// no valid component exists (length mismatch, both empty, all weights
// <= 0) — then the value is decimal.Zero. Never returns an error.
func WeightedMean(components, weights []decimal.Decimal) (decimal.Decimal, bool) {
	if len(components) != len(weights) {
		return decimal.Zero, false
	}
	num := decimal.Zero
	den := decimal.Zero
	for i, w := range weights {
		if w.Sign() <= 0 { // skip zero and negative weights
			continue
		}
		num = num.Add(components[i].Mul(w))
		den = den.Add(w)
	}
	if den.IsZero() {
		return decimal.Zero, false
	}
	return num.Div(den), true
}
func init() {
	// Full-precision division: 34 fractional digits (spec goldens:
	// F02-T4 case 12 and F10 tier2 quotients, e.g. 655/7). Callers
	// round via RoundHalfUp; never rely on shopspring's default 16.
	decimal.DivisionPrecision = 34
}