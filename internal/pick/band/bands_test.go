package band

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/shopspring/decimal"
)

// TestDefaultConfigDirection locks the default direction (SPEC §2.9).
func TestDefaultConfigDirection(t *testing.T) {
	if got, want := DefaultConfig().Direction, DirectionSpread; got != want {
		t.Errorf("DefaultConfig().Direction = %q, want %q", got, want)
	}
}

// TestDefaultConfigScalars locks the default gate and unknown weight.
func TestDefaultConfigScalars(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.GateAboveUsedPercent.Equal(decimal.NewFromFloat(98)) {
		t.Errorf("DefaultConfig().GateAboveUsedPercent = %s, want 98", cfg.GateAboveUsedPercent)
	}
	if !cfg.UnknownPressureWeight.Equal(decimal.NewFromFloat(0.90)) {
		t.Errorf("DefaultConfig().UnknownPressureWeight = %s, want 0.90", cfg.UnknownPressureWeight)
	}
}

// TestDefaultConfigTiers locks the default ladder: low/25/1.00, standard/50/0.85,
// elevated/75/0.60, critical/100/0.25, ascending (plan §5.2).
func TestDefaultConfigTiers(t *testing.T) {
	cfg := DefaultConfig()
	want := []BandSpec{
		{Name: "low", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1.00)},
		{Name: "standard", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(0.85)},
		{Name: "elevated", UpperUsedPercent: decimal.NewFromFloat(75), Weight: decimal.NewFromFloat(0.60)},
		{Name: "critical", UpperUsedPercent: decimal.NewFromFloat(100), Weight: decimal.NewFromFloat(0.25)},
	}
	if len(cfg.Tiers) != len(want) {
		t.Fatalf("DefaultConfig().Tiers has %d entries, want %d", len(cfg.Tiers), len(want))
	}
	for i := range want {
		got := cfg.Tiers[i]
		w := want[i]
		if got.Name != w.Name {
			t.Errorf("tier %d name = %q, want %q", i, got.Name, w.Name)
		}
		if !got.UpperUsedPercent.Equal(w.UpperUsedPercent) {
			t.Errorf("tier %d upper = %s, want %s", i, got.UpperUsedPercent, w.UpperUsedPercent)
		}
		if !got.Weight.Equal(w.Weight) {
			t.Errorf("tier %d weight = %s, want %s", i, got.Weight, w.Weight)
		}
		if i > 0 && got.UpperUsedPercent.LessThanOrEqual(cfg.Tiers[i-1].UpperUsedPercent) {
			t.Errorf("tier %d upper %s not strictly ascending after %s", i, got.UpperUsedPercent, cfg.Tiers[i-1].UpperUsedPercent)
		}
	}
}

// TestDefaultConfigValid asserts the defaults pass validation.
func TestDefaultConfigValid(t *testing.T) {
	if err := ValidateBands(DefaultConfig()); err != nil {
		t.Errorf("ValidateBands(DefaultConfig()) = %v, want nil", err)
	}
}

// validConfig is a validated baseline config to mutate per-case.
func validConfig() Config {
	return Config{
		Direction:             DirectionSpread,
		GateAboveUsedPercent:  decimal.NewFromFloat(98),
		UnknownPressureWeight: decimal.NewFromFloat(0.90),
		Tiers: []BandSpec{
			{Name: "low", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1.00)},
			{Name: "standard", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(0.85)},
			{Name: "elevated", UpperUsedPercent: decimal.NewFromFloat(75), Weight: decimal.NewFromFloat(0.60)},
			{Name: "critical", UpperUsedPercent: decimal.NewFromFloat(100), Weight: decimal.NewFromFloat(0.25)},
		},
	}
}

