package routing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadTable(t *testing.T) {
	t.Run("case 1: round trip preserves all fields and order", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		table := Table{
			SchemaVersion: TableSchemaVersion,
			ScoresHash:    "abc123",
			RefreshedAt: map[string]string{
				"anthropic": "2026-08-08T00:00:00Z",
				"openai":    "2026-08-08T00:01:00Z",
			},
			Routes: []Route{
				{Provider: "anthropic", ModelID: "claude-opus-4-5", Model: "Claude Opus 5", Reasoning: "high", WindowIDs: []string{"5h"}, Provenance: ProvenanceModelsDev},
				{Provider: "openai", ModelID: "gpt-6", Model: "GPT-6", Reasoning: "default", WindowIDs: nil, Provenance: ProvenanceUserDeclared},
			},
		}
		if err := SaveTable(path, table); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		got, err := LoadTable(path)
		if err != nil {
			t.Fatalf("LoadTable() error = %v", err)
		}
		if !reflect.DeepEqual(got, table) {
			t.Errorf("LoadTable() = %+v, want %+v", got, table)
		}
	})

	t.Run("case 2: file exists and parses as JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		if err := SaveTable(path, Table{SchemaVersion: TableSchemaVersion}); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("%s is a directory, want a file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		var probe map[string]any
		if err := json.Unmarshal(data, &probe); err != nil {
			t.Errorf("saved file does not parse as JSON: %v", err)
		}
	})

	t.Run("case 3: LoadTable missing path", func(t *testing.T) {
		dir := t.TempDir()
		_, err := LoadTable(filepath.Join(dir, "missing.json"))
		if err == nil || !os.IsNotExist(err) {
			t.Fatalf("LoadTable() error = %v, want os.IsNotExist == true", err)
		}
	})

	t.Run("case 4: LoadTable corrupt content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		if err := SaveTable(path, Table{SchemaVersion: TableSchemaVersion}); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := LoadTable(path)
		if err == nil {
			t.Fatal("LoadTable() error = nil, want non-nil")
		}
		if os.IsNotExist(err) {
			t.Error("LoadTable() error is os.IsNotExist == true, want false")
		}
	})

	t.Run("case 5: LoadTable empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := LoadTable(path)
		if err == nil {
			t.Fatal("LoadTable() error = nil, want non-nil")
		}
	})

	t.Run("case 6: nested directory not yet existing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "a", "b", "routes.json")
		table := Table{SchemaVersion: TableSchemaVersion, ScoresHash: "x"}
		if err := SaveTable(path, table); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		got, err := LoadTable(path)
		if err != nil {
			t.Fatalf("LoadTable() error = %v", err)
		}
		if !reflect.DeepEqual(got, table) {
			t.Errorf("LoadTable() = %+v, want %+v", got, table)
		}
	})

	t.Run("case 7: no leftover tmp files after save", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		if err := SaveTable(path, Table{SchemaVersion: TableSchemaVersion}); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		if len(entries) != 1 || entries[0].Name() != "routes.json" {
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			t.Errorf("directory entries = %v, want exactly [routes.json]", names)
		}
	})

	t.Run("case 8: zero routes and nil RefreshedAt preserved", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		table := Table{SchemaVersion: TableSchemaVersion}
		if err := SaveTable(path, table); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		got, err := LoadTable(path)
		if err != nil {
			t.Fatalf("LoadTable() error = %v", err)
		}
		if got.Routes != nil {
			t.Errorf("Routes = %+v, want nil", got.Routes)
		}
		if got.RefreshedAt != nil {
			t.Errorf("RefreshedAt = %+v, want nil", got.RefreshedAt)
		}
	})

	t.Run("case 9: SchemaVersion round trips as 1.0", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		if err := SaveTable(path, Table{SchemaVersion: TableSchemaVersion}); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		got, err := LoadTable(path)
		if err != nil {
			t.Fatalf("LoadTable() error = %v", err)
		}
		if got.SchemaVersion != "1.0" {
			t.Errorf("SchemaVersion = %q, want 1.0", got.SchemaVersion)
		}
	})
}

func TestTableStale(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		current string
		want    bool
	}{
		{"case 1: matching hash", "abc", "abc", false},
		{"case 2: differing hash", "abc", "abd", true},
		{"case 3: both empty", "", "", false},
		{"case 4: empty table hash vs non-empty current", "", "abc", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := Table{ScoresHash: tc.hash}
			if got := table.Stale(tc.current); got != tc.want {
				t.Errorf("Stale(%q) with ScoresHash %q = %v, want %v", tc.current, tc.hash, got, tc.want)
			}
		})
	}
}

func TestTableProvenanceCounts(t *testing.T) {
	t.Run("case 5: mixed provenance, only present keys", func(t *testing.T) {
		table := Table{Routes: []Route{
			{Provider: "a", ModelID: "m1", Provenance: ProvenanceModelsDev},
			{Provider: "a", ModelID: "m2", Provenance: ProvenanceModelsDev},
			{Provider: "b", ModelID: "m3", Provenance: ProvenanceUserDeclared},
		}}
		got := table.ProvenanceCounts()
		want := map[Provenance]int{ProvenanceModelsDev: 2, ProvenanceUserDeclared: 1}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ProvenanceCounts() = %+v, want %+v", got, want)
		}
		if _, ok := got[ProvenanceProviderLive]; ok {
			t.Errorf("ProvenanceCounts() has unexpected key provider_live: %+v", got)
		}
	})

	t.Run("case 6: zero routes returns empty map", func(t *testing.T) {
		got := Table{}.ProvenanceCounts()
		if len(got) != 0 {
			t.Errorf("ProvenanceCounts() = %+v, want empty map", got)
		}
	})

	t.Run("case 7: counts preserved across save/load round trip", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "routes.json")
		table := Table{
			SchemaVersion: TableSchemaVersion,
			Routes: []Route{
				{Provider: "a", ModelID: "m1", Provenance: ProvenanceModelsDev},
				{Provider: "a", ModelID: "m2", Provenance: ProvenanceModelsDev},
				{Provider: "b", ModelID: "m3", Provenance: ProvenanceUserDeclared},
			},
		}
		if err := SaveTable(path, table); err != nil {
			t.Fatalf("SaveTable() error = %v", err)
		}
		loaded, err := LoadTable(path)
		if err != nil {
			t.Fatalf("LoadTable() error = %v", err)
		}
		got := loaded.ProvenanceCounts()
		want := map[Provenance]int{ProvenanceModelsDev: 2, ProvenanceUserDeclared: 1}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ProvenanceCounts() after round trip = %+v, want %+v", got, want)
		}
	})
}
