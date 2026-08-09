package whichmodel

import (
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/schema"
)

// TestSchemaCmd covers F28-T8 test cases 1-3: the schema command prints the
// JSON Schema index, a named document, and exits 2 for unknown names.
func TestSchemaCmd(t *testing.T) {
	// Case 1: no args → index.
	code, stdout, stderr := captureExecute(t, []string{"schema"})
	if code != 0 {
		t.Fatalf("schema: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != string(schema.Index()) {
		t.Errorf("schema stdout = %q, want %q", stdout, string(schema.Index()))
	}

	// Case 2: one name → that command's document.
	code, stdout, stderr = captureExecute(t, []string{"schema", "pick"})
	if code != 0 {
		t.Fatalf("schema pick: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	want, err := schema.Emit("pick")
	if err != nil {
		t.Fatalf("schema.Emit(pick): %v", err)
	}
	if stdout != string(want) {
		t.Errorf("schema pick stdout differs from schema.Emit(\"pick\")")
	}

	// Case 3: unknown name → exit 2, stderr mentions it.
	code, _, stderr = captureExecute(t, []string{"schema", "nonsense"})
	if code != 2 {
		t.Errorf("schema nonsense: exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "nonsense") {
		t.Errorf("schema nonsense stderr = %q, want it to contain %q", stderr, "nonsense")
	}
}
