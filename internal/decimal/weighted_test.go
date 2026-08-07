package decimal

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Test cases from specs/features/F02-decimal/TASKS.md, Task F02-T4.
// Inputs are written as exact decimal strings (no float64).
func TestWeightedMean(t *testing.T) {
	cases := []struct {
		name       string
		components []string
		weights    []string
		want       string
		wantOK     bool
	}{
		{"weighted pair", []string{"10", "20"}, []string{"3", "1"}, "12.5", true},
		{"weighted triple", []string{"1", "2", "3"}, []string{"1", "1", "2"}, "2.25", true},
		{"zero weight skipped", []string{"10", "20"}, []string{"0", "1"}, "20", true},
		{"negative weight skipped", []string{"10", "20"}, []string{"1", "-1"}, "10", true},
		{"single weighted component", []string{"10"}, []string{"2"}, "10", true},
		{"single zero weight", []string{"100"}, []string{"0"}, "0", false},
		{"all zero weights", []string{"10", "20"}, []string{"0", "0"}, "0", false},
		{"both empty", []string{}, []string{}, "0", false},
		{"components shorter", []string{"10", "20"}, []string{"1"}, "0", false},
		{"weights shorter", []string{"10"}, []string{"1", "1"}, "0", false},
		{"alternating zero weights", []string{"1", "2", "3", "4"}, []string{"0", "2", "0", "2"}, "3", true},
		{"full precision quotient", []string{"1", "2"}, []string{"1", "2"}, "1.6666666666666666666666666666666667", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			components := make([]decimal.Decimal, len(tc.components))
			for i, value := range tc.components {
				components[i] = decimal.RequireFromString(value)
			}
			weights := make([]decimal.Decimal, len(tc.weights))
			for i, value := range tc.weights {
				weights[i] = decimal.RequireFromString(value)
			}

			got, gotOK := WeightedMean(components, weights)
			if gotOK != tc.wantOK {
				t.Errorf("WeightedMean(%v, %v) ok = %v, want %v", tc.components, tc.weights, gotOK, tc.wantOK)
			}
			if got.String() != tc.want {
				t.Errorf("WeightedMean(%v, %v) = %q, want %q", tc.components, tc.weights, got.String(), tc.want)
			}
		})
	}
}
