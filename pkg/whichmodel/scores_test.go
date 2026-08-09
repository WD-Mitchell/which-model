package whichmodel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScoresCmd(t *testing.T) {
	t.Run("case 1: happy text", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{deriveResult: DeriveResult{Rows: 39, ScoresCSVPath: "/tmp/scores.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "scores", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, "derived 39 rows -> /tmp/scores.csv") {
			t.Errorf("stdout = %q", stdout)
		}
	})

	t.Run("case 2: json shape, no collect key", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{deriveResult: DeriveResult{Rows: 39, ScoresCSVPath: "/tmp/scores.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "scores", "--json", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, `"rows":39`) {
			t.Errorf("stdout = %s", stdout)
		}
		if strings.Contains(stdout, `"collect"`) {
			t.Errorf("stdout = %s, want no collect key", stdout)
		}
	})

	t.Run("case 3: offline allowed", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{deriveResult: DeriveResult{Rows: 1, ScoresCSVPath: "/tmp/s.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "scores", "--offline", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if fake.deriveCalls != 1 {
			t.Errorf("deriveCalls = %d, want 1", fake.deriveCalls)
		}
	})

	t.Run("case 4: passthrough", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		inPath := filepath.Join(dir, "r.csv")
		outPath := filepath.Join(dir, "s.csv")
		benchPath := filepath.Join(dir, "b.toml")
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "scores", "--in", inPath, "--out", outPath, "--benchmarks", benchPath, "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if fake.capturedDO.InPath != inPath || fake.capturedDO.OutPath != outPath || fake.capturedDO.BenchmarksPath != benchPath {
			t.Errorf("capturedDO = %+v", fake.capturedDO)
		}
	})

	t.Run("case 5: missing raw", func(t *testing.T) {
		dir := t.TempDir()
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: filepath.Join(dir, "missing.csv")})
		if err == nil || !strings.HasPrefix(err.Error(), "raw CSV not found at") {
			t.Errorf("Derive() error = %v, want prefix 'raw CSV not found at'", err)
		}
	})

	t.Run("case 6: missing benchmarks", func(t *testing.T) {
		dir := t.TempDir()
		raw := filepath.Join(dir, "raw.csv")
		if err := os.WriteFile(raw, []byte("model,reasoning\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: raw, BenchmarksPath: filepath.Join(dir, "missing.toml")})
		if err == nil || !strings.HasPrefix(err.Error(), "benchmarks config not found at") {
			t.Errorf("Derive() error = %v, want prefix 'benchmarks config not found at'", err)
		}
	})

	t.Run("case 7: unknown aggregator", func(t *testing.T) {
		dir := t.TempDir()
		raw := filepath.Join(dir, "raw.csv")
		bench := filepath.Join(dir, "b.toml")
		if err := os.WriteFile(raw, []byte("model,reasoning\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.WriteFile(bench, []byte("[benchmark_selection]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: raw, BenchmarksPath: bench, Aggregator: "bogus", OutPath: filepath.Join(dir, "s.csv")})
		if err == nil {
			t.Fatal("Derive() error = nil, want error")
		}
	})

	t.Run("case 8: backup called on existing scores file", func(t *testing.T) {
		dir := t.TempDir()
		raw := filepath.Join(dir, "raw.csv")
		bench := filepath.Join(dir, "b.toml")
		out := filepath.Join(dir, "s.csv")
		if err := os.WriteFile(raw, []byte("model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index\nA,high,80,1,1,1,80,80\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.WriteFile(bench, []byte("[benchmark_selection]\ngroups = []\nbenchmarks = []\n\n[benchmark_groups]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.WriteFile(out, []byte("old content\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: raw, BenchmarksPath: bench, OutPath: out, Normalizer: "minmax-linear", Aggregator: "weighted-arithmetic-mean"})
		if err != nil {
			t.Fatalf("Derive() error = %v", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		hasBak := false
		for _, e := range entries {
			if filepathHasBakSuffix(e.Name()) {
				hasBak = true
			}
		}
		if !hasBak {
			t.Error("no .bak file found after Derive on an existing scores.csv")
		}
	})

	t.Run("case 9: row count passes through", func(t *testing.T) {
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{deriveResult: DeriveResult{Rows: 7, ScoresCSVPath: "/tmp/s.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "scores", "--json", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, `"rows":7`) {
			t.Errorf("stdout = %s, want rows:7", stdout)
		}
	})
}
