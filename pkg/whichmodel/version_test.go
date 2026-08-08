package whichmodel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	t.Run("version line format", func(t *testing.T) {
		prevV, prevC, prevB := Version, Commit, BuildDate
		Version, Commit, BuildDate = "1.2.3", "abc", "2026-08-07"
		defer func() { Version, Commit, BuildDate = prevV, prevC, prevB }()
		want := "which-model 1.2.3 (commit abc, built 2026-08-07)"
		if got := VersionLine(); got != want {
			t.Errorf("VersionLine = %q, want %q", got, want)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		line := VersionLine()
		for _, part := range []string{"dev", "unknown"} {
			if !strings.Contains(line, part) {
				t.Errorf("default line %q missing %q", line, part)
			}
		}
	})

	t.Run("version json keys", func(t *testing.T) {
		doc := VersionJSON()
		for _, key := range []string{"version", "commit", "built_at"} {
			if _, ok := doc[key]; !ok {
				t.Errorf("VersionJSON missing key %q", key)
			}
		}
	})

	t.Run("short-circuit flag", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"--version"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if out != VersionLine()+"\n" {
			t.Errorf("stdout = %q, want %q", out, VersionLine()+"\n")
		}
	})

	t.Run("flag variant true", func(t *testing.T) {
		code, _, _ := captureExecute(t, []string{"--version=true"})
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
	})

	t.Run("false ignored runs command", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"--version=false", "version"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if out != VersionLine()+"\n" {
			t.Errorf("stdout = %q, want version line", out)
		}
	})

	t.Run("subcommand", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"version"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if out != VersionLine()+"\n" {
			t.Errorf("stdout = %q, want version line", out)
		}
	})

	t.Run("json form", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"version", "--json"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("stdout not JSON: %v (%q)", err, out)
		}
		if doc["schema_version"] != "2.0" {
			t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
		}
		for _, key := range []string{"version", "commit", "built_at"} {
			if _, ok := doc[key]; !ok {
				t.Errorf("json missing %q", key)
			}
		}
	})
}
