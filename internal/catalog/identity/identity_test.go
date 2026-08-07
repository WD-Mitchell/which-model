package identity

import "testing"

func TestCollapseReasoning(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"default", "high"},
		{"minimal", "minimal"},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"xhigh", "xhigh"},
		{"max", "max"},
		{"", ""},
		{"adaptive", "adaptive"},
	}
	for _, tc := range cases {
		t.Run("level="+tc.input, func(t *testing.T) {
			if got := CollapseReasoning(tc.input); got != tc.want {
				t.Errorf("CollapseReasoning(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if EffortOrder["minimal"] != 0 {
		t.Errorf("EffortOrder[minimal] = %d, want 0", EffortOrder["minimal"])
	}
	if EffortOrder["high"] != 3 {
		t.Errorf("EffortOrder[high] = %d, want 3", EffortOrder["high"])
	}
	if EffortOrder["max"] != 5 {
		t.Errorf("EffortOrder[max] = %d, want 5", EffortOrder["max"])
	}
	if len(EffortOrder) != 6 {
		t.Errorf("len(EffortOrder) = %d, want 6", len(EffortOrder))
	}
	if len(ReasoningLevels) != 7 {
		t.Errorf("len(ReasoningLevels) = %d, want 7", len(ReasoningLevels))
	}
	if _, ok := ReasoningLevels["default"]; !ok {
		t.Error("ReasoningLevels must contain default")
	}
	if _, ok := ReasoningLevels["thinking"]; ok {
		t.Error("ReasoningLevels must not contain thinking")
	}
}

func TestIdentityKey(t *testing.T) {
	a := IdentityKey("Example", "default")
	b := IdentityKey("Example", "high")
	want := Identity{Model: "Example", Reasoning: "high"}
	if a != b {
		t.Errorf("IdentityKey(Example, default) = %v != IdentityKey(Example, high) = %v", a, b)
	}
	if a != want {
		t.Errorf("IdentityKey(Example, default) = %v, want %v", a, want)
	}
	if got := a == b && a == want; !got {
		t.Error("struct equality across default/high collapse failed")
	}

	low := IdentityKey("GPT-5.6 Sol", "low")
	high := IdentityKey("GPT-5.6 Sol", "high")
	if low == high {
		t.Errorf("low and high must be distinct identities: %v == %v", low, high)
	}

	annotated := IdentityKey("Claude Opus 4.5 [claude-opus-4-5-20251101]", "default")
	wantAnnotated := Identity{Model: "Claude Opus 4.5", Reasoning: "high"}
	if annotated != wantAnnotated {
		t.Errorf("IdentityKey(annotated, default) = %v, want %v", annotated, wantAnnotated)
	}
}
