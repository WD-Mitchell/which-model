package pick

import (
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// TestProfileFromJSONFlat covers the flat JSON form (TASKS.md F10-T3
// test 1): tier1_share/tier2_share/tier1_weights/tier2_weights.
func TestProfileFromJSONFlat(t *testing.T) {
	data := []byte(`{"tier1_share":70,"tier2_share":30,"tier1_weights":{"intelligence":5,"cost":1,"speed":1},"tier2_weights":{"research":5}}`)
	p, err := ProfileFromJSON(data)
	if err != nil {
		t.Fatalf("ProfileFromJSON: %v", err)
	}
	if p.Name != "custom" {
		t.Errorf("Name = %q, want %q", p.Name, "custom")
	}
	if !p.Tier1Share.Equal(decimal.NewFromInt(70)) {
		t.Errorf("Tier1Share = %v, want 70", p.Tier1Share)
	}
	if !p.Tier2Share.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Tier2Share = %v, want 30", p.Tier2Share)
	}
	wantTier1 := map[string]decimal.Decimal{
		"intelligence": decimal.NewFromInt(5),
		"cost":         decimal.NewFromInt(1),
		"speed":        decimal.NewFromInt(1),
	}
	if len(p.Tier1Weights) != len(wantTier1) {
		t.Errorf("Tier1Weights = %v, want %v", p.Tier1Weights, wantTier1)
	}
	for k, v := range wantTier1 {
		if got, ok := p.Tier1Weights[k]; !ok || !got.Equal(v) {
			t.Errorf("Tier1Weights[%q] = %v, want %v", k, got, v)
		}
	}
	wantTier2 := map[string]decimal.Decimal{"research": decimal.NewFromInt(5)}
	if len(p.Tier2Weights) != len(wantTier2) {
		t.Errorf("Tier2Weights = %v, want %v", p.Tier2Weights, wantTier2)
	}
	for k, v := range wantTier2 {
		if got, ok := p.Tier2Weights[k]; !ok || !got.Equal(v) {
			t.Errorf("Tier2Weights[%q] = %v, want %v", k, got, v)
		}
	}
	if err := ValidateProfile(p); err != nil {
		t.Errorf("ValidateProfile: %v", err)
	}
}

// TestProfileFromJSONNested covers the nested axis form (TASKS.md F10-T3
// test 2): tier1:{share, axis...} / tier2:{share, category...}.
func TestProfileFromJSONNested(t *testing.T) {
	data := []byte(`{"tier1":{"share":70,"intelligence":5,"cost":1,"speed":1},"tier2":{"share":30,"research":5}}`)
	p, err := ProfileFromJSON(data)
	if err != nil {
		t.Fatalf("ProfileFromJSON: %v", err)
	}
	if !p.Tier1Share.Equal(decimal.NewFromInt(70)) {
		t.Errorf("Tier1Share = %v, want 70", p.Tier1Share)
	}
	if !p.Tier2Share.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Tier2Share = %v, want 30", p.Tier2Share)
	}
	if got, ok := p.Tier2Weights["research"]; !ok || !got.Equal(decimal.NewFromInt(5)) {
		t.Errorf("Tier2Weights[research] = %v, want 5", got)
	}
}

// TestProfileFromJSONNestedWeights covers the nested weights-object form
// (TASKS.md F10-T3 test 3): tier1:{share, weights:{...}} / tier2:{share,
// weights:{...}} — identical result to the nested axis form.
func TestProfileFromJSONNestedWeights(t *testing.T) {
	data := []byte(`{"tier1":{"share":70,"weights":{"intelligence":5,"cost":1,"speed":1}},"tier2":{"share":30,"weights":{"research":5}}}`)
	p, err := ProfileFromJSON(data)
	if err != nil {
		t.Fatalf("ProfileFromJSON: %v", err)
	}
	if !p.Tier1Share.Equal(decimal.NewFromInt(70)) {
		t.Errorf("Tier1Share = %v, want 70", p.Tier1Share)
	}
	if !p.Tier2Share.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Tier2Share = %v, want 30", p.Tier2Share)
	}
	if got, ok := p.Tier2Weights["research"]; !ok || !got.Equal(decimal.NewFromInt(5)) {
		t.Errorf("Tier2Weights[research] = %v, want 5", got)
	}
	if got, ok := p.Tier1Weights["speed"]; !ok || !got.Equal(decimal.NewFromInt(1)) {
		t.Errorf("Tier1Weights[speed] = %v, want 1", got)
	}
}

// TestProfileFromJSONDefaults covers share defaults 100/0 (TASKS.md F10-T3
// test 4): a flat weights-only document.
func TestProfileFromJSONDefaults(t *testing.T) {
	data := []byte(`{"tier1_weights":{"intelligence":5,"cost":1,"speed":1}}`)
	p, err := ProfileFromJSON(data)
	if err != nil {
		t.Fatalf("ProfileFromJSON: %v", err)
	}
	if !p.Tier1Share.Equal(decimal.NewFromInt(100)) {
		t.Errorf("Tier1Share = %v, want 100", p.Tier1Share)
	}
	if !p.Tier2Share.Equal(decimal.NewFromInt(0)) {
		t.Errorf("Tier2Share = %v, want 0", p.Tier2Share)
	}
}

// TestProfileFromJSONErrors pins the verbatim parse/validation error
// messages (TASKS.md F10-T3 test 5).
func TestProfileFromJSONErrors(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "not json",
			input:   `not json`,
			wantErr: "weights JSON is invalid:",
		},
		{
			name:    "not an object",
			input:   `[1]`,
			wantErr: "weights JSON must be an object",
		},
		{
			name:    "weights not objects",
			input:   `{"tier1_weights":5}`,
			wantErr: "weights JSON tier1/tier2 weights must be objects",
		},
		{
			name:    "non-numeric weight",
			input:   `{"tier1_weights":{"intelligence":"x","cost":1,"speed":1}}`,
			wantErr: "tier 1 weight intelligence must be numeric",
		},
		{
			name:    "missing tier1 key",
			input:   `{"tier1_weights":{"intelligence":5,"cost":1}}`,
			wantErr: "tier 1 weights must include intelligence, cost, and speed (missing speed)",
		},
		{
			name:    "weight out of range",
			input:   `{"tier1_weights":{"intelligence":5,"cost":1,"speed":9}}`,
			wantErr: "tier 1 weight speed must be between 0 and 5",
		},
		{
			name:    "share non-finite",
			input:   `{"tier1_weights":{"intelligence":5,"cost":1,"speed":1},"tier1_share":"NaN"}`,
			wantErr: "tier1 share must be finite",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProfileFromJSON([]byte(tc.input))
			if err == nil {
				t.Fatalf("ProfileFromJSON(%s): got nil error, want %q", tc.input, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Errorf("ProfileFromJSON(%s): error %q, want prefix %q", tc.input, err.Error(), tc.wantErr)
			}
			var re *RankingError
			if !errors.As(err, &re) {
				t.Errorf("ProfileFromJSON(%s): error type %T, want *RankingError", tc.input, err)
			}
		})
	}
}
