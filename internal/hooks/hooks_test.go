// F29-T1: hook registry tests (specs/features/F29-agent-hooks/TASKS.md
// task F29-T1).
package hooks

import (
	"reflect"
	"testing"
)

// Test 1: Get returns ok=true for each of the four ids; All has exactly 4
// entries in annex-c §3 order.
func TestRegistryAll(t *testing.T) {
	if len(All) != 4 {
		t.Fatalf("len(All) = %d, want 4", len(All))
	}
	wantOrder := []string{"usage-refresh", "quota-guard", "spawn-gate", "model-audit"}
	for i, want := range wantOrder {
		if All[i].ID != want {
			t.Errorf("All[%d].ID = %q, want %q", i, All[i].ID, want)
		}
		h, ok := Get(want)
		if !ok {
			t.Errorf("Get(%q) ok = false, want true", want)
			continue
		}
		if h.ID != want {
			t.Errorf("Get(%q).ID = %q", want, h.ID)
		}
	}
}

// Test 2: Get("nonsense") returns ok=false.
func TestRegistryGetUnknown(t *testing.T) {
	if _, ok := Get("nonsense"); ok {
		t.Error("Get(\"nonsense\") ok = true, want false")
	}
}

// Test 3: (Event, Matcher, Timeout) per hook match the annex-c §3.1–§3.4
// table values.
func TestRegistryTable(t *testing.T) {
	tests := []struct {
		id      string
		event   string
		matcher string
		timeout int
	}{
		{"usage-refresh", "SessionStart", "*", 5},
		{"quota-guard", "SessionStart", "*", 5},
		{"spawn-gate", "PreToolUse", "Task", 8},
		{"model-audit", "PostToolUse", "Task", 5},
	}
	for _, tt := range tests {
		h, ok := Get(tt.id)
		if !ok {
			t.Fatalf("Get(%q) not found", tt.id)
		}
		if h.Event != tt.event {
			t.Errorf("%s: Event = %q, want %q", tt.id, h.Event, tt.event)
		}
		if h.Matcher != tt.matcher {
			t.Errorf("%s: Matcher = %q, want %q", tt.id, h.Matcher, tt.matcher)
		}
		if h.Timeout != tt.timeout {
			t.Errorf("%s: Timeout = %d, want %d", tt.id, h.Timeout, tt.timeout)
		}
	}
}

// Test 4: Underlying argv builders — exact argv with and without env
// overrides. Host env vars are pinned to empty so the default arms are
// deterministic.
func TestRegistryUnderlying(t *testing.T) {
	t.Setenv("WHICH_MODEL_TASK_PROFILE", "")
	t.Setenv("WHICH_MODEL_CANDIDATE_ID", "")

	tests := []struct {
		name string
		hook string
		env  map[string]string
		want []string
	}{
		{
			name: "usage-refresh",
			hook: "usage-refresh",
			want: []string{"usage", "--all", "--json", "--quiet", "--refresh-usage", "--timeout", "5s"},
		},
		{
			name: "quota-guard",
			hook: "quota-guard",
			want: []string{"usage", "--all", "--json", "--band-at-or-above", "critical", "--quiet"},
		},
		{
			name: "spawn-gate profile env",
			hook: "spawn-gate",
			env:  map[string]string{"WHICH_MODEL_TASK_PROFILE": "research"},
			want: []string{"pick", "--profile", "research", "--strategy", "priority", "--json"},
		},
		{
			name: "spawn-gate default profile",
			hook: "spawn-gate",
			want: []string{"pick", "--profile", "balanced_implementation", "--strategy", "priority", "--json"},
		},
		{
			name: "model-audit candidate env",
			hook: "model-audit",
			env:  map[string]string{"WHICH_MODEL_CANDIDATE_ID": "c-1"},
			want: []string{"explain", "--last", "--json"},
		},
		{
			name: "model-audit last",
			hook: "model-audit",
			want: []string{"explain", "--last", "--json"},
		},
	}
	for _, tt := range tests {
		h, ok := Get(tt.hook)
		if !ok {
			t.Fatalf("Get(%q) not found", tt.hook)
		}
		got := h.Underlying(nil, tt.env)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Underlying(nil, %v) = %v, want %v", tt.name, tt.env, got, tt.want)
		}
	}
}

// Test 5: passthrough args append AFTER the defaults (later args win).
func TestRegistryPassthrough(t *testing.T) {
	t.Setenv("WHICH_MODEL_TASK_PROFILE", "")
	h, ok := Get("spawn-gate")
	if !ok {
		t.Fatal("spawn-gate not found")
	}
	got := h.Underlying([]string{"--quiet"}, nil)
	want := []string{"pick", "--profile", "balanced_implementation", "--strategy", "priority", "--json", "--quiet"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spawn-gate.Underlying([\"--quiet\"], nil) = %v, want %v", got, want)
	}
}
