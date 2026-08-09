// F26-T1: pick command skeleton tests
// (specs/features/F26-cmd-pick/TASKS.md task F26-T1).
package whichmodel

import (
	"errors"
	"strings"
	"testing"
)

// F26-T1 row 1: registeredCommands() contains pick.
func TestPickCommandRegistered(t *testing.T) {
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "pick" {
			return
		}
	}
	t.Fatal("registeredCommands() does not contain pick")
}

// F26-T1 row 2: NewPickCmd() flags — exact names, types, defaults.
func TestPickCommandShape(t *testing.T) {
	cmd := NewPickCmd()
	if cmd.Use != "pick" {
		t.Fatalf("Use = %q, want pick", cmd.Use)
	}
	routesCheckFlag(t, cmd, "profile", "string", "")
	routesCheckFlag(t, cmd, "task-category", "string", "")
	routesCheckFlag(t, cmd, "complexity", "string", "")
	routesCheckFlag(t, cmd, "strategy", "string", "score")
	routesCheckFlag(t, cmd, "seed", "uint64", "0")
	routesCheckFlag(t, cmd, "available", "stringSlice", "[]")
}

// F26-T1 row 3: exit-code registrations — no_pick → 3, usage_gated → 4,
// auth_required → 5.
func TestPickExitCodeRegistrations(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{"no_pick", 3},
		{"usage_gated", 4},
		{"auth_required", 5},
	}
	for _, tc := range cases {
		if got := ExitCodeFor(&CodedError{Code: tc.code}); got != tc.want {
			t.Errorf("ExitCodeFor(CodedError{Code: %q}) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

// F26-T1 row 4: no selector flags → UsageError, exit 2.
func TestPickRunERequiresSelector(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	cmd := NewPickCmd()
	cmd.SetArgs([]string{"--strategy", "score"})
	err := cmd.Execute()
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err = %v, want *UsageError (exit %d)", err, ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "--profile or --task-category is required") {
		t.Fatalf("message = %q, want it to contain %q", err.Error(), "--profile or --task-category is required")
	}
	if ExitCodeFor(err) != 2 {
		t.Fatalf("exit = %d, want 2", ExitCodeFor(err))
	}
}

// F26-T1 row 5: --profile + --task-category → UsageError, mutually exclusive.
func TestPickRunEProfileAndCategory(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	cmd := NewPickCmd()
	cmd.SetArgs([]string{"--profile", "complex_implementation", "--task-category", "implementation"})
	err := cmd.Execute()
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err = %v, want *UsageError (exit %d)", err, ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("message = %q, want it to contain %q", err.Error(), "mutually exclusive")
	}
	if ExitCodeFor(err) != 2 {
		t.Fatalf("exit = %d, want 2", ExitCodeFor(err))
	}
}

// F26-T1 row 6: --task-category without --complexity → UsageError.
func TestPickRunECategoryWithoutComplexity(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	cmd := NewPickCmd()
	cmd.SetArgs([]string{"--task-category", "implementation"})
	err := cmd.Execute()
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err = %v, want *UsageError (exit %d)", err, ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "must be given together") {
		t.Fatalf("message = %q, want it to contain %q", err.Error(), "must be given together")
	}
	if ExitCodeFor(err) != 2 {
		t.Fatalf("exit = %d, want 2", ExitCodeFor(err))
	}
}
