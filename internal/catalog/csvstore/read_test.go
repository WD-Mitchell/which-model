package csvstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const rawHeader = "model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index"

func TestRead(t *testing.T) {
	t.Run("valid raw csv no comment", func(t *testing.T) {
		content := rawHeader + "\n" +
			"Claude Opus 5,max,63.1,465,2.34,61,78.0,59.2\n" +
			"Kimi K2.7 Code,default,43.0,,0.22,67,60.8,30.3\n"
		rows, prov, err := Read(writeTemp(t, content))
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if prov != nil {
			t.Errorf("provenance = %+v, want nil", prov)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		wantHeader := strings.Split(rawHeader, ",")
		for _, r := range rows {
			if strings.Join(r.Header, ",") != strings.Join(wantHeader, ",") {
				t.Errorf("header = %v, want %v", r.Header, wantHeader)
			}
		}
		if rows[1].Values[1] != "default" {
			t.Errorf("row 2 reasoning = %q, want default", rows[1].Values[1])
		}
		if rows[1].Values[3] != "" {
			t.Errorf("row 2 time cell = %q, want blank", rows[1].Values[3])
		}
	})

	hash64a := strings.Repeat("a", 64)
	fullProv := ProvenancePrefix + " raw_sha256=" + hash64a + " normalizer=minmax-linear aggregator=weighted-arithmetic-mean"
	scoresBody := "model,reasoning,intelligence_index_score\nClaude Fable 5,max,98\n"

	t.Run("full provenance line", func(t *testing.T) {
		rows, prov, err := Read(writeTemp(t, fullProv+"\n"+scoresBody))
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if prov == nil || prov.RawSHA256 != hash64a || prov.Normalizer != "minmax-linear" || prov.Aggregator != "weighted-arithmetic-mean" {
			t.Fatalf("provenance = %+v", prov)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		for _, r := range rows {
			for _, cell := range append([]string{}, append(r.Header, r.Values...)...) {
				if strings.Contains(cell, "#") {
					t.Errorf("comment leaked into cell %q", cell)
				}
			}
		}
	})

	t.Run("hash-only provenance", func(t *testing.T) {
		_, prov, err := Read(writeTemp(t, ProvenancePrefix+" raw_sha256="+hash64a+"\n"+scoresBody))
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if prov == nil || prov.RawSHA256 != hash64a || prov.Normalizer != "" || prov.Aggregator != "" {
			t.Fatalf("provenance = %+v", prov)
		}
	})

	t.Run("wrong keyword", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, "# raw_sha256="+hash64a+"\n"+scoresBody))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("non-hex hash", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, ProvenancePrefix+" raw_sha256=xyz\n"+scoresBody))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("missing raw_sha256 token", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, ProvenancePrefix+" normalizer=minmax-linear\n"+scoresBody))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("unknown token skipped", func(t *testing.T) {
		_, prov, err := Read(writeTemp(t, ProvenancePrefix+" raw_sha256="+hash64a+" foo=bar\n"+scoresBody))
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if prov == nil || prov.RawSHA256 != hash64a {
			t.Fatalf("provenance = %+v", prov)
		}
	})

	t.Run("two comment lines", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, fullProv+"\n"+ProvenancePrefix+" raw_sha256="+hash64a+"\n"+scoresBody))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("uppercase hash rejected", func(t *testing.T) {
		// F06 CONTRACTS pins 64 LOWERCASE hex; the writer already rejected
		// uppercase, so Read must too (issue #46 asymmetry).
		upper := strings.ToUpper(hash64a)
		_, _, err := Read(writeTemp(t, ProvenancePrefix+" raw_sha256="+upper+"\n"+scoresBody))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("short row", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, "a,b,c\n1,2\n"))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, err := Read(filepath.Join(t.TempDir(), "absent.csv"))
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})

	t.Run("oversized file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "big.csv")
		if err := os.WriteFile(path, make([]byte, MaxCsvBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := Read(path)
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("err = %v, want ErrFileTooLarge", err)
		}
	})

	t.Run("invalid utf8", func(t *testing.T) {
		_, _, err := Read(writeTemp(t, "model\xff\n"))
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})
}
