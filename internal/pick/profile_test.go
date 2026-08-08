package pick

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

// TestBuiltinProfilesAreValid asserts the package ships exactly the 11
// built-in profiles, every one passes ValidateProfile, every tier-1 weight
// key set is exactly {intelligence, cost, speed}, and every tier-2 key is a
// member of CategoryNames (annex-b §5.1; TASKS.md F10-T1 test 1).
func TestBuiltinProfilesAreValid(t *testing.T) {
	if len(Profiles) != 11 {
		t.Fatalf("len(Profiles) = %d, want 11", len(Profiles))
	}
	tier1Keys := map[string]bool{"intelligence": true, "cost": true, "speed": true}
	categoryNames := map[string]bool{}
	for _, name := range CategoryNames {
		categoryNames[name] = true
	}
	for name, p := range Profiles {
		if err := ValidateProfile(p); err != nil {
			t.Errorf("profile %q: ValidateProfile error: %v", name, err)
		}
		if len(p.Tier1Weights) != len(tier1Keys) {
			t.Errorf("profile %q: tier1 weight keys = %d, want exactly 3", name, len(p.Tier1Weights))
		}
		for key := range p.Tier1Weights {
			if !tier1Keys[key] {
				t.Errorf("profile %q: unknown tier1 weight key %q", name, key)
			}
		}
		for key := range p.Tier2Weights {
			if !categoryNames[key] {
				t.Errorf("profile %q: tier2 key %q is not a CategoryNames member", name, key)
			}
		}
	}
}

// TestPlanningProfileWeights pins Profiles["planning"] (TASKS.md F10-T1
// test 2): tier2 {planning_capability: 5}, shares 60/40.
func TestPlanningProfileWeights(t *testing.T) {
	p := Profiles["planning"]
	want := map[string]decimal.Decimal{"planning_capability": decimal.NewFromInt(5)}
	if len(p.Tier2Weights) != len(want) {
		t.Fatalf("planning tier2 weights = %v, want %v", p.Tier2Weights, want)
	}
	for key, w := range want {
		if got, ok := p.Tier2Weights[key]; !ok || !got.Equal(w) {
			t.Errorf("planning tier2 weight %q = %v, want %v", key, got, w)
		}
	}
	if !p.Tier1Share.Equal(decimal.NewFromInt(60)) {
		t.Errorf("planning Tier1Share = %v, want 60", p.Tier1Share)
	}
	if !p.Tier2Share.Equal(decimal.NewFromInt(40)) {
		t.Errorf("planning Tier2Share = %v, want 40", p.Tier2Share)
	}
}

// TestOrchestrationProfileWeights pins Profiles["orchestration"]
// (TASKS.md F10-T1 test 3): shares 60/40, tier1 {intelligence:5, cost:5,
// speed:4}, tier2 {planning_capability:5, instruction_following:5}, and
// tier2 keys disjoint from {reasoning, knowledge, agentic_tools, research}.
func TestOrchestrationProfileWeights(t *testing.T) {
	p := Profiles["orchestration"]
	if !p.Tier1Share.Equal(decimal.NewFromInt(60)) || !p.Tier2Share.Equal(decimal.NewFromInt(40)) {
		t.Errorf("orchestration shares = %v/%v, want 60/40", p.Tier1Share, p.Tier2Share)
	}
	wantTier1 := map[string]decimal.Decimal{
		"intelligence": decimal.NewFromInt(5),
		"cost":         decimal.NewFromInt(5),
		"speed":        decimal.NewFromInt(4),
	}
	if len(p.Tier1Weights) != len(wantTier1) {
		t.Errorf("orchestration tier1 weights = %v, want %v", p.Tier1Weights, wantTier1)
	}
	for key, w := range wantTier1 {
		if got, ok := p.Tier1Weights[key]; !ok || !got.Equal(w) {
			t.Errorf("orchestration tier1 weight %q = %v, want %v", key, got, w)
		}
	}
	wantTier2 := map[string]decimal.Decimal{
		"planning_capability":   decimal.NewFromInt(5),
		"instruction_following": decimal.NewFromInt(5),
	}
	if len(p.Tier2Weights) != len(wantTier2) {
		t.Errorf("orchestration tier2 weights = %v, want %v", p.Tier2Weights, wantTier2)
	}
	for key, w := range wantTier2 {
		if got, ok := p.Tier2Weights[key]; !ok || !got.Equal(w) {
			t.Errorf("orchestration tier2 weight %q = %v, want %v", key, got, w)
		}
	}
	doubleCounted := []string{"reasoning", "knowledge", "agentic_tools", "research"}
	for _, key := range doubleCounted {
		if _, ok := p.Tier2Weights[key]; ok {
			t.Errorf("orchestration tier2 must not weight %q (double counting)", key)
		}
	}
}

