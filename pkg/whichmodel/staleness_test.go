package whichmodel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleFixture writes a raw CSV and a scores CSV whose provenance line
// references rawContent's hash (fresh) or a different hash (stale).
func staleFixture(t *testing.T, dir string, fresh bool) (scoresPath, rawPath string) {
	t.Helper()
	rawPath = filepath.Join(dir, "raw.csv")
	rawContent := []byte("model,reasoning\nA,high\n")
	if err := os.WriteFile(rawPath, rawContent, 0o644); err != nil {
		t.Fatalf("WriteFile(raw) error = %v", err)
	}
	sum := sha256.Sum256(rawContent)
	hash := hex.EncodeToString(sum[:])
	if !fresh {
		hash = strings.Repeat("0", 64)
	}
	scoresPath = filepath.Join(dir, "scores.csv")
	var buf bytes.Buffer
	buf.WriteString("# which-model-scores-provenance raw_sha256=" + hash + "\n")
	buf.WriteString("model,reasoning\nA,high\n")
	if err := os.WriteFile(scoresPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(scores) error = %v", err)
	}
	return scoresPath, rawPath
}

func TestWarnIfStale(t *testing.T) {
	t.Run("case 1: stale warning exact text", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, false)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, false, true)
		want := "warning: stale scores CSV " + scoresPath + ": its recorded raw CSV hash does not match the current " + rawPath + "; run --refresh-scores to regenerate\n"
		if buf.String() != want {
			t.Errorf("stderr = %q, want %q", buf.String(), want)
		}
	})

	t.Run("case 2: fresh silent", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, true)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, false, true)
		if buf.Len() != 0 {
			t.Errorf("stderr = %q, want empty", buf.String())
		}
	})

	t.Run("case 3: provenance-unknown silent", func(t *testing.T) {
		dir := t.TempDir()
		rawPath := filepath.Join(dir, "raw.csv")
		os.WriteFile(rawPath, []byte("model,reasoning\nA,high\n"), 0o644)
		scoresPath := filepath.Join(dir, "scores.csv")
		os.WriteFile(scoresPath, []byte("model,reasoning\nA,high\n"), 0o644)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, false, true)
		if buf.Len() != 0 {
			t.Errorf("stderr = %q, want empty (no provenance line)", buf.String())
		}
	})

	t.Run("case 4: missing raw silent", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, false)
		os.Remove(rawPath)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, false, true)
		if buf.Len() != 0 {
			t.Errorf("stderr = %q, want empty (StaleCheck error suppressed)", buf.String())
		}
	})

	t.Run("case 5: quiet suppresses", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, false)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, true, true)
		if buf.Len() != 0 {
			t.Errorf("stderr = %q, want empty (quiet)", buf.String())
		}
	})

	t.Run("case 6: config suppresses", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, false)
		var buf bytes.Buffer
		origStderr := Stderr
		Stderr = &buf
		defer func() { Stderr = origStderr }()
		warnIfStale(scoresPath, rawPath, false, false)
		if buf.Len() != 0 {
			t.Errorf("stderr = %q, want empty (warnOnStale false)", buf.String())
		}
	})

	t.Run("case 7: benchmarks integration", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		dir := t.TempDir()
		scoresPath, rawPath := staleFixture(t, dir, false)

		configPath := filepath.Join(dir, "config.toml")
		os.WriteFile(configPath, []byte("[catalog]\nraw_csv_path = \""+rawPath+"\"\nscores_csv_path = \""+scoresPath+"\"\n"), 0o644)
		providersPath := writeEmptyProviders(t, dir)

		fake := &fakeStageRunner{collectResult: CollectResult{Providers: 1, Models: 1, RawCSVPath: rawPath}}
		newRunner = func() StageRunner { return fake }
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")

		code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--config", configPath, "--provider-config", providersPath})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if !strings.Contains(stderr, "stale scores CSV") {
			t.Errorf("stderr = %q, want the staleness warning", stderr)
		}
		if !strings.Contains(stdout, "collected 1 providers, 1 models") {
			t.Errorf("stdout = %q, want the collect line", stdout)
		}
	})
}
