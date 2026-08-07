package decimal

import "testing"

// Test cases from specs/features/F02-decimal/TASKS.md, Task F02-T1.
func TestParse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		err   bool
	}{
		{"zero", "0", "0", false},
		{"fraction", "0.85", "0.85", false},
		{"integer", "100", "100", false},
		{"negative", "-1.5", "-1.5", false},
		{"scientific", "1e3", "1000", false},
		{"empty", "", "", true},
		{"letters", "abc", "", true},
		{"double dot", "1..2", "", true},
		{"two dots", "1.2.3", "", true},
		{"nan", "NaN", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if tc.err {
				if err == nil {
					t.Errorf("Parse(%q) error = nil, want non-nil error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}
			if got.String() != tc.want {
				t.Errorf("Parse(%q) = %q, want %q", tc.input, got.String(), tc.want)
			}
		})
	}
}
