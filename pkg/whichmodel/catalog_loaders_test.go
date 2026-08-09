package whichmodel

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestLoadProviderConfig(t *testing.T) {
	t.Run("case 1: happy", func(t *testing.T) {
		path := writeTOML(t, "[providers.anthropic]\nexcluded_models = [\"grok-4.5\"]\n[providers.z]\nexcluded_models = []\n")
		got, err := loadProviderConfig(path)
		if err != nil {
			t.Fatalf("loadProviderConfig() error = %v", err)
		}
		want := map[string][]string{"anthropic": {"grok-4.5"}, "z": {}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %+v, want %+v", got, want)
		}
	})

	t.Run("case 2: blank id", func(t *testing.T) {
		path := writeTOML(t, "[providers.\"\"]\nexcluded_models = []\n")
		if _, err := loadProviderConfig(path); err == nil {
			t.Error("loadProviderConfig() error = nil, want error")
		}
	})

	t.Run("case 3: blank excluded entry", func(t *testing.T) {
		path := writeTOML(t, "[providers.x]\nexcluded_models = [\"\"]\n")
		if _, err := loadProviderConfig(path); err == nil {
			t.Error("loadProviderConfig() error = nil, want error")
		}
	})

	t.Run("case 4: duplicate excluded", func(t *testing.T) {
		path := writeTOML(t, "[providers.x]\nexcluded_models = [\"a\", \"a\"]\n")
		if _, err := loadProviderConfig(path); err == nil {
			t.Error("loadProviderConfig() error = nil, want error")
		}
	})

	t.Run("case 5: unknown key", func(t *testing.T) {
		path := writeTOML(t, "[providers.x]\nbogus = 1\n")
		_, err := loadProviderConfig(path)
		if err == nil {
			t.Fatal("loadProviderConfig() error = nil, want error")
		}
	})

	t.Run("case 6: missing file", func(t *testing.T) {
		_, err := loadProviderConfig(filepath.Join(t.TempDir(), "missing.toml"))
		if err == nil || !strings.HasPrefix(err.Error(), "provider config not found at") {
			t.Errorf("loadProviderConfig() error = %v, want prefix 'provider config not found at'", err)
		}
	})
}

func TestLoadBenchmarkConfig(t *testing.T) {
	t.Run("case 7: happy expansion", func(t *testing.T) {
		content := `[benchmark_selection]
groups = ["g1", "g2"]
benchmarks = ["shared", "direct"]

[benchmark_groups.g1]
benchmarks = ["a", "shared"]

[benchmark_groups.g2]
benchmarks = ["b"]
`
		path := writeTOML(t, content)
		got, err := loadBenchmarkConfig(path)
		if err != nil {
			t.Fatalf("loadBenchmarkConfig() error = %v", err)
		}
		want := []string{"a", "shared", "b", "direct"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %v, want %v", got, want)
		}
	})

	t.Run("case 8: group without table", func(t *testing.T) {
		path := writeTOML(t, "[benchmark_selection]\ngroups = [\"g\"]\n")
		if _, err := loadBenchmarkConfig(path); err == nil {
			t.Error("loadBenchmarkConfig() error = nil, want error")
		}
	})

	t.Run("case 9: duplicate in list", func(t *testing.T) {
		path := writeTOML(t, "[benchmark_selection]\nbenchmarks = [\"a\", \"a\"]\n")
		if _, err := loadBenchmarkConfig(path); err == nil {
			t.Error("loadBenchmarkConfig() error = nil, want error")
		}
	})

	t.Run("case 10: blank entry", func(t *testing.T) {
		path := writeTOML(t, "[benchmark_selection]\nbenchmarks = [\"\"]\n")
		if _, err := loadBenchmarkConfig(path); err == nil {
			t.Error("loadBenchmarkConfig() error = nil, want error")
		}
	})

	t.Run("case 11: unknown key", func(t *testing.T) {
		path := writeTOML(t, "[benchmark_selection]\nnope = 1\n")
		if _, err := loadBenchmarkConfig(path); err == nil {
			t.Error("loadBenchmarkConfig() error = nil, want error")
		}
	})

	t.Run("case 12: missing file", func(t *testing.T) {
		_, err := loadBenchmarkConfig(filepath.Join(t.TempDir(), "missing.toml"))
		if err == nil || !strings.HasPrefix(err.Error(), "benchmarks config not found at") {
			t.Errorf("loadBenchmarkConfig() error = %v, want prefix 'benchmarks config not found at'", err)
		}
	})
}
