package band

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestReasonCodeBandGated locks the reason-code constant (CONTRACTS §7).
func TestReasonCodeBandGated(t *testing.T) {
	if got, want := ReasonCodeBandGated, "band_gated"; got != want {
		t.Errorf("ReasonCodeBandGated = %q, want %q", got, want)
	}
}

// TestDirections locks the direction constants (SPEC §2.5).
func TestDirections(t *testing.T) {
	if got, want := DirectionSpread, Direction("spread"); got != want {
		t.Errorf("DirectionSpread = %q, want %q", got, want)
	}
	if got, want := DirectionDrain, Direction("drain"); got != want {
		t.Errorf("DirectionDrain = %q, want %q", got, want)
	}
}

// TestPressureZeroValue locks the zero value of Pressure.
func TestPressureZeroValue(t *testing.T) {
	var p Pressure
	if p.Known {
		t.Error("Pressure{}.Known = true, want false")
	}
	if !p.Percent.Equal(decimal.Zero) {
		t.Errorf("Pressure{}.Percent = %s, want decimal.Zero", p.Percent)
	}
}

// TestResultZeroValue locks the zero value of Result.
func TestResultZeroValue(t *testing.T) {
	var r Result
	if r.Name != "" {
		t.Errorf("Result{}.Name = %q, want empty", r.Name)
	}
	if r.Gated {
		t.Error("Result{}.Gated = true, want false")
	}
	if r.Warning != "" {
		t.Errorf("Result{}.Warning = %q, want empty", r.Warning)
	}
	if !r.Weight.Equal(decimal.Zero) {
		t.Errorf("Result{}.Weight = %s, want decimal.Zero", r.Weight)
	}
}

// TestBandSpecFieldsReadBack locks BandSpec field round-tripping.
func TestBandSpecFieldsReadBack(t *testing.T) {
	spec := BandSpec{
		Name:             "low",
		UpperUsedPercent: decimal.NewFromFloat(25),
		Weight:           decimal.NewFromFloat(1),
	}
	if spec.Name != "low" {
		t.Errorf("spec.Name = %q, want %q", spec.Name, "low")
	}
	if !spec.UpperUsedPercent.Equal(decimal.NewFromFloat(25)) {
		t.Errorf("spec.UpperUsedPercent = %s, want 25", spec.UpperUsedPercent)
	}
	if !spec.Weight.Equal(decimal.NewFromFloat(1)) {
		t.Errorf("spec.Weight = %s, want 1", spec.Weight)
	}
}

// TestConfigZeroValue locks the zero value of Config.
func TestConfigZeroValue(t *testing.T) {
	var cfg Config
	if len(cfg.Tiers) != 0 {
		t.Errorf("Config{}.Tiers has %d entries, want 0", len(cfg.Tiers))
	}
}
