// Issue #37: zero-positional commands must reject stray positional arguments
// with exit 2 (annex-d §1.4), not silently ignore them. One table-driven test
// walks every affected command through the real ExecuteArgs path.
package whichmodel

import "testing"

func TestZeroPositionalCommandsRejectStrayArgs(t *testing.T) {
	// pick/explain fail on missing selectors BEFORE args reach RunE only if
	// the cobra Args validator rejects first; these cases assert exit 2 with
	// the cobra "unknown command" text for the stray positional.
	cases := []struct {
		name string
		args []string
	}{
		{"version", []string{"version", "junk"}},
		{"pick", []string{"pick", "--profile", "complex_implementation", "junk"}},
		{"explain", []string{"explain", "--last", "junk"}},
		{"config show", []string{"config", "show", "junk"}},
		{"config path", []string{"config", "path", "junk"}},
		{"config validate", []string{"config", "validate", "junk"}},
		{"catalog refresh", []string{"catalog", "refresh", "junk"}},
		{"catalog benchmarks", []string{"catalog", "benchmarks", "junk"}},
		{"catalog scores", []string{"catalog", "scores", "junk"}},
		{"hooks install", []string{"hooks", "install", "junk"}},
		{"hooks remove", []string{"hooks", "remove", "junk"}},
		{"serve", []string{"serve", "junk"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := captureExecuteFresh(t, tc.args)
			if code != 2 {
				t.Errorf("ExecuteArgs(%v) exit = %d, want 2 (stray positional must be rejected)", tc.args, code)
			}
		})
	}
}
