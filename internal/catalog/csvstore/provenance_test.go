package csvstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestProvenanceHash(t *testing.T) {
	t.Run("hash of known bytes", func(t *testing.T) {
		content := "model,reasoning\nClaude Opus 5,max\n"
		path := writeTemp(t, content)
		got, err := ProvenanceHash(path)
		if err != nil {
			t.Fatal(err)
		}
		if want := sha256Hex([]byte(content)); got != want {
			t.Errorf("hash = %s, want %s", got, want)
		}
	})

	t.Run("order sensitive", func(t *testing.T) {
		a := writeTemp(t, "model,reasoning\nr1,high\nr2,high\n")
		b := writeTemp(t, "model,reasoning\nr2,high\nr1,high\n")
		ha, err := ProvenanceHash(a)
		if err != nil {
			t.Fatal(err)
		}
		hb, err := ProvenanceHash(b)
		if err != nil {
			t.Fatal(err)
		}
		if ha == hb {
			t.Error("row order must change the hash")
		}
	})

	t.Run("identical content equal hash", func(t *testing.T) {
		a := writeTemp(t, "x,y\n1,2\n")
		b := writeTemp(t, "x,y\n1,2\n")
		ha, _ := ProvenanceHash(a)
		hb, _ := ProvenanceHash(b)
		if ha != hb {
			t.Errorf("hashes differ: %s vs %s", ha, hb)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ProvenanceHash(filepath.Join(t.TempDir(), "absent.csv"))
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})
}

func scoresWithProvenance(t *testing.T, rawHash string) string {
	t.Helper()
	return ProvenancePrefix + " raw_sha256=" + rawHash + "\nmodel,reasoning,intelligence_index_score\nClaude Fable 5,max,98\n"
}

func TestStaleCheck(t *testing.T) {
	t.Run("matching provenance", func(t *testing.T) {
		dir := t.TempDir()
		rawPath := filepath.Join(dir, "raw.csv")
		rawContent := "model,reasoning\nM,max\n"
		if err := os.WriteFile(rawPath, []byte(rawContent), 0o644); err != nil {
			t.Fatal(err)
		}
		scoresPath := filepath.Join(dir, "scores.csv")
		if err := os.WriteFile(scoresPath, []byte(scoresWithProvenance(t, sha256Hex([]byte(rawContent)))), 0o644); err != nil {
			t.Fatal(err)
		}
		stale, err := StaleCheck(scoresPath, rawPath)
		if err != nil || stale {
			t.Errorf("stale = %v, err = %v; want false, nil", stale, err)
		}
	})

	t.Run("mismatched hash", func(t *testing.T) {
		dir := t.TempDir()
		rawPath := filepath.Join(dir, "raw.csv")
		if err := os.WriteFile(rawPath, []byte("model,reasoning\nM,max\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		scoresPath := filepath.Join(dir, "scores.csv")
		if err := os.WriteFile(scoresPath, []byte(scoresWithProvenance(t, strings.Repeat("b", 64))), 0o644); err != nil {
			t.Fatal(err)
		}
		stale, err := StaleCheck(scoresPath, rawPath)
		if err != nil || !stale {
			t.Errorf("stale = %v, err = %v; want true, nil", stale, err)
		}
	})

	t.Run("provenance unknown", func(t *testing.T) {
		dir := t.TempDir()
		rawPath := filepath.Join(dir, "raw.csv")
		if err := os.WriteFile(rawPath, []byte("model,reasoning\nM,max\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		scoresPath := filepath.Join(dir, "scores.csv")
		if err := os.WriteFile(scoresPath, []byte("model,reasoning,intelligence_index_score\nM,max,1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stale, err := StaleCheck(scoresPath, rawPath)
		if err != nil || stale {
			t.Errorf("stale = %v, err = %v; want false, nil", stale, err)
		}
	})

	t.Run("missing scores", func(t *testing.T) {
		rawPath := writeTemp(t, "model,reasoning\nM,max\n")
		_, err := StaleCheck(filepath.Join(t.TempDir(), "absent.csv"), rawPath)
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})

	t.Run("missing raw", func(t *testing.T) {
		dir := t.TempDir()
		scoresPath := filepath.Join(dir, "scores.csv")
		if err := os.WriteFile(scoresPath, []byte(scoresWithProvenance(t, strings.Repeat("c", 64))), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := StaleCheck(scoresPath, filepath.Join(dir, "absent.csv"))
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})

	t.Run("end to end write then modify", func(t *testing.T) {
		dir := t.TempDir()
		rawPath := filepath.Join(dir, "raw.csv")
		rawContent := "model,reasoning\nM,max\n"
		if err := os.WriteFile(rawPath, []byte(rawContent), 0o644); err != nil {
			t.Fatal(err)
		}
		scoresPath := filepath.Join(dir, "scores.csv")
		hash, err := ProvenanceHash(rawPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scoresPath, []byte("model,reasoning,intelligence_index_score\nM,max,1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rows, _, err := Read(scoresPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteAtomic(scoresPath, rows, &Provenance{RawSHA256: hash}); err != nil {
			t.Fatal(err)
		}
		stale, err := StaleCheck(scoresPath, rawPath)
		if err != nil || stale {
			t.Fatalf("after write: stale = %v, err = %v; want false, nil", stale, err)
		}
		if err := os.WriteFile(rawPath, []byte(rawContent+"N,max\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stale, err = StaleCheck(scoresPath, rawPath)
		if err != nil || !stale {
			t.Errorf("after modify: stale = %v, err = %v; want true, nil", stale, err)
		}
	})

	t.Run("byte-exact write preserves hash", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.csv")
		content := "model,reasoning\nM,max\n"
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		before, err := ProvenanceHash(target)
		if err != nil {
			t.Fatal(err)
		}
		rows, _, err := Read(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteAtomic(target, rows, nil); err != nil {
			t.Fatal(err)
		}
		after, err := ProvenanceHash(target)
		if err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Errorf("hash changed across WriteAtomic: %s → %s", before, after)
		}
	})
}

func TestStaleWarning(t *testing.T) {
	want := "stale scores CSV a.csv: its recorded raw CSV hash does not match the current b.csv; run --refresh-scores to regenerate"
	if got := StaleWarning("a.csv", "b.csv"); got != want {
		t.Errorf("StaleWarning = %q, want %q", got, want)
	}
}