// TestValidateProfileRules pins the six verbatim validation rules of
// rank_models.py:80-103 (annex-b §5.2; TASKS.md F10-T2 test table). Each
// case mutates a deep copy of Profiles["balanced_implementation"].
func TestValidateProfileRules(t *testing.T) {
	base := func() catalog.Profile {
		p := Profiles["balanced_implementation"]
		tier1 := make(map[string]decimal.Decimal, len(p.Tier1Weights))
		for k, v := range p.Tier1Weights {
			tier1[k] = v
		}
		tier2 := make(map[string]decimal.Decimal, len(p.Tier2Weights))
		for k, v := range p.Tier2Weights {
			tier2[k] = v
		}
		p.Tier1Weights = tier1
		p.Tier2Weights = tier2
		return p
	}
	cases := []struct {
		name    string
		mutate  func(*catalog.Profile)
		wantErr string
	}{
		{
			name:    "tier1 share zero",
			mutate:  func(p *catalog.Profile) { p.Tier1Share = decimal.NewFromInt(0) },
			wantErr: "tier 1 share must be positive and tier 2 share cannot be negative",
		},
		{
			name:    "tier2 share negative",
			mutate:  func(p *catalog.Profile) { p.Tier2Share = decimal.NewFromInt(-1) },
			wantErr: "tier 1 share must be positive and tier 2 share cannot be negative",
		},
		{
			name: "shares sum to 99",
			mutate: func(p *catalog.Profile) {
				p.Tier1Share = decimal.NewFromInt(70)
				p.Tier2Share = decimal.NewFromInt(29)
			},
			wantErr: "tier 1 and tier 2 shares must sum to 100",
		},
		{
			name:    "tier1 missing speed",
			mutate:  func(p *catalog.Profile) { delete(p.Tier1Weights, "speed") },
			wantErr: "tier 1 weights must include intelligence, cost, and speed (missing speed)",
		},
		{
			name:    "tier1 unknown foo",
			mutate:  func(p *catalog.Profile) { p.Tier1Weights["foo"] = decimal.NewFromInt(5) },
			wantErr: "tier 1 weights must include intelligence, cost, and speed (unknown foo)",
		},
		{
			name: "tier1 missing speed and unknown foo",
			mutate: func(p *catalog.Profile) {
				delete(p.Tier1Weights, "speed")
				p.Tier1Weights["foo"] = decimal.NewFromInt(5)
			},
			wantErr: "tier 1 weights must include intelligence, cost, and speed (missing speed; unknown foo)",
		},
		{
			name:    "tier1 weight zero",
			mutate:  func(p *catalog.Profile) { p.Tier1Weights["intelligence"] = decimal.NewFromInt(0) },
			wantErr: "tier 1 weight intelligence must be greater than 0 and at most 5",
		},
		{
			name:    "tier1 weight six",
			mutate:  func(p *catalog.Profile) { p.Tier1Weights["intelligence"] = decimal.NewFromInt(6) },
			wantErr: "tier 1 weight intelligence must be greater than 0 and at most 5",
		},
		{
			name:    "tier2 unknown notacategory",
			mutate:  func(p *catalog.Profile) { p.Tier2Weights["notacategory"] = decimal.NewFromInt(1) },
			wantErr: "unknown tier 2 categories: notacategory",
		},
		{
			name: "tier2 unknown notacategory and bogus",
			mutate: func(p *catalog.Profile) {
				p.Tier2Weights["notacategory"] = decimal.NewFromInt(1)
				p.Tier2Weights["bogus"] = decimal.NewFromInt(1)
			},
			wantErr: "unknown tier 2 categories: bogus, notacategory",
		},
		{
			name:    "tier2 weight zero",
			mutate:  func(p *catalog.Profile) { p.Tier2Weights["software_engineering"] = decimal.NewFromInt(0) },
			wantErr: "tier 2 weight software_engineering must be greater than 0 and at most 5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base()
			tc.mutate(&p)
			err := ValidateProfile(p)
			if err == nil {
				t.Fatalf("ValidateProfile: got nil error, want %q", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("ValidateProfile: got %q, want %q", err.Error(), tc.wantErr)
			}
			var re *RankingError
			if !errors.As(err, &re) {
				t.Errorf("ValidateProfile: error type %T, want *RankingError", err)
			}
		})
	}
	if err := ValidateProfile(Profiles["balanced_implementation"]); err != nil {
		t.Errorf("untouched balanced_implementation: ValidateProfile error: %v", err)
	}
}
