package config

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{name: "default", cfg: Default()},
		{name: "bad usage", cfg: &Config{Usage: UsageConfig{Enabled: UsageEnabled("banana")}}, want: "usage.enabled"},
		{name: "bad backend", cfg: &Config{Usage: UsageConfig{Backend: UsageBackend("banana")}}, want: "usage.backend"},
		{name: "zero weight", cfg: &Config{Providers: map[string]ProviderConfig{"claude": {Weight: decimal.Decimal{}}}}},
		{name: "negative weight", cfg: &Config{Providers: map[string]ProviderConfig{"claude": {Weight: decimal.NewFromInt(-1)}}}, want: "providers.claude.weight"},
		{name: "negative ttl", cfg: &Config{Providers: map[string]ProviderConfig{"claude": {CacheTTL: -time.Second}}}, want: "providers.claude.cache_ttl"},
		{name: "empty provider", cfg: &Config{Providers: map[string]ProviderConfig{"": {}}}, want: "providers.<empty>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				if tt.name == "zero weight" && tt.cfg.Providers["claude"].Weight.String() != "1" {
					t.Fatalf("weight = %q", tt.cfg.Providers["claude"].Weight.String())
				}
				return
			}
			var ce *ConfigError
			if !errors.As(err, &ce) || ce.Key != tt.want {
				t.Fatalf("error = %v, want key %q", err, tt.want)
			}
		})
	}
}

func TestValidateLoadedConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config.toml", "[usage]\nenabled = true\n[providers.claude]\nenabled = true\nweight = 0.85\n")
	cfg := Default()
	if err := cfg.DecodeFile(path); err != nil {
		t.Fatalf("DecodeFile: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Usage.Enabled != UsageTrue || !cfg.Providers["claude"].Enabled {
		t.Fatalf("cfg = %#v", cfg)
	}
}
