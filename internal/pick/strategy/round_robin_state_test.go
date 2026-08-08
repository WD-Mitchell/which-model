package strategy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCursor(t *testing.T) {
	t.Run("case 1: nonexistent dir", func(t *testing.T) {
		dir := t.TempDir()
		got, err := loadCursor(filepath.Join(dir, "missing"), "abc")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		if got != 0 {
			t.Errorf("loadCursor() = %d, want 0", got)
		}
	})

	t.Run("case 2: corrupt file", func(t *testing.T) {
		dir := t.TempDir()
		if err := writeRawState(t, dir, "not json"); err != nil {
			t.Fatalf("setup error = %v", err)
		}
		got, err := loadCursor(dir, "abc")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		if got != 0 {
			t.Errorf("loadCursor() = %d, want 0", got)
		}
	})

	t.Run("case 3: after saveCursor", func(t *testing.T) {
		dir := t.TempDir()
		if err := saveCursor(dir, "abc", 3); err != nil {
			t.Fatalf("saveCursor() error = %v", err)
		}
		got, err := loadCursor(dir, "abc")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		if got != 3 {
			t.Errorf("loadCursor() = %d, want 3", got)
		}
	})

	t.Run("case 4: overwrite same key", func(t *testing.T) {
		dir := t.TempDir()
		if err := saveCursor(dir, "abc", 3); err != nil {
			t.Fatalf("saveCursor() error = %v", err)
		}
		if err := saveCursor(dir, "abc", 7); err != nil {
			t.Fatalf("saveCursor() error = %v", err)
		}
		got, err := loadCursor(dir, "abc")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		if got != 7 {
			t.Errorf("loadCursor() = %d, want 7", got)
		}
		raw, err := readRawState(t, dir)
		if err != nil {
			t.Fatalf("readRawState() error = %v", err)
		}
		if len(raw) != 1 {
			t.Errorf("file keys = %v, want exactly [abc]", raw)
		}
	})

	t.Run("case 5: two independent keys", func(t *testing.T) {
		dir := t.TempDir()
		if err := saveCursor(dir, "a", 3); err != nil {
			t.Fatalf("saveCursor() error = %v", err)
		}
		if err := saveCursor(dir, "b", 1); err != nil {
			t.Fatalf("saveCursor() error = %v", err)
		}
		gotA, err := loadCursor(dir, "a")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		gotB, err := loadCursor(dir, "b")
		if err != nil {
			t.Fatalf("loadCursor() error = %v", err)
		}
		if gotA != 3 || gotB != 1 {
			t.Errorf("loadCursor(a)=%d loadCursor(b)=%d, want 3 and 1", gotA, gotB)
		}
	})
}

func TestScopeKey(t *testing.T) {
	routes := []string{"codex/gpt-5.6-sol/max", "claude/claude-opus-4-8-20260115/max"}
	t.Run("case 6: stable and shaped", func(t *testing.T) {
		k1 := scopeKey("balanced_implementation", routes)
		k2 := scopeKey("balanced_implementation", routes)
		if k1 != k2 {
			t.Errorf("scopeKey() not stable: %q != %q", k1, k2)
		}
		if len(k1) != 16 {
			t.Errorf("scopeKey() length = %d, want 16", len(k1))
		}
		for _, r := range k1 {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				t.Errorf("scopeKey() = %q, want lowercase hex", k1)
				break
			}
		}
	})

	t.Run("case 7: order independent", func(t *testing.T) {
		reversed := []string{routes[1], routes[0]}
		if scopeKey("balanced_implementation", routes) != scopeKey("balanced_implementation", reversed) {
			t.Error("scopeKey() differs when route order is reversed")
		}
	})

	t.Run("case 8: different profile differs", func(t *testing.T) {
		if scopeKey("balanced_implementation", routes) == scopeKey("simple_action_execution", routes) {
			t.Error("scopeKey() same for different profiles")
		}
	})
}

// writeRawState writes raw content directly to the round-robin state file
// (test helper, bypassing saveCursor's JSON encoding).
func writeRawState(t *testing.T, dataDir, content string) error {
	t.Helper()
	path := stateFilePath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func readRawState(t *testing.T, dataDir string) (map[string]cursorDoc, error) {
	t.Helper()
	data, err := os.ReadFile(stateFilePath(dataDir))
	if err != nil {
		return nil, err
	}
	var m roundRobinFile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
