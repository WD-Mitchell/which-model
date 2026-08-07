package identity

import "testing"

func TestParseEffort(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantLevel string
		wantOK    bool
	}{
		{"reasoning effort xhigh", "reasoning effort xhigh", "xhigh", true},
		{"reasoning effort none", "reasoning effort none", "default", true},
		{"high with tools suffix", "high, with tools", "high", true},
		{"case-insensitive HIGH", "HIGH", "high", true},
		{"empty variant", "", "", false},
		{"minimal effort", "minimal effort", "minimal", true},
		{"medium reasoning", "medium reasoning", "medium", true},
		{"xhigh context compaction", "xhigh, context compaction", "xhigh", true},
		{"low effort with tools", "low effort, with tools", "low", true},
		{"extra high not a level", "extra high", "", false},
		{"deep thinking not effort", "deep thinking", "", false},
		{"underscore dash normalization", "reasoning_effort-max", "max", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, ok := ParseEffort(tc.input)
			if level != tc.wantLevel || ok != tc.wantOK {
				t.Errorf("ParseEffort(%q) = (%q, %v), want (%q, %v)", tc.input, level, ok, tc.wantLevel, tc.wantOK)
			}
		})
	}
	// Bare ladder word coverage folded per T3 instruction 17.
	if level, ok := ParseEffort("max"); level != "max" || !ok {
		t.Errorf("ParseEffort(\"max\") = (%q, %v), want (\"max\", true)", level, ok)
	}
}
