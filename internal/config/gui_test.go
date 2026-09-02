package config

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

var testCategories = []string{"agentic_coding", "software_engineering"}

func loadFixture(t *testing.T, file string) *Config {
	t.Helper()
	cfg := Default()
	if file != "" {
		path := writeFile(t, t.TempDir(), "config.toml", file)
		if err := cfg.DecodeFile(path); err != nil {
			t.Fatalf("DecodeFile: %v", err)
		}
	}
	return cfg
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		load    func(c *Config) error
		wantErr string
	}{
		{
			name:    "P1 profile slug",
			file:    "[profiles.Bad]\ncore_share = 60\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.Bad: slug must match [a-z0-9_]+",
		},
		{
			name:    "P2 core_share range",
			file:    "[profiles.p]\ncore_share = 5\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.core_share: must be between 10 and 90",
		},
		{
			name:    "P2 core_share missing",
			file:    "[profiles.p]\n[profiles.p.tier1]\nintelligence = 3\ncost = 3\nspeed = 3\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.core_share: must be between 10 and 90",
		},
		{
			name:    "P3 core_share step",
			file:    "[profiles.p]\ncore_share = 62\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.core_share: must be a multiple of 5",
		},
		{
			name:    "P4 tier1 missing key",
			file:    "[profiles.p]\ncore_share = 60\n[profiles.p.tier1]\nintelligence = 3\ncost = 3\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.tier1: keys must be exactly intelligence, cost, speed",
		},
		{
			name:    "P4 tier1 absent",
			file:    "[profiles.p]\ncore_share = 60\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.tier1: keys must be exactly intelligence, cost, speed",
		},
		{
			name:    "P5 tier1 range",
			file:    "[profiles.p]\ncore_share = 60\n[profiles.p.tier1]\nintelligence = 6\ncost = 3\nspeed = 3\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.tier1.intelligence: must be between 1 and 5",
		},
		{
			name:    "P6 tier2 unknown category",
			file:    "[profiles.p]\ncore_share = 60\n[profiles.p.tier1]\nintelligence = 3\ncost = 3\nspeed = 3\n[profiles.p.tier2]\nbanana = 3\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.tier2.banana: unknown tier2 category",
		},
		{
			name:    "P7 tier2 range",
			file:    "[profiles.p]\ncore_share = 60\n[profiles.p.tier1]\nintelligence = 3\ncost = 3\nspeed = 3\n[profiles.p.tier2]\nsoftware_engineering = 0\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.tier2.software_engineering: must be between 1 and 5",
		},
		{
			name:    "H1 harness slug",
			file:    "[harnesses.\"bad-slug\"]\nname = \"x\"\ncommand = \"x\"\n",
			load:    func(c *Config) error { _, err := c.LoadHarnesses(); return err },
			wantErr: "config: invalid value for harnesses.bad-slug: slug must match [a-z0-9_]+",
		},
		{
			name:    "H2 harness name empty",
			file:    "[harnesses.h]\nname = \"\"\ncommand = \"x\"\n",
			load:    func(c *Config) error { _, err := c.LoadHarnesses(); return err },
			wantErr: "config: invalid value for harnesses.h.name: must not be empty",
		},
		{
			name:    "H3 harness command empty",
			file:    "[harnesses.h]\nname = \"x\"\ncommand = \"\"\n",
			load:    func(c *Config) error { _, err := c.LoadHarnesses(); return err },
			wantErr: "config: invalid value for harnesses.h.command: must not be empty",
		},
		{
			name:    "H4 harness provider slug",
			file:    "[harnesses.h]\nname = \"x\"\ncommand = \"x\"\nproviders = [\"Bad!\"]\n",
			load:    func(c *Config) error { _, err := c.LoadHarnesses(); return err },
			wantErr: "config: invalid value for harnesses.h.providers: provider \"Bad!\" must match [a-z0-9_]+",
		},
		{
			name:    "F1 invalid route key",
			file:    "[favourites]\npins = [\"nope\"]\n",
			load:    func(c *Config) error { _, err := c.LoadFavourites(); return err },
			wantErr: "config: invalid value for favourites.pins: invalid route key \"nope\"",
		},
		{
			name:    "F1 invalid reasoning",
			file:    "[favourites]\npins = [\"claude/claude-opus-5@turbo\"]\n",
			load:    func(c *Config) error { _, err := c.LoadFavourites(); return err },
			wantErr: "config: invalid value for favourites.pins: invalid route key \"claude/claude-opus-5@turbo\"",
		},
		{
			name:    "F2 duplicate pin",
			file:    "[favourites]\npins = [\"claude/claude-opus-5@max\", \"claude/claude-opus-5@max\"]\n",
			load:    func(c *Config) error { _, err := c.LoadFavourites(); return err },
			wantErr: "config: invalid value for favourites.pins: duplicate pin \"claude/claude-opus-5@max\"",
		},
		{
			name:    "R1 provider slug",
			file:    "[routes.disabled]\nBAD = [\"m@low\"]\n",
			load:    func(c *Config) error { _, err := c.LoadRoutesDisabled(); return err },
			wantErr: "config: invalid value for routes.disabled: provider \"BAD\" must match [a-z0-9_]+",
		},
		{
			name:    "R2 invalid route",
			file:    "[routes.disabled]\nclaude = [\"bad route\"]\n",
			load:    func(c *Config) error { _, err := c.LoadRoutesDisabled(); return err },
			wantErr: "config: invalid value for routes.disabled.claude: invalid route \"bad route\"",
		},
		{
			name:    "R3 duplicate route",
			file:    "[routes.disabled]\nclaude = [\"claude-opus-5@low\", \"claude-opus-5@low\"]\n",
			load:    func(c *Config) error { _, err := c.LoadRoutesDisabled(); return err },
			wantErr: "config: invalid value for routes.disabled.claude: duplicate route \"claude-opus-5@low\"",
		},
		{
			name:    "G1 group slug",
			file:    "[groups.Bad]\nbenchmarks = [\"x\"]\n",
			load:    func(c *Config) error { _, err := c.LoadGroups(); return err },
			wantErr: "config: invalid value for groups.Bad: slug must match [a-z0-9_]+",
		},
		{
			name:    "G2 benchmarks empty",
			file:    "[groups.g]\nbenchmarks = []\n",
			load:    func(c *Config) error { _, err := c.LoadGroups(); return err },
			wantErr: "config: invalid value for groups.g.benchmarks: must not be empty",
		},
		{
			name:    "G3 benchmark name empty",
			file:    "[groups.g]\nbenchmarks = [\"\"]\n",
			load:    func(c *Config) error { _, err := c.LoadGroups(); return err },
			wantErr: "config: invalid value for groups.g.benchmarks: benchmark name must not be empty",
		},
		{
			name:    "G4 duplicate benchmark",
			file:    "[groups.g]\nbenchmarks = [\"SWE-Bench Verified\", \"SWE-Bench Verified\"]\n",
			load:    func(c *Config) error { _, err := c.LoadGroups(); return err },
			wantErr: "config: invalid value for groups.g.benchmarks: duplicate benchmark \"SWE-Bench Verified\"",
		},
		{
			name:    "U1 layout",
			file:    "[gui]\nlayout = \"grid\"\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.layout: must be \"carousel\" or \"list\"",
		},
		{
			name:    "U2 weight_control",
			file:    "[gui]\nweight_control = \"dial\"\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.weight_control: must be \"step\", \"bar\" or \"slider\"",
		},
		{
			name:    "U3 holds",
			file:    "[gui]\nholds = 7\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.holds: must be 3, 5 or 10",
		},
		{
			name:    "U4 shortcut",
			file:    "[gui]\nshortcut = \"f13\"\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.shortcut: must be \"alt+space\", \"ctrl+space\" or \"cmd+shift+m\"",
		},
		{
			name:    "U5 auto_update_frequency",
			file:    "[gui]\nauto_update_frequency = \"yearly\"\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.auto_update_frequency: must be \"hourly\", \"daily\", \"weekly\" or \"monthly\"",
		},
		{
			name:    "unknown key inside gui",
			file:    "[gui]\nbanana = 1\n",
			load:    func(c *Config) error { _, err := c.LoadGUI(); return err },
			wantErr: "config: invalid value for gui.banana: gui.banana",
		},
		{
			name:    "unknown key inside profile",
			file:    "[profiles.p]\ncore_share = 60\nbanana = 1\n",
			load:    func(c *Config) error { _, err := c.LoadProfiles(testCategories); return err },
			wantErr: "config: invalid value for profiles.p.banana: profiles.p.banana",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadFixture(t, tt.file)
			err := tt.load(cfg)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("err = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSetterValidationErrors(t *testing.T) {
	cfg := Default()
	tests := []struct {
		name    string
		call    func() error
		wantErr string
	}{
		{
			name:    "SetProfile slug",
			call:    func() error { return cfg.SetProfile("Bad", ProfileTOML{CoreShare: 60}, testCategories) },
			wantErr: "config: invalid value for profiles.Bad: slug must match [a-z0-9_]+",
		},
		{
			name: "SetProfile tier1 extra key",
			call: func() error {
				return cfg.SetProfile("p", ProfileTOML{
					CoreShare: 60,
					Tier1:     map[string]int{"intelligence": 3, "cost": 3, "speed": 3, "vibes": 3},
				}, testCategories)
			},
			wantErr: "config: invalid value for profiles.p.tier1: keys must be exactly intelligence, cost, speed",
		},
		{
			name:    "SetHarness name",
			call:    func() error { return cfg.SetHarness("h", HarnessTOML{Command: "x"}) },
			wantErr: "config: invalid value for harnesses.h.name: must not be empty",
		},
		{
			name:    "SetFavourites route key",
			call:    func() error { return cfg.SetFavourites(FavouritesTOML{Pins: []string{"nope"}}) },
			wantErr: "config: invalid value for favourites.pins: invalid route key \"nope\"",
		},
		{
			name: "SetRoutesDisabled route",
			call: func() error {
				return cfg.SetRoutesDisabled(RoutesDisabledTOML{"claude": {"bad route"}})
			},
			wantErr: "config: invalid value for routes.disabled.claude: invalid route \"bad route\"",
		},
		{
			name:    "SetGroup benchmarks",
			call:    func() error { return cfg.SetGroup("g", GroupTOML{}) },
			wantErr: "config: invalid value for groups.g.benchmarks: must not be empty",
		},
		{
			name: "SetGUI layout",
			call: func() error {
				g := DefaultGUIConfig()
				g.Layout = "grid"
				return cfg.SetGUI(g)
			},
			wantErr: "config: invalid value for gui.layout: must be \"carousel\" or \"list\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("err = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadAbsentSections(t *testing.T) {
	cfg := Default()

	gui, err := cfg.LoadGUI()
	if err != nil {
		t.Fatalf("LoadGUI: %v", err)
	}
	if gui != DefaultGUIConfig() {
		t.Fatalf("LoadGUI = %+v, want defaults", gui)
	}

	profiles, err := cfg.LoadProfiles(testCategories)
	if err != nil || len(profiles) != 0 {
		t.Fatalf("LoadProfiles = %v, %v", profiles, err)
	}
	harnesses, err := cfg.LoadHarnesses()
	if err != nil || len(harnesses) != 0 {
		t.Fatalf("LoadHarnesses = %v, %v", harnesses, err)
	}
	favourites, err := cfg.LoadFavourites()
	if err != nil || len(favourites.Pins) != 0 {
		t.Fatalf("LoadFavourites = %v, %v", favourites, err)
	}
	disabled, err := cfg.LoadRoutesDisabled()
	if err != nil || len(disabled) != 0 {
		t.Fatalf("LoadRoutesDisabled = %v, %v", disabled, err)
	}
	groups, err := cfg.LoadGroups()
	if err != nil || len(groups) != 0 {
		t.Fatalf("LoadGroups = %v, %v", groups, err)
	}
}

func TestLoadGUIPerKeyDefaults(t *testing.T) {
	cfg := loadFixture(t, "[gui]\nlayout = \"list\"\n")
	gui, err := cfg.LoadGUI()
	if err != nil {
		t.Fatalf("LoadGUI: %v", err)
	}
	want := DefaultGUIConfig()
	want.Layout = "list"
	if gui != want {
		t.Fatalf("LoadGUI = %+v, want %+v", gui, want)
	}
}

func TestLoadProfilesGroupSlugTier2(t *testing.T) {
	cfg := loadFixture(t, "[groups.my_group]\nbenchmarks = [\"x\"]\n"+
		"[profiles.p]\ncore_share = 60\n[profiles.p.tier1]\nintelligence = 3\ncost = 3\nspeed = 3\n"+
		"[profiles.p.tier2]\nmy_group = 4\n")
	profiles, err := cfg.LoadProfiles(testCategories)
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if profiles["p"].Tier2["my_group"] != 4 {
		t.Fatalf("profiles = %+v", profiles)
	}
}

// reload marshals cfg, decodes the output into a fresh Config, and returns it.
func reload(t *testing.T, cfg *Config) *Config {
	t.Helper()
	out, err := cfg.MarshalTOML()
	if err != nil {
		t.Fatalf("MarshalTOML: %v", err)
	}
	return loadFixture(t, string(out))
}

func TestSetLoadRoundTrip(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		cfg := Default()
		p := ProfileTOML{
			CoreShare: 65,
			Tier1:     map[string]int{"intelligence": 4, "cost": 3, "speed": 2},
			Tier2:     map[string]int{"software_engineering": 5},
		}
		if err := cfg.SetProfile("deep_work", p, testCategories); err != nil {
			t.Fatalf("SetProfile: %v", err)
		}
		got, err := reload(t, cfg).LoadProfiles(testCategories)
		if err != nil {
			t.Fatalf("LoadProfiles: %v", err)
		}
		if !reflect.DeepEqual(got, ProfilesTOML{"deep_work": p}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("harness", func(t *testing.T) {
		cfg := Default()
		h := HarnessTOML{Name: "Claude Code", Command: "claude --model {model_id}", Providers: []string{"claude"}, Builtin: true}
		if err := cfg.SetHarness("claude_code", h); err != nil {
			t.Fatalf("SetHarness: %v", err)
		}
		got, err := reload(t, cfg).LoadHarnesses()
		if err != nil {
			t.Fatalf("LoadHarnesses: %v", err)
		}
		if !reflect.DeepEqual(got, HarnessesTOML{"claude_code": h}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("favourites", func(t *testing.T) {
		cfg := Default()
		f := FavouritesTOML{Pins: []string{"claude/claude-opus-5@max", "codex/gpt-5.6@high"}}
		if err := cfg.SetFavourites(f); err != nil {
			t.Fatalf("SetFavourites: %v", err)
		}
		got, err := reload(t, cfg).LoadFavourites()
		if err != nil {
			t.Fatalf("LoadFavourites: %v", err)
		}
		if !reflect.DeepEqual(got, f) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("routes disabled", func(t *testing.T) {
		cfg := Default()
		r := RoutesDisabledTOML{"claude": {"claude-opus-5@low"}}
		if err := cfg.SetRoutesDisabled(r); err != nil {
			t.Fatalf("SetRoutesDisabled: %v", err)
		}
		got, err := reload(t, cfg).LoadRoutesDisabled()
		if err != nil {
			t.Fatalf("LoadRoutesDisabled: %v", err)
		}
		if !reflect.DeepEqual(got, r) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("group", func(t *testing.T) {
		cfg := Default()
		g := GroupTOML{Benchmarks: []string{"SWE-Bench Verified", "Terminal-Bench"}}
		if err := cfg.SetGroup("my_group", g); err != nil {
			t.Fatalf("SetGroup: %v", err)
		}
		got, err := reload(t, cfg).LoadGroups()
		if err != nil {
			t.Fatalf("LoadGroups: %v", err)
		}
		if !reflect.DeepEqual(got, GroupsTOML{"my_group": g}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("gui writes all keys", func(t *testing.T) {
		cfg := Default()
		g := DefaultGUIConfig()
		g.Layout = "list"
		g.Holds = 10
		if err := cfg.SetGUI(g); err != nil {
			t.Fatalf("SetGUI: %v", err)
		}
		out, err := cfg.MarshalTOML()
		if err != nil {
			t.Fatalf("MarshalTOML: %v", err)
		}
		var doc map[string]any
		if _, err := toml.Decode(string(out), &doc); err != nil {
			t.Fatalf("toml.Decode: %v", err)
		}
		gui, ok := doc["gui"].(map[string]any)
		if !ok {
			t.Fatalf("no [gui] in %q", out)
		}
		// Bump deliberately when a setting is added: catalog_repo +
		// use_local_aa took this from 14 to 16.
		if len(gui) != 16 {
			t.Fatalf("[gui] has %d keys, want 16: %v", len(gui), gui)
		}
		got, err := loadFixture(t, string(out)).LoadGUI()
		if err != nil {
			t.Fatalf("LoadGUI: %v", err)
		}
		if got != g {
			t.Fatalf("got %+v, want %+v", got, g)
		}
	})
}

func TestDeleteIdempotent(t *testing.T) {
	cfg := Default()

	// Deletes on a config with no sections at all must not panic or error.
	cfg.DeleteProfile("absent")
	cfg.DeleteHarness("absent")
	cfg.DeleteGroup("absent")

	if err := cfg.SetProfile("p", ProfileTOML{CoreShare: 60, Tier1: map[string]int{"intelligence": 3, "cost": 3, "speed": 3}}, testCategories); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if err := cfg.SetHarness("h", HarnessTOML{Name: "x", Command: "y"}); err != nil {
		t.Fatalf("SetHarness: %v", err)
	}
	if err := cfg.SetGroup("g", GroupTOML{Benchmarks: []string{"x"}}); err != nil {
		t.Fatalf("SetGroup: %v", err)
	}

	for range 2 {
		cfg.DeleteProfile("p")
		cfg.DeleteHarness("h")
		cfg.DeleteGroup("g")
	}

	profiles, err := cfg.LoadProfiles(testCategories)
	if err != nil || len(profiles) != 0 {
		t.Fatalf("LoadProfiles = %v, %v", profiles, err)
	}
	harnesses, err := cfg.LoadHarnesses()
	if err != nil || len(harnesses) != 0 {
		t.Fatalf("LoadHarnesses = %v, %v", harnesses, err)
	}
	groups, err := cfg.LoadGroups()
	if err != nil || len(groups) != 0 {
		t.Fatalf("LoadGroups = %v, %v", groups, err)
	}
}

const goldenFixture = `[usage]
enabled = true

[scoring]
normalizer = "minmax-linear"
aggregator = "weighted-arithmetic-mean"

[future]
mystery = "keep me"

[future.nested]
answer = 42

[providers.claude]
enabled = true
priority = 1

[profiles.deep_work]
core_share = 60

[profiles.deep_work.tier1]
intelligence = 4
cost = 3
speed = 3

[profiles.deep_work.tier2]
software_engineering = 5

[harnesses.claude_code]
name = "Claude Code"
command = "claude --model {model_id} --reasoning {reasoning}"
providers = ["claude"]
builtin = true

[favourites]
pins = ["claude/claude-opus-5@max"]

[routes.disabled]
claude = ["claude-opus-5@low"]

[groups.my_group]
benchmarks = ["SWE-Bench Verified"]

[gui]
layout = "list"
holds = 10
`

func TestMarshalGoldenRoundTrip(t *testing.T) {
	cfg := loadFixture(t, goldenFixture)
	first, err := cfg.MarshalTOML()
	if err != nil {
		t.Fatalf("MarshalTOML: %v", err)
	}
	second, err := loadFixture(t, string(first)).MarshalTOML()
	if err != nil {
		t.Fatalf("MarshalTOML (reload): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("marshal not byte-stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	var got map[string]any
	if _, err := toml.Decode(string(first), &got); err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	// Zero keys lost: every leaf of the original fixture survives.
	for _, tc := range []struct {
		key  string
		want any
	}{
		{"usage.enabled", true},
		{"scoring.normalizer", "minmax-linear"},
		{"scoring.aggregator", "weighted-arithmetic-mean"},
		{"future.mystery", "keep me"},
		{"future.nested.answer", int64(42)},
		{"providers.claude.enabled", true},
		{"providers.claude.priority", int64(1)},
		{"profiles.deep_work.core_share", int64(60)},
		{"profiles.deep_work.tier1.intelligence", int64(4)},
		{"profiles.deep_work.tier1.cost", int64(3)},
		{"profiles.deep_work.tier1.speed", int64(3)},
		{"profiles.deep_work.tier2.software_engineering", int64(5)},
		{"harnesses.claude_code.name", "Claude Code"},
		{"harnesses.claude_code.command", "claude --model {model_id} --reasoning {reasoning}"},
		{"harnesses.claude_code.builtin", true},
		{"favourites.pins", []any{"claude/claude-opus-5@max"}},
		{"routes.disabled.claude", []any{"claude-opus-5@low"}},
		{"groups.my_group.benchmarks", []any{"SWE-Bench Verified"}},
		{"gui.layout", "list"},
		{"gui.holds", int64(10)},
	} {
		if value := rawLookup(got, tc.key); !reflect.DeepEqual(value, tc.want) {
			t.Fatalf("%s = %#v, want %#v\nout:\n%s", tc.key, value, tc.want, first)
		}
	}

	var reloaded map[string]any
	if _, err := toml.Decode(string(second), &reloaded); err != nil {
		t.Fatalf("toml.Decode (second): %v", err)
	}
	if !reflect.DeepEqual(got, reloaded) {
		t.Fatalf("documents differ semantically")
	}
}