// TestValidateBands locks the fixed-order validation checks and their exact
// error strings (SPEC §2.8, CONTRACTS §5).
func TestValidateBands(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "valid config",
			mutate: func(*Config) {
			},
			wantErr: "",
		},
		{
			name: "invalid direction",
			mutate: func(c *Config) {
				c.Direction = "sideways"
			},
			wantErr: `bands: direction must be "spread" or "drain"`,
		},
		{
			name: "negative gate",
			mutate: func(c *Config) {
				c.GateAboveUsedPercent = decimal.NewFromFloat(-1)
			},
			wantErr: "bands: gate_above_used_percent must not be negative",
		},
		{
			name: "zero unknown pressure weight",
			mutate: func(c *Config) {
				c.UnknownPressureWeight = decimal.Zero
			},
			wantErr: "bands: unknown_pressure_weight must be positive",
		},
		{
			name: "empty tiers",
			mutate: func(c *Config) {
				c.Tiers = nil
			},
			wantErr: "bands: tiers must not be empty",
		},
		{
			name: "non ascending bounds",
			mutate: func(c *Config) {
				c.Tiers = []BandSpec{
					{Name: "a", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1)},
					{Name: "b", UpperUsedPercent: decimal.NewFromFloat(20), Weight: decimal.NewFromFloat(1)},
				}
			},
			wantErr: "bands: tier 2 upper_used_percent 20 must be greater than the previous bound 25",
		},
		{
			name: "duplicate tier names",
			mutate: func(c *Config) {
				c.Tiers = []BandSpec{
					{Name: "a", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1)},
					{Name: "a", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(1)},
				}
			},
			wantErr: "bands: tier names must be unique",
		},
		{
			name: "reserved tier name",
			mutate: func(c *Config) {
				c.Tiers = []BandSpec{
					{Name: "unknown", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1)},
				}
			},
			wantErr: `bands: tier name "unknown" is reserved`,
		},
		{
			name: "non positive tier weight",
			mutate: func(c *Config) {
				c.Tiers = []BandSpec{
					{Name: "low", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.Zero},
				}
			},
			wantErr: `bands: tier "low" weight must be positive`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := ValidateBands(cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateBands() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateBands() = nil, want error %q", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("ValidateBands() error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestFromTOML locks the decode recipe: DefaultConfig baseline, pointer
// scalars override, nil Tiers keeps the default ladder (SPEC §2.9, TASKS T5).
func TestFromTOML(t *testing.T) {
	ptr := func(v decimal.Decimal) *decimal.Decimal { return &v }
	drn := func(v string) *string { return &v }

	tests := []struct {
		name    string
		toml    TOMLConfig
		want    Config
		wantErr string
	}{
		{
			name: "empty config is defaults",
			toml: TOMLConfig{},
			want: DefaultConfig(),
		},
		{
			name: "direction only",
			toml: TOMLConfig{Direction: drn("drain")},
			want: func() Config {
				c := DefaultConfig()
				c.Direction = DirectionDrain
				return c
			}(),
		},
		{
			name: "gate only",
			toml: TOMLConfig{GateAboveUsedPercent: ptr(decimal.NewFromFloat(95.5))},
			want: func() Config {
				c := DefaultConfig()
				c.GateAboveUsedPercent = decimal.NewFromFloat(95.5)
				return c
			}(),
		},
		{
			name: "unknown weight only",
			toml: TOMLConfig{UnknownPressureWeight: ptr(decimal.NewFromFloat(0.5))},
			want: func() Config {
				c := DefaultConfig()
				c.UnknownPressureWeight = decimal.NewFromFloat(0.5)
				return c
			}(),
		},
		{
			name: "nil tiers keeps default ladder",
			toml: TOMLConfig{Tiers: nil},
			want: DefaultConfig(),
		},
		{
			name:    "empty tiers is an error",
			toml:    TOMLConfig{Tiers: []TOMLTier{}},
			wantErr: "bands: tiers must not be empty",
		},
		{
			name: "custom tiers preserved in order",
			toml: TOMLConfig{Tiers: []TOMLTier{
				{Name: "a", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1)},
				{Name: "b", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(0.5)},
			}},
			want: func() Config {
				c := DefaultConfig()
				c.Tiers = []BandSpec{
					{Name: "a", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.NewFromFloat(1)},
					{Name: "b", UpperUsedPercent: decimal.NewFromFloat(50), Weight: decimal.NewFromFloat(0.5)},
				}
				return c
			}(),
		},
		{
			name:    "invalid direction",
			toml:    TOMLConfig{Direction: drn("sideways")},
			wantErr: `bands: direction must be "spread" or "drain"`,
		},
		{
			name: "zero tier weight",
			toml: TOMLConfig{Tiers: []TOMLTier{
				{Name: "low", UpperUsedPercent: decimal.NewFromFloat(25), Weight: decimal.Zero},
			}},
			wantErr: `bands: tier "low" weight must be positive`,
		},
		{
			name:    "negative gate",
			toml:    TOMLConfig{GateAboveUsedPercent: ptr(decimal.NewFromFloat(-1))},
			wantErr: "bands: gate_above_used_percent must not be negative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromTOML(tc.toml)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("FromTOML() = %+v, nil error; want %q", got, tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Errorf("FromTOML() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromTOML() error = %v, want nil", err)
			}
			if got.Direction != tc.want.Direction {
				t.Errorf("FromTOML().Direction = %q, want %q", got.Direction, tc.want.Direction)
			}
			if !got.GateAboveUsedPercent.Equal(tc.want.GateAboveUsedPercent) {
				t.Errorf("FromTOML().GateAboveUsedPercent = %s, want %s", got.GateAboveUsedPercent, tc.want.GateAboveUsedPercent)
			}
			if !got.UnknownPressureWeight.Equal(tc.want.UnknownPressureWeight) {
				t.Errorf("FromTOML().UnknownPressureWeight = %s, want %s", got.UnknownPressureWeight, tc.want.UnknownPressureWeight)
			}
			if len(got.Tiers) != len(tc.want.Tiers) {
				t.Fatalf("FromTOML().Tiers has %d entries, want %d", len(got.Tiers), len(tc.want.Tiers))
			}
			for i := range tc.want.Tiers {
				g, w := got.Tiers[i], tc.want.Tiers[i]
				if g.Name != w.Name {
					t.Errorf("tier %d name = %q, want %q", i, g.Name, w.Name)
				}
				if !g.UpperUsedPercent.Equal(w.UpperUsedPercent) {
					t.Errorf("tier %d upper = %s, want %s", i, g.UpperUsedPercent, w.UpperUsedPercent)
				}
				if !g.Weight.Equal(w.Weight) {
					t.Errorf("tier %d weight = %s, want %s", i, g.Weight, w.Weight)
				}
			}
		})
	}
}

