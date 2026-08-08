package pick

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/shopspring/decimal"
)

func mustDec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", s, err)
	}
	return d
}

func TestDegradedCandidatesEmpty(t *testing.T) {
	got := DegradedCandidates(nil)
	if got == nil || len(got) != 0 {
		t.Errorf("DegradedCandidates(nil) = %v, want empty non-nil slice", got)
	}
}

func TestDegradedCandidatesStripsBands(t *testing.T) {
	in := []Candidate{
		{Route: routing.Route{Provider: "claude", ModelID: "a"}, Band: "low", BandWeight: mustDec(t, "0.5")},
		{Route: routing.Route{Provider: "codex", ModelID: "b"}, Band: "high", BandWeight: mustDec(t, "0.5")},
	}
	got := DegradedCandidates(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for i, c := range got {
		if c.Band != "" {
			t.Errorf("candidate %d Band = %q, want empty", i, c.Band)
		}
		if !c.BandWeight.Equal(decimal.NewFromFloat(1.0)) {
			t.Errorf("candidate %d BandWeight = %v, want 1.0", i, c.BandWeight)
		}
	}
}

func TestDegradedCandidatesFinalScoreEqualsModelScore(t *testing.T) {
	in := []Candidate{{
		Route:      routing.Route{Provider: "claude", ModelID: "m"},
		ModelScore: mustDec(t, "88.4"),
		BandWeight: mustDec(t, "0.85"),
		FinalScore: mustDec(t, "75.14"), // 88.4 × 0.85
	}}
	got := DegradedCandidates(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].FinalScore.Equal(in[0].ModelScore) {
		t.Errorf("degraded FinalScore = %v, want Equal(ModelScore %v)", got[0].FinalScore, in[0].ModelScore)
	}
	if !got[0].FinalScore.Equal(decimal.NewFromFloat(88.4)) {
		t.Errorf("degraded FinalScore = %v, want 88.4", got[0].FinalScore)
	}
}

func TestDegradedCandidatesPreservesRouteFields(t *testing.T) {
	in := []Candidate{{
		Route: routing.Route{
			Provider:  "claude",
			ModelID:   "claude-opus-4-8-20260115",
			Model:     "Claude Opus 4.8",
			Reasoning: "max",
		},
		ModelScore:     mustDec(t, "92"),
		ProviderWeight: decimal.NewFromFloat(1.0),
	}}
	got := DegradedCandidates(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	c := got[0]
	if c.Route.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", c.Route.Provider, "claude")
	}
	if c.Route.ModelID != "claude-opus-4-8-20260115" {
		t.Errorf("ModelID = %q, want %q", c.Route.ModelID, "claude-opus-4-8-20260115")
	}
	if c.Route.Reasoning != "max" {
		t.Errorf("Reasoning = %q, want %q", c.Route.Reasoning, "max")
	}
	if !c.ProviderWeight.Equal(decimal.NewFromFloat(1.0)) {
		t.Errorf("ProviderWeight = %v, want 1.0", c.ProviderWeight)
	}
	if !c.ModelScore.Equal(mustDec(t, "92")) {
		t.Errorf("ModelScore = %v, want 92", c.ModelScore)
	}
}

func TestDegradedCandidatesNoInputMutation(t *testing.T) {
	in := []Candidate{
		{Route: routing.Route{Provider: "claude", ModelID: "a"}, Band: "low", BandWeight: mustDec(t, "0.5")},
		{Route: routing.Route{Provider: "codex", ModelID: "b"}, Band: "high", BandWeight: mustDec(t, "0.9")},
	}
	wantBands := []string{"low", "high"}
	wantWeights := []string{"0.5", "0.9"}
	_ = DegradedCandidates(in)
	for i := range in {
		if in[i].Band != wantBands[i] {
			t.Errorf("input candidate %d Band mutated: %q, want %q", i, in[i].Band, wantBands[i])
		}
		if !in[i].BandWeight.Equal(mustDec(t, wantWeights[i])) {
			t.Errorf("input candidate %d BandWeight mutated: %v, want %s", i, in[i].BandWeight, wantWeights[i])
		}
	}
}
