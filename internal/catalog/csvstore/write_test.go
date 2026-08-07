package csvstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertNoTempFiles(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "."+base+".") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestWriteAtomic(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		content := rawHeader + "\nClaude Opus 5,max,63.1,465,2.34,61,78.0,59.2\n"
		path := writeTemp(t, content)
		rows, _, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteAtomic(path, rows, nil); err != nil {
			t.Fatal(err)
		}
		rows2, prov, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if prov != nil {
			t.Errorf("provenance = %+v, want nil", prov)
		}
		if len(rows2) != len(rows) || rows2[0].Values[0] != rows[0].Values[0] {
			t.Errorf("round-trip rows differ: %v", rows2)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "\r\n") {
			t.Error("file contains \\r\\n terminators")
		}
	})

	hash64a := strings.Repeat("a", 64)
	scoresBody := "model,reasoning,intelligence_index_score\nClaude Fable 5,max,98\n"
	rowsForScores := func(t *testing.T, path string) []Row {
		t.Helper()
		if err := os.WriteFile(path, []byte(scoresBody), 0o644); err != nil {
			t.Fatal(err)
		}
		rows, _, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	t.Run("full provenance round trip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "scores.csv")
		rows := rowsForScores(t, path)
		prov := &Provenance{RawSHA256: hash64a, Normalizer: "minmax-linear", Aggregator: "weighted-arithmetic-mean"}
		if err := WriteAtomic(path, rows, prov); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		wantFirst := ProvenancePrefix + " raw_sha256=" + hash64a + " normalizer=minmax-linear aggregator=weighted-arithmetic-mean"
		if first := strings.SplitN(string(raw), "\n", 2)[0]; first != wantFirst {
			t.Errorf("first line = %q, want %q", first, wantFirst)
		}
		_, got, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.RawSHA256 != hash64a || got.Normalizer != "minmax-linear" || got.Aggregator != "weighted-arithmetic-mean" {
			t.Errorf("provenance = %+v", got)
		}
	})

	t.Run("hash-only provenance", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "scores.csv")
		rows := rowsForScores(t, path)
		if err := WriteAtomic(path, rows, &Provenance{RawSHA256: hash64a}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		wantFirst := ProvenancePrefix + " raw_sha256=" + hash64a
		if first := strings.SplitN(string(raw), "\n", 2)[0]; first != wantFirst {
			t.Errorf("first line = %q, want %q", first, wantFirst)
		}
		_, got, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Normalizer != "" || got.Aggregator != "" {
			t.Errorf("provenance = %+v, want empty normalizer/aggregator", got)
		}
	})

	t.Run("no provenance", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "scores.csv")
		rows := rowsForScores(t, path)
		if err := WriteAtomic(path, rows, nil); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if !strings.HasPrefix(string(raw), "model,") {
			t.Errorf("first line = %q, want header", strings.SplitN(string(raw), "\n", 2)[0])
		}
	})

	t.Run("crash safety A values mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		original := "a,b\n1,2\n"
		if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}
		rows := []Row{{Header: []string{"a", "b"}, Values: []string{"1"}}}
		err := WriteAtomic(path, rows, nil)
		if !errors.Is(err, ErrMalformedCSV) {
			t.Fatalf("err = %v, want ErrMalformedCSV", err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != original {
			t.Errorf("file mutated: %q", got)
		}
		assertNoTempFiles(t, dir, "data.csv")
	})

	t.Run("crash safety B target is directory", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		rows := []Row{{Header: []string{"a"}, Values: []string{"1"}}}
		if err := WriteAtomic(path, rows, nil); err == nil {
			t.Fatal("expected error writing over a directory")
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Errorf("target directory lost: %v", err)
		}
		assertNoTempFiles(t, dir, "data.csv")
	})

	t.Run("missing target", func(t *testing.T) {
		rows := []Row{{Header: []string{"a"}, Values: []string{"1"}}}
		err := WriteAtomic(filepath.Join(t.TempDir(), "absent.csv"), rows, nil)
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		path := writeTemp(t, "a,b\n1,2\n")
		original, _ := os.ReadFile(path)
		err := WriteAtomic(path, nil, nil)
		if !errors.Is(err, ErrMalformedCSV) {
			t.Fatalf("err = %v, want ErrMalformedCSV", err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != string(original) {
			t.Error("file mutated on empty rows")
		}
	})

	t.Run("comma cell quoting", func(t *testing.T) {
		path := writeTemp(t, "a,b\nx,y\n")
		rows := []Row{{Header: []string{"a", "b"}, Values: []string{"a,b", "2"}}}
		if err := WriteAtomic(path, rows, nil); err != nil {
			t.Fatal(err)
		}
		back, _, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if back[0].Values[0] != "a,b" {
			t.Errorf("cell = %q, want a,b", back[0].Values[0])
		}
	})

	t.Run("blank cells", func(t *testing.T) {
		path := writeTemp(t, "a,b\nx,y\n")
		rows := []Row{{Header: []string{"a", "b"}, Values: []string{"", ""}}}
		if err := WriteAtomic(path, rows, nil); err != nil {
			t.Fatal(err)
		}
		back, _, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if back[0].Values[0] != "" || back[0].Values[1] != "" {
			t.Errorf("cells = %v, want blanks", back[0].Values)
		}
	})
}

func TestWriteAtomicBytes(t *testing.T) {
	t.Run("byte exact", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "scores.csv")
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		hash64b := strings.Repeat("b", 64)
		newContent := []byte(ProvenancePrefix + " raw_sha256=" + hash64b + " normalizer=minmax-linear\nmodel,reasoning,intelligence_index_score\nM,max,1\n")
		if err := WriteAtomicBytes(path, newContent); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != string(newContent) {
			t.Errorf("bytes differ:\n got %q\nwant %q", got, newContent)
		}
		rows, prov, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if prov == nil || prov.RawSHA256 != hash64b || prov.Normalizer != "minmax-linear" {
			t.Errorf("provenance = %+v", prov)
		}
		if len(rows) != 1 {
			t.Errorf("rows = %d, want 1", len(rows))
		}
	})

	t.Run("nonexistent target", func(t *testing.T) {
		err := WriteAtomicBytes(filepath.Join(t.TempDir(), "absent.csv"), []byte("x"))
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})
}
