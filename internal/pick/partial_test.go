package pick

import (
	"slices"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/shopspring/decimal"
)

func TestPartialTier1UsesAvailableWeightsAndWarns(t *testing.T) {
	profile := Profiles["balanced_implementation"]
	row := scoreRow("GPT-6 Astra", "max", "90", "30", "", nil)
	ms := ScoreModelWithOptions(row, profile, RankOptions{AllowIncomplete: true})
	want := decimal.NewFromInt(90).Mul(profile.Tier1Weights["intelligence"]).Add(decimal.NewFromInt(30).Mul(profile.Tier1Weights["cost"])).Div(profile.Tier1Weights["intelligence"].Add(profile.Tier1Weights["cost"]))
	if len(ms.ExcludedReasons) != 0 || !ms.Total.Equal(want) {
		t.Fatalf("partial score = %+v, want %s with no exclusion", ms, want)
	}
	if !slices.Contains(ms.Warnings, "Missing benchmark data: speed. Ranked using available scores.") {
		t.Errorf("warnings = %v", ms.Warnings)
	}
	if _, exists := row.Tier1[Tier1ScoreColumn[AxisSpeed]]; exists {
		t.Error("scoring fabricated a speed value")
	}
}

func TestPartialTier1ZeroIsEvidenceAndEmptyRowsStayExcluded(t *testing.T) {
	ms := ScoreModelWithOptions(scoreRow("Measured zero", "high", "0", "", "", nil), Profiles["balanced_implementation"], RankOptions{AllowIncomplete: true})
	if len(ms.ExcludedReasons) != 0 || !ms.Total.IsZero() {
		t.Fatalf("measured zero must remain eligible: %+v", ms)
	}
	ms = ScoreModelWithOptions(scoreRow("No core evidence", "high", "", "", "", nil), Profiles["balanced_implementation"], RankOptions{AllowIncomplete: true})
	if !slices.Equal(ms.ExcludedReasons, []string{"missing_tier1:intelligence,cost,speed"}) {
		t.Errorf("empty row exclusion = %v", ms.ExcludedReasons)
	}
}

func TestRankPartialTiePrefersPublishedAxisOverMissing(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Alpha missing intelligence", "high", "", "0", "0", nil),
		scoreRow("Beta measured intelligence", "high", "0", "", "", nil),
	}
	result, err := RankWithOptions(rows, Profiles["balanced_implementation"], nil, CategoryNames, RankOptions{AllowIncomplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateCount != 2 || result.Recommendation.Model != "Beta measured intelligence" {
		t.Fatalf("partial tie result = %+v", result)
	}
}
