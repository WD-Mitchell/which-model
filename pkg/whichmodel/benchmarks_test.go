package whichmodel

import (
	"os"
	"strings"
	"testing"
)

func TestBenchmarksCmd(t *testing.T) {
	t.Run("case 1: happy text", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{collectResult: CollectResult{Providers: 2, Models: 21, RawCSVPath: "/tmp/raw.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, "collected 2 providers, 21 models -> /tmp/raw.csv") {
			t.Errorf("stdout = %q", stdout)
		}
		if fake.deriveCalls != 0 {
			t.Errorf("deriveCalls = %d, want 0", fake.deriveCalls)
		}
	})

	t.Run("case 2: json shape, no derive key", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{collectResult: CollectResult{Providers: 2, Models: 21, RawCSVPath: "/tmp/raw.csv"}}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, stdout, _ := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--json", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(stdout, `"providers":2`) || !strings.Contains(stdout, `"models":21`) {
			t.Errorf("stdout = %s", stdout)
		}
		if strings.Contains(stdout, `"rows"`) {
			t.Errorf("stdout = %s, want no derive/rows key", stdout)
		}
	})

	t.Run("case 3: offline refusal", func(t *testing.T) {
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--offline"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		want := "which-model catalog benchmarks: [arguments] Collect requires network access; incompatible with --offline"
		if strings.TrimSpace(stderr) != want {
			t.Errorf("stderr = %q, want %q", strings.TrimSpace(stderr), want)
		}
	})

	t.Run("case 4: key refusal", func(t *testing.T) {
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 2 {
			t.Errorf("exit = %d, want 2; stderr=%s", code, stderr)
		}
		want := "which-model catalog benchmarks: [arguments] ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key"
		if strings.TrimSpace(stderr) != want {
			t.Errorf("stderr = %q, want %q", strings.TrimSpace(stderr), want)
		}
	})

	t.Run("case 5: aa page flag", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--add", "aa_page", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if !fake.capturedCO.AddAAPage {
			t.Error("capturedCO.AddAAPage = false, want true")
		}
	})

	t.Run("case 6: unknown add value", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--add", "nope"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if !strings.Contains(stderr, "[arguments]") {
			t.Errorf("stderr = %q", stderr)
		}
		if fake.collectCalls != 0 {
			t.Errorf("collectCalls = %d, want 0", fake.collectCalls)
		}
	})

	t.Run("case 7: provider passthrough", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		providersPath := writeEmptyProviders(t, dir)
		if err := writeFileHelper(t, providersPath, "[providers.anthropic]\nexcluded_models=[]\n"); err != nil {
			t.Fatalf("write error = %v", err)
		}
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "benchmarks", "--config", writeMinimalConfig(t, dir), "--provider-config", providersPath, "--provider", "anthropic"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		if len(fake.capturedCO.Providers) != 1 || fake.capturedCO.Providers[0] != "anthropic" {
			t.Errorf("capturedCO.Providers = %v", fake.capturedCO.Providers)
		}
	})

	t.Run("case 8: scores never derived", func(t *testing.T) {
		t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
		orig := newRunner
		defer func() { newRunner = orig }()
		fake := &fakeStageRunner{}
		newRunner = func() StageRunner { return fake }
		dir := t.TempDir()
		captureExecuteFresh(t, []string{"catalog", "benchmarks", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if fake.deriveCalls != 0 {
			t.Errorf("deriveCalls = %d, want 0", fake.deriveCalls)
		}
	})
}

func writeFileHelper(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o644)
}
