package identity

import "testing"

func TestCleanModelName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"annotated bracket id", "Claude Opus 4.5 [claude-opus-4-5-20251101]", "Claude Opus 4.5"},
		{"annotated latest paren", "Claude Opus 4.5 (latest)", "Claude Opus 4.5"},
		{"haiku latest paren", "Claude Haiku 4.5 (latest)", "Claude Haiku 4.5"},
		{"already clean", "Claude Opus 4.5", "Claude Opus 4.5"},
		{"clean gpt", "GPT-5.6 Sol", "GPT-5.6 Sol"},
		{"clean example", "Example", "Example"},
		{"clean nova", "Nova", "Nova"},
		{"whitespace normalization", "  Claude   Opus 4.5  ", "Claude Opus 4.5"},
		{"nested annotation removed", "[outer [inner]]", ""},
		{"unmatched opener suppresses rest", "[unterminated", ""},
		{"unmatched paren opener", "(latest", ""},
		{"mismatched closer discarded", "A[bc)de]", "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CleanModelName(tc.input); got != tc.want {
				t.Errorf("CleanModelName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCleanModelNameEmpty(t *testing.T) {
	if got := CleanModelName(""); got != "" {
		t.Errorf("CleanModelName(\"\") = %q, want \"\"", got)
	}
}
