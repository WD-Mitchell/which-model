package whichmodel

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeStageRunner records calls and returns canned results/errors.
type fakeStageRunner struct {
	collectCalls  int
	collectResult CollectResult
	collectErr    error
	deriveCalls   int
	deriveResult  DeriveResult
	deriveErr     error
	callOrder     []string
	capturedCO    CollectOptions
	capturedDO    DeriveOptions
}

func (f *fakeStageRunner) Collect(ctx context.Context, o CollectOptions) (CollectResult, error) {
	f.collectCalls++
	f.callOrder = append(f.callOrder, "collect")
	f.capturedCO = o
	return f.collectResult, f.collectErr
}

func (f *fakeStageRunner) Derive(ctx context.Context, o DeriveOptions) (DeriveResult, error) {
	f.deriveCalls++
	f.callOrder = append(f.callOrder, "derive")
	f.capturedDO = o
	return f.deriveResult, f.deriveErr
}

func TestRunStagesOrder(t *testing.T) {
	t.Run("case 1: order enforced, defensive reorder", func(t *testing.T) {
		r := &fakeStageRunner{}
		report, err := runStages(context.Background(), r, fakeKeyResolver(""), "", &GlobalFlags{}, []Stage{StageDerive, StageCollect}, CollectOptions{}, DeriveOptions{})
		if err != nil {
			t.Fatalf("runStages() error = %v", err)
		}
		if len(r.callOrder) != 2 || r.callOrder[0] != "collect" || r.callOrder[1] != "derive" {
			t.Errorf("callOrder = %v, want [collect derive]", r.callOrder)
		}
		if report.Collect == nil || report.Derive == nil {
			t.Errorf("report = %+v, want both stages present", report)
		}
	})

	t.Run("case 2: collect failure aborts derive", func(t *testing.T) {
		r := &fakeStageRunner{collectErr: errBoom}
		_, err := runStages(context.Background(), r, fakeKeyResolver(""), "", &GlobalFlags{}, []Stage{StageCollect, StageDerive}, CollectOptions{}, DeriveOptions{})
		if err == nil {
			t.Fatal("runStages() error = nil, want error")
		}
		if r.deriveCalls != 0 {
			t.Errorf("deriveCalls = %d, want 0", r.deriveCalls)
		}
	})

	t.Run("case 3: offline refusal", func(t *testing.T) {
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "refresh", "--offline"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		want := "which-model catalog refresh: [arguments] Collect requires network access; incompatible with --offline"
		if strings.TrimSpace(stderr) != want {
			t.Errorf("stderr = %q, want %q", strings.TrimSpace(stderr), want)
		}
	})

	t.Run("case 4: key refusal", func(t *testing.T) {
		r := &fakeStageRunner{}
		_, err := runStages(context.Background(), r, fakeKeyResolver("fail"), "", &GlobalFlags{}, []Stage{StageCollect}, CollectOptions{}, DeriveOptions{})
		if err == nil {
			t.Fatal("runStages() error = nil, want error")
		}
		var ue *UsageError
		if !isUsageError(err, &ue) {
			t.Fatalf("error = %T, want *UsageError", err)
		}
		want := "ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key"
		if ue.Message != want {
			t.Errorf("message = %q, want %q", ue.Message, want)
		}
	})

	t.Run("case 5: key resolved once and passed through", func(t *testing.T) {
		r := &fakeStageRunner{}
		calls := 0
		resolver := func(repoRoot string) (string, error) {
			calls++
			return "the-key", nil
		}
		_, err := runStages(context.Background(), r, resolver, "", &GlobalFlags{}, []Stage{StageCollect}, CollectOptions{}, DeriveOptions{})
		if err != nil {
			t.Fatalf("runStages() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("resolver calls = %d, want 1", calls)
		}
		if r.capturedCO.AAKey != "the-key" {
			t.Errorf("capturedCO.AAKey = %q, want the-key", r.capturedCO.AAKey)
		}
	})
}

func fakeKeyResolver(mode string) AAKeyResolver {
	return func(repoRoot string) (string, error) {
		if mode == "fail" {
			return "", errBoom
		}
		return "test-key", nil
	}
}

var errBoom = &UsageError{Message: "boom"}

func TestRefreshOutput(t *testing.T) {
	t.Run("case 6: text output", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{
			collectResult: CollectResult{Providers: 2, Models: 21, RawCSVPath: "/tmp/raw.csv"},
			deriveResult:  DeriveResult{Rows: 39, ScoresCSVPath: "/tmp/scores.csv"},
		}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "refresh", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stdout=%s", code, stdout)
		}
		wantLine1 := "collected 2 providers, 21 models -> /tmp/raw.csv"
		wantLine2 := "derived 39 rows -> /tmp/scores.csv"
		if !strings.Contains(stdout, wantLine1) || !strings.Contains(stdout, wantLine2) {
			t.Errorf("stdout = %q, want lines %q and %q", stdout, wantLine1, wantLine2)
		}
	})

	t.Run("case 7: json output", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{
			collectResult: CollectResult{Providers: 2, Models: 21, RawCSVPath: "/tmp/raw.csv"},
			deriveResult:  DeriveResult{Rows: 39, ScoresCSVPath: "/tmp/scores.csv"},
		}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "refresh", "--json", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stdout=%s", code, stdout)
		}
		if !strings.Contains(stdout, `"providers":2`) || !strings.Contains(stdout, `"models":21`) || !strings.Contains(stdout, `"rows":39`) || !strings.Contains(stdout, `"schema_version":"2.0"`) {
			t.Errorf("stdout = %s, want collect/derive/schema fields", stdout)
		}
	})

	t.Run("case 8: flags passthrough", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		providersPath := filepath.Join(dir, "providers.toml")
		if err := os.WriteFile(providersPath, []byte("[providers.a]\nexcluded_models=[]\n[providers.b]\nexcluded_models=[]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		benchPath := filepath.Join(dir, "b.toml")
		if err := os.WriteFile(benchPath, []byte("[benchmark_selection]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "refresh",
			"--config", writeMinimalConfig(t, dir),
			"--provider-config", providersPath,
			"--provider", "a", "--provider", "b",
			"--add", "aa_page",
			"--out", filepath.Join(dir, "x.csv"),
			"--benchmarks", benchPath,
			"--timeout", "5s",
		})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if len(fake.capturedCO.Providers) != 2 {
			t.Errorf("capturedCO.Providers = %v", fake.capturedCO.Providers)
		}
		if fake.capturedCO.Timeout.String() != "5s" {
			t.Errorf("capturedCO.Timeout = %v, want 5s", fake.capturedCO.Timeout)
		}
		if fake.capturedDO.OutPath != filepath.Join(dir, "x.csv") {
			t.Errorf("capturedDO.OutPath = %q", fake.capturedDO.OutPath)
		}
	})
}

func writeMinimalConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func writeEmptyProviders(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "providers-empty.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestCountRows(t *testing.T) {
	t.Run("case 9", func(t *testing.T) {
		n, err := countRows([]byte("# which-model-scores-provenance raw_sha256=abc\nmodel,reasoning\nA,high\nB,low\n"))
		if err != nil || n != 2 {
			t.Errorf("countRows() = (%d, %v), want (2, nil)", n, err)
		}
		n, err = countRows([]byte("model\n"))
		if err != nil || n != 0 {
			t.Errorf("countRows() = (%d, %v), want (0, nil)", n, err)
		}
	})
}

func TestDefaultRunnerDerive(t *testing.T) {
	t.Run("case 10: unknown normalizer", func(t *testing.T) {
		dir := t.TempDir()
		raw := filepath.Join(dir, "raw.csv")
		bench := filepath.Join(dir, "b.toml")
		if err := os.WriteFile(raw, []byte("model,reasoning\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := os.WriteFile(bench, []byte("[benchmark_selection]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: raw, BenchmarksPath: bench, Normalizer: "bogus", OutPath: filepath.Join(dir, "s.csv")})
		if err == nil {
			t.Fatal("Derive() error = nil, want error")
		}
	})

	t.Run("case 11: missing raw", func(t *testing.T) {
		dir := t.TempDir()
		_, err := (defaultRunner{}).Derive(context.Background(), DeriveOptions{InPath: filepath.Join(dir, "missing.csv")})
		if err == nil || !strings.HasPrefix(err.Error(), "raw CSV not found at") {
			t.Errorf("Derive() error = %v, want prefix 'raw CSV not found at'", err)
		}
	})

	t.Run("case 12: missing benchmarks", func(t *testing.T) {
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
}

var _ = strconv.Itoa
