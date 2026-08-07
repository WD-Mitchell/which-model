package decimal

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Test cases from specs/features/F02-decimal/TASKS.md, Task F02-T2.
// Inputs are written as exact decimal strings (no float64).
func TestRoundHalfUp(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		places int32
		want   string
	}{
		{"0.5 at 0", "0.5", 0, "1"},     // tie rounds away from zero, annex-b §1.1
		{"1.5 at 0", "1.5", 0, "2"},     //
		{"2.5 at 0", "2.5", 0, "3"},     //
		{"-0.5 at 0", "-0.5", 0, "-1"},  //
		{"-1.5 at 0", "-1.5", 0, "-2"},  //
		{"63.4 at 0", "63.4", 0, "63"},  //
		{"63.5 at 0", "63.5", 0, "64"},  //
		{"2.25 at 1", "2.25", 1, "2.3"}, //
		{"-2.25 at 1", "-2.25", 1, "-2.3"},
		{"0.05 at 1", "0.05", 1, "0.1"},
		{"0.45 at 1", "0.45", 1, "0.5"},
		{"125 at -1", "125", -1, "130"}, // negative places pass through
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RoundHalfUp(decimal.RequireFromString(tc.input), tc.places)
			if got.String() != tc.want {
				t.Errorf("RoundHalfUp(%s, %d) = %q, want %q", tc.input, tc.places, got.String(), tc.want)
			}
		})
	}
}
