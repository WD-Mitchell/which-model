package whichmodel

import (
	"slices"
	"strings"
	"testing"
)

func TestSchema(t *testing.T) {
	t.Run("index command", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"schema"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "version") {
			t.Errorf("schema index missing version: %q", out)
		}
	})

	t.Run("doc command", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"schema", "version"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, `"type":"object"`) {
			t.Errorf("version doc missing type object: %q", out)
		}
	})

	t.Run("unknown path", func(t *testing.T) {
		code, _, errOut := captureExecute(t, []string{"schema", "nope"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if !strings.Contains(errOut, "[arguments]") {
			t.Errorf("stderr = %q, want [arguments]", errOut)
		}
	})

	t.Run("hook short-circuits", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"version", "--schema"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, `"type":"object"`) {
			t.Errorf("hook stdout is not the version doc: %q", out)
		}
	})

	t.Run("hook before flags", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"--schema"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "commands") {
			t.Errorf("hook stdout is not the index: %q", out)
		}
	})

	t.Run("hook unknown command", func(t *testing.T) {
		code, _, _ := captureExecute(t, []string{"nope", "--schema"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
	})

	t.Run("hook respects terminator", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"--", "--schema"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (help, flag not scanned)", code)
		}
		if !strings.Contains(out, "Usage:") {
			t.Errorf("expected help text after --, got %q", out)
		}
	})

	t.Run("index sorted", func(t *testing.T) {
		RegisterSchema("z", map[string]any{"type": "object"})
		RegisterSchema("a", map[string]any{"type": "object"})
		idx := SchemaIndex()
		if !slices.IsSorted(idx) {
			t.Errorf("SchemaIndex not sorted: %v", idx)
		}
		for _, want := range []string{"a", "version", "z"} {
			if !slices.Contains(idx, want) {
				t.Errorf("SchemaIndex missing %q: %v", want, idx)
			}
		}
	})

	t.Run("last write wins", func(t *testing.T) {
		doc2 := map[string]any{"type": "object", "title": "doc2-marker"}
		RegisterSchema("version", doc2)
		code, out, _ := captureExecute(t, []string{"schema", "version"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "doc2-marker") {
			t.Errorf("schema version did not pick up doc2: %q", out)
		}
	})
}