// TestFromTOMLBurntSushiDecode proves the critical decode path end to end:
// BurntSushi/toml decodes TOML floats straight into decimal.Decimal via
// shopspring's TextUnmarshaler (SPEC §2.10, F01 SPEC B12).
func TestFromTOMLBurntSushiDecode(t *testing.T) {
	raw := `
direction = "drain"
gate_above_used_percent = 95.5
unknown_pressure_weight = 0.5

[[tier]]
name = "low"
upper_used_percent = 25.0
weight = 1.0

[[tier]]
name = "high"
upper_used_percent = 100.0
weight = 0.1
`
	var decoded TOMLConfig
	if _, err := toml.Decode(raw, &decoded); err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	cfg, err := FromTOML(decoded)
	if err != nil {
		t.Fatalf("FromTOML: %v", err)
	}
	if cfg.Direction != DirectionDrain {
		t.Errorf("direction = %q, want %q", cfg.Direction, DirectionDrain)
	}
	if !cfg.GateAboveUsedPercent.Equal(decimal.NewFromFloat(95.5)) {
		t.Errorf("gate = %s, want 95.5", cfg.GateAboveUsedPercent)
	}
	if !cfg.UnknownPressureWeight.Equal(decimal.NewFromFloat(0.5)) {
		t.Errorf("unknown weight = %s, want 0.5", cfg.UnknownPressureWeight)
	}
	if len(cfg.Tiers) != 2 {
		t.Fatalf("tiers = %d, want 2", len(cfg.Tiers))
	}
	if cfg.Tiers[0].Name != "low" || !cfg.Tiers[0].UpperUsedPercent.Equal(decimal.NewFromFloat(25)) {
		t.Errorf("tier 0 = %+v, want low/25", cfg.Tiers[0])
	}
	if cfg.Tiers[1].Name != "high" || !cfg.Tiers[1].Weight.Equal(decimal.NewFromFloat(0.1)) {
		t.Errorf("tier 1 = %+v, want high/0.1", cfg.Tiers[1])
	}
}

