package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

type testScoring struct {
	Normalizer string `toml:"normalizer"`
	Aggregator string `toml:"aggregator"`
}

type testCatalog struct {
	CacheTTL time.Duration    `toml:"cache_ttl"`
	Budget   *decimal.Decimal `toml:"budget"`
	Publish  struct {
		Schedule string `toml:"schedule"`
	} `toml:"publish"`
}

type testCustom struct {
	Weight decimal.Decimal `toml:"weight"`
}

func TestUnmarshalKey(t *testing.T) {
	tests := []struct {
		name string
		file string
		run  func(t *testing.T, cfg *Config)
	}{
		{
			name: "scoring",
			file: "[scoring]\nnormalizer = \"minmax-linear\"\naggregator = \"weighted-arithmetic-mean\"\n",
			run: func(t *testing.T, cfg *Config) {
				var got testScoring
				if err := cfg.UnmarshalKey("scoring", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.Normalizer != "minmax-linear" || got.Aggregator != "weighted-arithmetic-mean" {
					t.Fatalf("got %#v", got)
				}
			},
		},
		{
			name: "nested",
			file: "[catalog.publish]\nschedule = \"0 6 * * *\"\n",
			run: func(t *testing.T, cfg *Config) {
				var got testCatalog
				if err := cfg.UnmarshalKey("catalog", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.Publish.Schedule != "0 6 * * *" {
					t.Fatalf("schedule = %q", got.Publish.Schedule)
				}
			},
		},
		{
			name: "missing",
			file: "[scoring]\nnormalizer = \"minmax-linear\"\n",
			run: func(t *testing.T, cfg *Config) {
				var got testScoring
				if err := cfg.UnmarshalKey("nonexistent", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got != (testScoring{}) {
					t.Fatalf("got %#v", got)
				}
			},
		},
		{
			name: "not-table",
			file: "cache_ttl = \"1h\"\n",
			run: func(t *testing.T, cfg *Config) {
				var got testScoring
				err := cfg.UnmarshalKey("cache_ttl", &got)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Kind != KindInvalidValue || ce.ExitCode() != 2 {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "unknown-flat",
			file: "[scoring]\nnormalizer = \"minmax-linear\"\nbanana = 1\n",
			run: func(t *testing.T, cfg *Config) {
				var got testScoring
				err := cfg.UnmarshalKey("scoring", &got)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Kind != KindInvalidValue || ce.Key != "scoring.banana" {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "unknown-nested",
			file: "[catalog.publish]\nschedule = \"0 6 * * *\"\nbanana = 1\n",
			run: func(t *testing.T, cfg *Config) {
				var got testCatalog
				err := cfg.UnmarshalKey("catalog", &got)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Key != "catalog.publish.banana" {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "decimal",
			file: "[custom]\nweight = 0.85\n",
			run: func(t *testing.T, cfg *Config) {
				var got testCustom
				if err := cfg.UnmarshalKey("custom", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.Weight.String() != "0.85" {
					t.Fatalf("weight = %q", got.Weight.String())
				}
			},
		},
		{
			name: "malformed",
			file: "[usage\n",
			run: func(t *testing.T, cfg *Config) {
				err := cfg.DecodeFile(filepath.Join(t.TempDir(), "unused"))
				_ = err
			},
		},
		{
			name: "env-overlay",
			file: "[catalog]\n",
			run: func(t *testing.T, cfg *Config) {
				cfg.env = map[string]string{"catalog.cache_ttl": "1h", "catalog.budget": "2.5"}
				var got testCatalog
				if err := cfg.UnmarshalKey("catalog", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.CacheTTL != time.Hour || got.Budget == nil || got.Budget.String() != "2.5" {
					t.Fatalf("got %#v", got)
				}
			},
		},
		{
			name: "env-overlay-without-file-section",
			file: "",
			run: func(t *testing.T, cfg *Config) {
				cfg.env = map[string]string{"catalog.cache_ttl": "1h"}
				var got testCatalog
				if err := cfg.UnmarshalKey("catalog", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.CacheTTL != time.Hour {
					t.Fatalf("cache TTL = %s, want 1h", got.CacheTTL)
				}
			},
		},
		{
			name: "env-parse-error",
			file: "[catalog]\n",
			run: func(t *testing.T, cfg *Config) {
				cfg.env = map[string]string{"catalog.cache_ttl": "banana"}
				var got testCatalog
				err := cfg.UnmarshalKey("catalog", &got)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Key != "catalog.cache_ttl" {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "env-typo",
			file: "[catalog]\n",
			run: func(t *testing.T, cfg *Config) {
				cfg.env = map[string]string{"catalog.cache_tl": "1h"}
				var got testCatalog
				err := cfg.UnmarshalKey("catalog", &got)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Key != "catalog.cache_tl" {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "caller-default",
			file: "[scoring]\naggregator = \"weighted-arithmetic-mean\"\n",
			run: func(t *testing.T, cfg *Config) {
				got := testScoring{Normalizer: "keep-me"}
				if err := cfg.UnmarshalKey("scoring", &got); err != nil {
					t.Fatalf("UnmarshalKey: %v", err)
				}
				if got.Normalizer != "keep-me" {
					t.Fatalf("normalizer = %q", got.Normalizer)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "config.toml", tt.file)
			cfg := Default()
			if tt.name == "malformed" {
				err := cfg.DecodeFile(path)
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.Kind != KindInvalidTOML {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err := cfg.DecodeFile(path); err != nil {
				t.Fatalf("DecodeFile: %v", err)
			}
			tt.run(t, cfg)
		})
	}
}
