package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestMarshalTOML(t *testing.T) {
	tests := []struct {
		name string
		file string
		env  map[string]string
		check func(t *testing.T, out string)
	}{
		{
			name: "default",
			check: func(t *testing.T, out string) {
				if !strings.HasPrefix(out, "[usage]") || !strings.Contains(out, "enabled = \"auto\"") || strings.Contains(out, "[providers") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "usage false",
			file: "[usage]\nenabled = false\n",
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "enabled = false") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "provider",
			file: "[providers.claude]\nenabled = true\npriority = 5\nweight = 0.85\ncache_ttl = \"5m\"\nsource_preference = [\"native\"]\ncredential_path = \"/x\"\n",
			check: func(t *testing.T, out string) {
				for _, want := range []string{"[providers.claude]", "enabled = true", "priority = 5", "weight = \"0.85\"", "cache_ttl = \"5m\"", "source_preference = [\"native\"]", "credential_path = \"/x\""} {
					if !strings.Contains(out, want) {
						t.Fatalf("out = %q, missing %q", out, want)
					}
				}
			},
		},
		{
			name: "scoring",
			file: "[scoring]\nnormalizer = \"minmax-linear\"\naggregator = \"weighted-arithmetic-mean\"\n",
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "[scoring]") || !strings.Contains(out, "normalizer = \"minmax-linear\"") || !strings.Contains(out, "aggregator = \"weighted-arithmetic-mean\"") {
				t.Fatalf("out = %q", out)
			}
			},
		},
		{
			name: "env overrides file",
			file: "[catalog]\ncache_ttl = \"30m\"\n",
			env:  map[string]string{"catalog.cache_ttl": "1h"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "[catalog]") || !strings.Contains(out, "cache_ttl = \"1h\"") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "env int",
			env:  map[string]string{"strategy.tier1_share": "80"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "tier1_share = 80") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "env bool",
			env:  map[string]string{"catalog.warn_on_stale_scores": "false"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "warn_on_stale_scores = false") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "env nested",
			env:  map[string]string{"catalog.publish.schedule": "0 6 * * *"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "[catalog.publish]") || !strings.Contains(out, "schedule = \"0 6 * * *\"") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "round trip",
			env:  map[string]string{"catalog.cache_ttl": "1h"},
			check: func(t *testing.T, out string) {
				var doc map[string]any
				if _, err := toml.Decode(out, &doc); err != nil {
					t.Fatalf("toml.Decode: %v", err)
				}
				if got := rawLookup(doc, "catalog.cache_ttl"); got != "1h" {
					t.Fatalf("cache_ttl = %#v", got)
				}
			},
		},
		{
			name: "canonical order",
			file: "[usage]\nenabled = true\n[providers.claude]\nenabled = true\n",
			check: func(t *testing.T, out string) {
				if strings.Index(out, "[usage]") >= strings.Index(out, "[providers.claude]") {
					t.Fatalf("out = %q", out)
				}
			},
		},
		{
			name: "array of tables",
			file: "[[bands.tier]]\nweight = 1.0\n",
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "[[bands.tier]]") {
					t.Fatalf("out = %q", out)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			if tt.file != "" {
				path := writeFile(t, t.TempDir(), "config.toml", tt.file)
				if err := cfg.DecodeFile(path); err != nil {
					t.Fatalf("DecodeFile: %v", err)
				}
			}
			if tt.env != nil {
				cfg.env = tt.env
			}
			out, err := cfg.MarshalTOML()
			if err != nil {
				t.Fatalf("MarshalTOML: %v", err)
			}
			tt.check(t, string(out))
		})
	}
}
