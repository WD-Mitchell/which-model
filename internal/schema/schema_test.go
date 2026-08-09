package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestCommands covers F28-T1 test 1: index order and slice independence.
func TestCommands(t *testing.T) {
	want := []string{"usage", "pick", "explain", "routes"}
	got := Commands()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
	got[0] = "mutated"
	if again := Commands(); !reflect.DeepEqual(again, want) {
		t.Fatalf("Commands() returned a shared slice: second call = %v, want %v", again, want)
	}
}

// TestEmitMatchesGolden covers F28-T2 tests 1-4: byte equality with the
// testdata goldens for every command.
func TestEmitMatchesGolden(t *testing.T) {
	for _, name := range Commands() {
		t.Run(name, func(t *testing.T) {
			doc, err := Emit(name)
			if err != nil {
				t.Fatalf("Emit(%q): %v", name, err)
			}
			golden, err := os.ReadFile("testdata/" + name + "-schema.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(doc, golden) {
				t.Errorf("Emit(%q) bytes differ from golden", name)
			}
		})
	}
}

// TestDocumentsWellFormed covers F28-T2 test 5: every document parses as JSON
// and carries schema_version const "2.0" at the root.
func TestDocumentsWellFormed(t *testing.T) {
	for _, name := range Commands() {
		t.Run(name, func(t *testing.T) {
			doc, err := Emit(name)
			if err != nil {
				t.Fatalf("Emit(%q): %v", name, err)
			}
			if !json.Valid(doc) {
				t.Fatalf("Emit(%q) is not valid JSON", name)
			}
			var root map[string]any
			if err := json.Unmarshal(doc, &root); err != nil {
				t.Fatalf("unmarshal %q: %v", name, err)
			}
			props, ok := root["properties"].(map[string]any)
			if !ok {
				t.Fatalf("%q: root properties missing", name)
			}
			sv, ok := props["schema_version"].(map[string]any)
			if !ok {
				t.Fatalf("%q: schema_version property missing", name)
			}
			if sv["const"] != "2.0" {
				t.Errorf("%q: schema_version const = %v, want \"2.0\"", name, sv["const"])
			}
		})
	}
}

// TestUsageEnabledRequired covers F28-T2 test 6: usage/pick/explain roots
// require usage_enabled and carry usage_disabled_reason plus the if/then.
func TestUsageEnabledRequired(t *testing.T) {
	for _, name := range []string{"usage", "pick", "explain"} {
		t.Run(name, func(t *testing.T) {
			doc, err := Emit(name)
			if err != nil {
				t.Fatalf("Emit(%q): %v", name, err)
			}
			var root map[string]any
			if err := json.Unmarshal(doc, &root); err != nil {
				t.Fatalf("unmarshal %q: %v", name, err)
			}
			required, ok := root["required"].([]any)
			if !ok {
				t.Fatalf("%q: root required missing", name)
			}
			found := false
			for _, r := range required {
				if r == "usage_enabled" {
					found = true
				}
			}
			if !found {
				t.Errorf("%q: root required %v missing usage_enabled", name, required)
			}
			if !bytes.Contains(doc, []byte("usage_disabled_reason")) {
				t.Errorf("%q: document lacks usage_disabled_reason", name)
			}
			if _, ok := root["if"]; !ok {
				t.Errorf("%q: root lacks the \"if\" key", name)
			}
			if _, ok := root["then"]; !ok {
				t.Errorf("%q: root lacks the \"then\" key", name)
			}
		})
	}
}

// TestIndex covers F28-T2 test 7: the no-argument index is exact.
func TestIndex(t *testing.T) {
	want := `{"commands":["usage","pick","explain","routes"]}` + "\n"
	if got := string(Index()); got != want {
		t.Errorf("Index() = %q, want %q", got, want)
	}
}

// TestEmitStable covers F28-T2 test 8: repeated Emit is byte-identical.
func TestEmitStable(t *testing.T) {
	a, err := Emit("pick")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Emit("pick")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("Emit(\"pick\") is not stable across calls")
	}
}

// TestUnknownCommand covers F28-T1 tests 2-3 and F28-T2 test 9: unknown
// names yield *UnknownCommandError naming the valid commands.
func TestUnknownCommand(t *testing.T) {
	_, err := Emit("nonsense")
	if err == nil {
		t.Fatal("Emit(\"nonsense\") = nil error, want *UnknownCommandError")
	}
	u, ok := err.(*UnknownCommandError)
	if !ok {
		t.Fatalf("error type = %T, want *UnknownCommandError", err)
	}
	if u.Name != "nonsense" {
		t.Errorf("Name = %q, want %q", u.Name, "nonsense")
	}
	if !reflect.DeepEqual(u.Commands, []string{"usage", "pick", "explain", "routes"}) {
		t.Errorf("Commands = %v", u.Commands)
	}
	msg := err.Error()
	for _, want := range append(u.Commands, "nonsense") {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q does not mention %q", msg, want)
		}
	}
}