// TestEvaluateBand locks the ladder evaluation: gating before unknown before
// band match; inclusive upper bounds; clamping above the last bound; drain
// reverses weights (SPEC §2.4-§2.7, TASKS T6).
func TestEvaluateBand(t *testing.T) {
	// Default ladder with the gate pushed to 150 so the band-match cases
	// below are not gated (TASKS T6 instruction).
	cfg := DefaultConfig()
	cfg.GateAboveUsedPercent = decimal.NewFromFloat(150)

	tests := []struct {
		name string
		p    Pressure
		cfg  Config
		want Result
	}{
		{
			name: "inclusive lower edge maps to low",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(25)},
			cfg:  cfg,
			want: Result{Name: "low", Weight: decimal.NewFromFloat(1.00), Gated: false, Warning: ""},
		},
		{
			name: "below first bound lands in tier 0",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(0)},
			cfg:  cfg,
			want: Result{Name: "low", Weight: decimal.NewFromFloat(1.00)},
		},
		{
			name: "just below 25",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(24.99)},
			cfg:  cfg,
			want: Result{Name: "low", Weight: decimal.NewFromFloat(1.00)},
		},
		{
			name: "inclusive edge maps to standard",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(50)},
			cfg:  cfg,
			want: Result{Name: "standard", Weight: decimal.NewFromFloat(0.85)},
		},
		{
			name: "just below 75",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(74.99)},
			cfg:  cfg,
			want: Result{Name: "elevated", Weight: decimal.NewFromFloat(0.60)},
		},
		{
			name: "inclusive edge maps to elevated",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(75)},
			cfg:  cfg,
			want: Result{Name: "elevated", Weight: decimal.NewFromFloat(0.60)},
		},
		{
			name: "exactly 100 is critical",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(100)},
			cfg:  cfg,
			want: Result{Name: "critical", Weight: decimal.NewFromFloat(0.25)},
		},
		{
			name: "above last bound clamps to critical",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(120)},
			cfg:  cfg,
			want: Result{Name: "critical", Weight: decimal.NewFromFloat(0.25)},
		},
		{
			name: "unknown pressure",
			p:    Pressure{Known: false},
			cfg:  cfg,
			want: Result{
				Name:    "unknown",
				Weight:  decimal.NewFromFloat(0.90),
				Gated:   false,
				Warning: "pressure unknown for route; using unknown_pressure_weight",
			},
		},
		{
			name: "gated at or above gate",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(200)},
			cfg:  cfg,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true, Warning: ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateBand(tc.p, tc.cfg)
			if got.Name != tc.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tc.want.Name)
			}
			if !got.Weight.Equal(tc.want.Weight) {
				t.Errorf("Weight = %s, want %s", got.Weight, tc.want.Weight)
			}
			if got.Gated != tc.want.Gated {
				t.Errorf("Gated = %v, want %v", got.Gated, tc.want.Gated)
			}
			if got.Warning != tc.want.Warning {
				t.Errorf("Warning = %q, want %q", got.Warning, tc.want.Warning)
			}
		})
	}
}

