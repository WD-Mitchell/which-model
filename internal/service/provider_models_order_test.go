package service

import "testing"

// Regression (Codex P1 follow-up): reversed-order mixed family. The -max
// context row is seen FIRST, so the empty-effort slot must not inherit it;
// the unsuffixed executable must still survive.
func TestReversedOrderUnsuffixedSurvives(t *testing.T) {
	output := "Available models\n" +
		"foo-max - Foo Max\n" +
		"foo-low - Foo Low\n" +
		"foo - Foo\n" +
		"Tip: done\n"
	got, err := parseCursorModelList(output)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	ids := map[string]bool{}
	for _, e := range got {
		ids[e.ModelID] = true
	}
	if !ids["foo"] {
		t.Fatalf("unsuffixed route foo lost; got %#v", got)
	}
	if ids["foo-max"] {
		t.Fatalf("context row foo-max should be dropped; got %#v", got)
	}
	if !ids["foo-low"] {
		t.Fatalf("effort route foo-low lost; got %#v", got)
	}
}
