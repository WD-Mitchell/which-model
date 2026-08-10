package config

import (
	"errors"
	"testing"
	"time"
)

func TestApplyEnv(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "usage true",
			env:  map[string]string{"WHICH_MODEL_USAGE_ENABLED": "true"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("Usage.Enabled = %q", cfg.Usage.Enabled)
				}
			},
		},
		{
			name: "usage backend",
			env:  map[string]string{"WHICH_MODEL_USAGE_BACKEND": "codexbar"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Usage.Backend != UsageBackendCodexBar {
					t.Fatalf("Usage.Backend = %q", cfg.Usage.Backend)
				}
			},
		},
		{
			name:  "usage invalid",
			env:   map[string]string{"WHICH_MODEL_USAGE_ENABLED": "banana"},
			check: func(t *testing.T, cfg *Config) {},
		},
		{
			name: "claude enabled",
			env:  map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED": "true"},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Providers["claude"].Enabled {
					t.Fatal("claude is not enabled")
				}
			},
		},
		{
			name: "github copilot enabled",
			env:  map[string]string{"WHICH_MODEL_PROVIDERS_GITHUB_COPILOT_ENABLED": "true"},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Providers["github-copilot"].Enabled {
					t.Fatal("github-copilot is not enabled")
				}
			},
		},
		{
			name: "priority",
			env:  map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_PRIORITY": "5"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Providers["claude"].Priority != 5 {
					t.Fatalf("Priority = %d", cfg.Providers["claude"].Priority)
				}
			},
		},
		{
			name: "weight",
			env:  map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_WEIGHT": "0.85"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Providers["claude"].Weight.String() != "0.85" {
					t.Fatalf("Weight = %q", cfg.Providers["claude"].Weight.String())
				}
			},
		},
		{
			name: "cache ttl",
			env:  map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_CACHE_TTL": "5m"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Providers["claude"].CacheTTL != 5*time.Minute {
					t.Fatalf("CacheTTL = %v", cfg.Providers["claude"].CacheTTL)
				}
			},
		},
		{
			name:  "cache ttl invalid",
			env:   map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_CACHE_TTL": "banana"},
			check: func(t *testing.T, cfg *Config) {},
		},
		{
			name:  "enabled invalid",
			env:   map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED": "banana"},
			check: func(t *testing.T, cfg *Config) {},
		},
		{
			name:  "unknown provider key",
			env:   map[string]string{"WHICH_MODEL_PROVIDERS_CLAUDE_BANANA": "1"},
			check: func(t *testing.T, cfg *Config) {},
		},
		{
			name:  "unknown generic key",
			env:   map[string]string{"WHICH_MODEL_CATALOG_BANANA": "1"},
			check: func(t *testing.T, cfg *Config) {},
		},
		{
			name: "generic dotted key",
			env:  map[string]string{"WHICH_MODEL_BANDS_DIRECTION": "spread"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.env["bands.direction"] != "spread" {
					t.Fatalf("bands.direction = %q", cfg.env["bands.direction"])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			values := tt.env
			environ := make([]string, 0, len(values))
			for name, value := range values {
				environ = append(environ, name+"="+value)
			}
			getenv := func(name string) string { return values[name] }
			err := ApplyEnv(cfg, getenv, environ)
			wantErr := tt.name == "usage invalid" || tt.name == "cache ttl invalid" || tt.name == "enabled invalid" || tt.name == "unknown provider key" || tt.name == "unknown generic key"
			if wantErr {
				var ce *ConfigError
				if !errors.As(err, &ce) || ce.ExitCode() != 2 {
					t.Fatalf("error = %v", err)
				}
				wantKey := map[string]string{
					"usage invalid":        "WHICH_MODEL_USAGE_ENABLED",
					"cache ttl invalid":    "WHICH_MODEL_PROVIDERS_CLAUDE_CACHE_TTL",
					"enabled invalid":      "WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED",
					"unknown provider key": "WHICH_MODEL_PROVIDERS_CLAUDE_BANANA",
					"unknown generic key":  "WHICH_MODEL_CATALOG_BANANA",
				}[tt.name]
				if ce.Key != wantKey {
					t.Fatalf("Key = %q, want %q", ce.Key, wantKey)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyEnv: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}
