package config

import (
	"github.com/shopspring/decimal"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestMarshalTOML(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		env   map[string]string
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

func TestMarshalEnvScalarTypesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		key, value string
		want       any
	}{
		{"strategy.tier1_share", "1", int64(1)}, {"strategy.tier2_share", "0", int64(0)},
		{"strategy.tier1_share", "100", int64(100)}, {"strategy.tier2_share", "99", int64(99)},
		{"bands.gate_above_used_percent", "0", "0"}, {"bands.gate_above_used_percent", "1", "1"},
		{"output.identity_default", "0", false}, {"output.identity_default", "1", true},
		{"catalog.publish.pr_title", "true", "true"}, {"catalog.publish.pr_title", "0", "0"}, {"catalog.publish.pr_title", "001", "001"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			c := Default()
			c.env = map[string]string{tc.key: tc.value}
			data, err := c.MarshalTOML()
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := toml.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			var got any = doc
			for _, segment := range strings.Split(tc.key, ".") {
				got = got.(map[string]any)[segment]
			}
			if got != tc.want {
				t.Fatalf("rendered %T(%v), want %T(%v)", got, got, tc.want, tc.want)
			}
			if c.env[tc.key] != tc.value {
				t.Fatal("render mutated config")
			}
		})
	}
}

func TestMarshalRejectsInvalidTypedEnv(t *testing.T) {
	for _, key := range []string{"strategy.tier1_share", "output.identity_default"} {
		c := Default()
		c.env = map[string]string{key: "invalid"}
		if _, err := c.MarshalTOML(); err == nil {
			t.Errorf("%s: wanted invalid value error", key)
		}
	}
}

func TestRenderedEnvironmentReloadsThroughOwningSchemas(t *testing.T) {
	cfg := Default()
	path := filepath.Join(t.TempDir(), "source.toml")
	if err := os.WriteFile(path, []byte("[strategy]\ndefault = \"priority\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.DecodeFile(path); err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"WHICH_MODEL_STRATEGY_TIER1_SHARE": "1", "WHICH_MODEL_STRATEGY_TIER2_SHARE": "99", "WHICH_MODEL_BANDS_GATE_ABOVE_USED_PERCENT": "0", "WHICH_MODEL_OUTPUT_IDENTITY_DEFAULT": "1", "WHICH_MODEL_CATALOG_PUBLISH_PR_TITLE": "001"}
	entries := make([]string, 0, len(vars))
	for k, v := range vars {
		entries = append(entries, k+"="+v)
	}
	if err := ApplyEnv(cfg, func(k string) string { return vars[k] }, entries); err != nil {
		t.Fatal(err)
	}
	data, err := cfg.MarshalTOML()
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(t.TempDir(), "saved.toml")
	if err := os.WriteFile(saved, data, 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Default string `toml:"default"`
		Tier1   int    `toml:"tier1_share"`
		Tier2   int    `toml:"tier2_share"`
	}
	if err := reloaded.UnmarshalKey("strategy", &st); err != nil || st.Default != "priority" || st.Tier1 != 1 || st.Tier2 != 99 {
		t.Fatalf("strategy: %+v, %v", st, err)
	}
	var band struct {
		Gate decimal.Decimal `toml:"gate_above_used_percent"`
	}
	if err := reloaded.UnmarshalKey("bands", &band); err != nil || !band.Gate.IsZero() {
		t.Fatalf("band: %+v, %v", band, err)
	}
	var output struct {
		Identity bool `toml:"identity_default"`
	}
	if err := reloaded.UnmarshalKey("output", &output); err != nil || !output.Identity {
		t.Fatalf("output: %+v, %v", output, err)
	}
	var publish struct {
		Title string `toml:"pr_title"`
	}
	if err := reloaded.UnmarshalKey("catalog.publish", &publish); err != nil || publish.Title != "001" {
		t.Fatalf("publish: %+v, %v", publish, err)
	}
	var original struct {
		Default string `toml:"default"`
		Tier1   int    `toml:"tier1_share"`
		Tier2   int    `toml:"tier2_share"`
	}
	if err := cfg.UnmarshalKey("strategy", &original); err != nil || original != st {
		t.Fatalf("original config changed: %+v %v", original, err)
	}
}
