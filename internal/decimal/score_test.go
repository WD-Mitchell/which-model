package decimal

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Test cases from specs/features/F02-decimal/TASKS.md, Task F02-T3.
// Inputs are written as exact decimal strings (no float64).
func TestScoreString(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"zero", "0", "0"},       // no sign
		{"hundred", "100", "100"},
		{"63.4", "63.4", "63"},
		{"63.5", "63.5", "64"},   // half away from zero
		{"0.5", "0.5", "1"},
		{"-0.4", "-0.4", "0"},     // rounds to zero, no sign
		{"-0.5", "-0.5", "-1"},
		{"99.999", "99.999", "100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScoreString(decimal.RequireFromString(tc.input))
			if got != tc.want {
				t.Errorf("ScoreString(%s) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
