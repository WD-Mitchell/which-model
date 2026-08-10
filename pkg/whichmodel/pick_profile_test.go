// F26-T2: profile resolution tests
// (specs/features/F26-cmd-pick/TASKS.md task F26-T2).
package whichmodel

import (
	"errors"
	"strings"
	"testing"
)

// setStrategyNames injects the strategy registry-name seam for one test.
func setStrategyNames(t *testing.T, names []string) {
	t.Helper()
	orig := strategyNamesFunc
	strategyNamesFunc = func() []string { return names }
	t.Cleanup(func() { strategyNamesFunc = orig })
}

// F26-T2 row 1: the 11 annex-c §2.1 profile names resolve; unknown profiles
// error with the exact verbatim list.
func TestResolveProfileNames(t *testing.T) {
	if got, err := resolveProfile(PickArgs{Profile: "complex_implementation"}); err != nil || got != "complex_implementation" {
		t.Fatalf("resolveProfile(complex_implementation) = %q, %v; want complex_implementation, nil", got, err)
	}
	wantList := "simple_implementation, simple_action_execution, balanced_implementation, complex_implementation, ui_ux, complex_action_execution, financial_work, research, planning, orchestration, review"
	_, err := resolveProfile(PickArgs{Profile: "bogus"})
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err = %v, want *UsageError", err)
	}
	want := `unknown profile "bogus"; valid: ` + wantList
	if err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
}

// F26-T2 row 2: the category mapping table (7 mapped rows + six 1:1 rows).
func TestResolveProfileCategoryMapping(t *testing.T) {
	cases := []struct {
		category, complexity, want string
	}{
		{"implementation", "simple", "simple_implementation"},
		{"implementation", "medium", "balanced_implementation"},
		{"implementation", "complex", "complex_implementation"},
		{"action_execution", "simple", "simple_action_execution"},
		{"action_execution", "medium", "balanced_implementation"},
		{"action_execution", "complex", "complex_action_execution"},
		{"ui_ux", "", "ui_ux"},
		{"financial_work", "", "financial_work"},
		{"research", "", "research"},
		{"planning", "", "planning"},
		{"orchestration", "", "orchestration"},
		{"review", "", "review"},
	}
	for _, tc := range cases {
		got, err := resolveProfile(PickArgs{TaskCategory: tc.category, Complexity: tc.complexity})
		if err != nil || got != tc.want {
			t.Errorf("resolveProfile(%q, %q) = %q, %v; want %q, nil", tc.category, tc.complexity, got, err, tc.want)
		}
	}
}

// F26-T2 row 3: rejection rows — complexity on a 1:1 category, unknown
// complexity, unknown category.
func TestResolveProfileRejections(t *testing.T) {
	cases := []struct {
		category, complexity, want string
	}{
		{"ui_ux", "simple", `--complexity is not valid for task category "ui_ux"`},
		{"implementation", "hard", `unknown complexity "hard"`},
		{"coding", "simple", `unknown task category "coding"`},
	}
	for _, tc := range cases {
		_, err := resolveProfile(PickArgs{TaskCategory: tc.category, Complexity: tc.complexity})
		if err == nil || err.Error() != tc.want {
			t.Errorf("resolveProfile(%q, %q) error = %v, want %q", tc.category, tc.complexity, err, tc.want)
		}
	}
}

// F26-T2 row 4: strategy validation against the injected names list.
func TestValidateStrategy(t *testing.T) {
	names := []string{"priority", "round-robin", "least-used", "most-used", "closest-to-reset"}
	for _, valid := range names {
		if err := validateStrategy(valid, names); err != nil {
			t.Errorf("validateStrategy(%q) = %v, want nil", valid, err)
		}
	}
	err := validateStrategy("score", names)
	if err == nil || err.Error() != `unknown strategy "score"; valid: priority, round-robin, least-used, most-used, closest-to-reset` {
		t.Errorf("validateStrategy(score) = %v, want unknown strategy error", err)
	}
}

// F26-T2: the 11-name list is exposed verbatim for the error message.
func TestValidProfilesList(t *testing.T) {
	want := "simple_implementation, simple_action_execution, balanced_implementation, complex_implementation, ui_ux, complex_action_execution, financial_work, research, planning, orchestration, review"
	if got := strings.Join(validProfiles, ", "); got != want {
		t.Fatalf("validProfiles = %q, want %q", got, want)
	}
}
