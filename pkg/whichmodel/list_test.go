package whichmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256HexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// listFixture writes a scores CSV with the given header/rows and a matching
// raw CSV so its provenance hash is fresh (avoids staleness noise unless the
// test wants it).
func listFixture(t *testing.T, dir string, header string, rows []string, fresh bool) (scoresPath, rawPath string) {
	t.Helper()
	scoresPath, rawPath = staleFixtureContent(t, dir, header, rows, fresh)
	return scoresPath, rawPath
}

func staleFixtureContent(t *testing.T, dir, header string, rows []string, fresh bool) (scoresPath, rawPath string) {
	t.Helper()
	rawPath = filepath.Join(dir, "raw.csv")
	rawContent := []byte("model,reasoning\nA,high\n")
	if err := os.WriteFile(rawPath, rawContent, 0o644); err != nil {
		t.Fatalf("WriteFile(raw) error = %v", err)
	}
	hash := sha256HexOf(rawContent)
	if !fresh {
		hash = strings.Repeat("0", 64)
	}
	var sb strings.Builder
	sb.WriteString("# which-model-scores-provenance raw_sha256=" + hash + "\n")
	sb.WriteString(header + "\n")
	for _, r := range rows {
		sb.WriteString(r + "\n")
	}
	scoresPath = filepath.Join(dir, "scores.csv")
	if err := os.WriteFile(scoresPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("WriteFile(scores) error = %v", err)
	}
	return scoresPath, rawPath
}

func writeListConfig(t *testing.T, dir, scoresPath, rawPath string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	content := "[catalog]\nraw_csv_path = \"" + rawPath + "\"\nscores_csv_path = \"" + scoresPath + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	return path
}

func TestListCmd(t *testing.T) {
	header := "model,reasoning,intelligence_index,cost_per_intelligence_index_task_usd"

	t.Run("case 1: text table", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,63.1,2.34", "B,low,55.5,1.10"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stdout=%s", code, stdout)
		}
		if !strings.Contains(stdout, "model") || !strings.Contains(stdout, "reasoning") || !strings.Contains(stdout, "intelligence_index") || !strings.Contains(stdout, "cost_per_intelligence_index_task_usd") {
			t.Errorf("stdout header missing columns: %q", stdout)
		}
		if !strings.Contains(stdout, "A") || !strings.Contains(stdout, "B") {
			t.Errorf("stdout missing rows: %q", stdout)
		}
	})

	t.Run("case 2: json bare array", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,63.1,2.34"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
			t.Fatalf("json.Unmarshal() error = %v; stdout=%s", err, stdout)
		}
		if strings.Contains(stdout, "schema_version") {
			t.Errorf("stdout = %s, want no schema_version key (bare array)", stdout)
		}
	})

	t.Run("case 3: default sort desc by index", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"C,high,20.0,1", "A,high,63.1,1", "B,high,55.5,1"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout), &arr)
		if len(arr) != 3 || arr[0]["model"] != "A" || arr[1]["model"] != "B" || arr[2]["model"] != "C" {
			t.Errorf("order = %+v, want A, B, C (63.1, 55.5, 20.0)", arr)
		}
	})

	t.Run("case 4: tiebreak by model asc", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"B,high,50,1", "A,high,50,1"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout), &arr)
		if len(arr) != 2 || arr[0]["model"] != "A" || arr[1]["model"] != "B" {
			t.Errorf("order = %+v, want A before B", arr)
		}
	})

	t.Run("case 5: unparseable sorts last, omitted from json", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,50,1", "B,high,abc,1"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout), &arr)
		if len(arr) != 2 || arr[0]["model"] != "A" || arr[1]["model"] != "B" {
			t.Errorf("order = %+v, want A then B (unparseable last)", arr)
		}
		if _, ok := arr[1]["intelligence_index"]; ok {
			t.Errorf("arr[1] = %+v, want intelligence_index omitted (unparseable)", arr[1])
		}
	})

	t.Run("case 6: reasoning filter", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,max,50,1", "B,high,60,1", "C,low,70,1"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--reasoning", "max", "--reasoning", "high", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout), &arr)
		if len(arr) != 2 {
			t.Errorf("arr = %+v, want 2 matching rows", arr)
		}
		for _, r := range arr {
			if r["model"] == "C" {
				t.Errorf("arr = %+v, want C filtered out (reasoning low)", arr)
			}
		}
	})

	t.Run("case 7: min-score filter", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,40,1", "B,high,60,1"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--min-score", "50", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout), &arr)
		if len(arr) != 1 || arr[0]["model"] != "B" {
			t.Errorf("arr = %+v, want only B (index >= 50)", arr)
		}

		code2, stdout2, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--min-score", "0", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code2 != 0 {
			t.Fatalf("exit = %d, want 0", code2)
		}
		var arr2 []map[string]string
		json.Unmarshal([]byte(stdout2), &arr2)
		if len(arr2) != 2 {
			t.Errorf("arr2 = %+v, want both rows (min-score 0)", arr2)
		}
	})

	t.Run("case 8: missing file", func(t *testing.T) {
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "list", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 1 {
			t.Errorf("exit = %d, want 1; stderr=%s", code, stderr)
		}
		if !strings.Contains(stderr, "scores CSV not found at") {
			t.Errorf("stderr = %q, want prefix 'scores CSV not found at'", stderr)
		}
	})

	t.Run("case 9: missing column", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, "model,reasoning,intelligence_index", []string{"A,high,50"}, true)
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "list", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, "-") {
			t.Errorf("stdout = %q, want a '-' placeholder for the missing column", stdout)
		}

		code2, stdout2, _ := captureExecuteFresh(t, []string{"catalog", "list", "--json", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code2 != 0 {
			t.Fatalf("exit = %d, want 0", code2)
		}
		var arr []map[string]string
		json.Unmarshal([]byte(stdout2), &arr)
		if _, ok := arr[0]["cost_per_intelligence_index_task_usd"]; ok {
			t.Errorf("arr[0] = %+v, want missing column omitted", arr[0])
		}
	})

	t.Run("case 10: stale warning, still lists", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,50,1"}, false)
		code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "list", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stderr, "stale scores CSV") {
			t.Errorf("stderr = %q, want staleness warning", stderr)
		}
		if !strings.Contains(stdout, "A") {
			t.Errorf("stdout = %q, want the row still listed", stdout)
		}
	})

	t.Run("case 11: quiet suppresses staleness", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,50,1"}, false)
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "list", "--quiet", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if strings.Contains(stderr, "stale") {
			t.Errorf("stderr = %q, want no staleness warning under --quiet", stderr)
		}
	})

	t.Run("case 12: global refresh-scores stage runs derive first", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		dir := t.TempDir()
		scoresPath, rawPath := listFixture(t, dir, header, []string{"A,high,50,1"}, true)
		fake := &fakeStageRunner{deriveResult: DeriveResult{Rows: 1, ScoresCSVPath: scoresPath}}
		newRunner = func() StageRunner { return fake }
		code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "list", "--refresh-scores", "--config", writeListConfig(t, dir, scoresPath, rawPath), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if fake.deriveCalls != 1 {
			t.Errorf("deriveCalls = %d, want 1", fake.deriveCalls)
		}
		if !strings.Contains(stdout, "A") {
			t.Errorf("stdout = %q, want fixture row rendered", stdout)
		}
	})
}