// TestGatingDrainMatrix locks the gating/drain boundary behaviour together
// (SPEC §2.5-§2.7; TASKS T7): gating is inclusive at-or-above
// gate_above_used_percent; unknown pressure overrides gating; drain reverses
// the weight assignment across tiers but never the tier naming, and never
// applies to a gated candidate.
func TestGatingDrainMatrix(t *testing.T) {
	spreadGate98 := DefaultConfig() // gate 98, spread (the defaults)
	// Drain rows 7-11 need a gate above 100 so p{100} reaches critical
	// ungated (TASKS T7 case 10); row 12 uses the real gate 98.
	drainGate150 := DefaultConfig()
	drainGate150.Direction = DirectionDrain
	drainGate150.GateAboveUsedPercent = decimal.NewFromFloat(150)
	drainGate98 := DefaultConfig()
	drainGate98.Direction = DirectionDrain // drain + real gate: gating wins (TASKS T7 case 12)
	spreadGate0 := DefaultConfig()
	spreadGate0.GateAboveUsedPercent = decimal.Zero
	spreadGate100 := DefaultConfig()
	spreadGate100.GateAboveUsedPercent = decimal.NewFromFloat(100)

	tests := []struct {
		name string
		p    Pressure
		cfg  Config
		want Result
	}{
		{
			name: "just below gate is not gated",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(97.99)},
			cfg:  spreadGate98,
			// First bound >= 97.99 is 100 -> critical (inclusive rule,
			// DEFERRED D8; the table's literal {elevated} value is the
			// pre-D8 off-by-one arithmetic).
			want: Result{Name: "critical", Weight: decimal.NewFromFloat(0.25)},
		},
		{
			name: "at gate is gated (inclusive at-or-above)",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(98)},
			cfg:  spreadGate98,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true},
		},
		{
			name: "above gate is gated",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(99.5)},
			cfg:  spreadGate98,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true},
		},
		{
			name: "gate zero gates zero (gate itself inclusive)",
			p:    Pressure{Known: true, Percent: decimal.Zero},
			cfg:  spreadGate0,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true},
		},
		{
			name: "critical bound equals gate is gated",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(100)},
			cfg:  spreadGate100,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true},
		},
		{
			name: "unknown pressure is never gated",
			p:    Pressure{Known: false},
			cfg:  spreadGate98,
			want: Result{
				Name:    "unknown",
				Weight:  decimal.NewFromFloat(0.90),
				Gated:   false,
				Warning: "pressure unknown for route; using unknown_pressure_weight",
			},
		},
		{
			name: "drain tier 1 takes the reversed weight",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(30)},
			cfg:  drainGate150,
			// First bound >= 30 is 50 -> tier 1; drain weight =
			// weight[len-1-1] = weight[2] (DEFERRED D8).
			want: Result{Name: "standard", Weight: decimal.NewFromFloat(0.60)},
		},
		{
			name: "drain tier 2 takes the reversed weight",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(60)},
			cfg:  drainGate150,
			// First bound >= 60 is 75 -> tier 2; drain weight =
			// weight[len-1-2] = weight[1] (DEFERRED D8).
			want: Result{Name: "elevated", Weight: decimal.NewFromFloat(0.85)},
		},
		{
			name: "drain tier 3 takes the reversed weight",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(90)},
			cfg:  drainGate150,
			// First bound >= 90 is 100 -> tier 3; drain weight =
			// weight[len-1-3] = weight[0] (DEFERRED D8).
			want: Result{Name: "critical", Weight: decimal.NewFromFloat(1.00)},
		},
		{
			name: "drain top tier takes the first tier weight",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(100)},
			cfg:  drainGate150,
			want: Result{Name: "critical", Weight: decimal.NewFromFloat(1.00)},
		},
		{
			name: "drain does not change unknown handling",
			p:    Pressure{Known: false},
			cfg:  drainGate150,
			want: Result{
				Name:    "unknown",
				Weight:  decimal.NewFromFloat(0.90),
				Gated:   false,
				Warning: "pressure unknown for route; using unknown_pressure_weight",
			},
		},
		{
			name: "drain never applies to a gated candidate",
			p:    Pressure{Known: true, Percent: decimal.NewFromFloat(99)},
			cfg:  drainGate98,
			want: Result{Name: "", Weight: decimal.Zero, Gated: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateBand(tc.p, tc.cfg)
			if got.Name != tc.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tc.want.Name)
			}
			if !got.Weight.Equal(tc.want.Weight) {
				t.Errorf("Weight = %s, want %s", got.Weight, tc.want.Weight)
			}
			if got.Gated != tc.want.Gated {
				t.Errorf("Gated = %v, want %v", got.Gated, tc.want.Gated)
			}
			if got.Warning != tc.want.Warning {
				t.Errorf("Warning = %q, want %q", got.Warning, tc.want.Warning)
			}
		})
	}
}
