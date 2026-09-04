package pick

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

// scoreRow builds a ScoreRow (TASKS.md F10-T4 helper): Tier1 keyed
// intelligence_index_score/cost_per_intelligence_index_task_usd_score/
// median_end_to_end_response_time_seconds_score (key skipped when the value
// is ""), Categories keyed by category name (skipped when "").
func scoreRow(model, reasoning string, intelligence, cost, speed string, cats map[string]string) catalog.ScoreRow {
	row := catalog.ScoreRow{
		Model:      model,
		Reasoning:  reasoning,
		Tier1:      map[string]decimal.Decimal{},
		Categories: map[string]decimal.Decimal{},
	}
	if intelligence != "" {
		row.Tier1[Tier1ScoreColumn[AxisIntelligence]] = decimal.RequireFromString(intelligence)
	}
	if cost != "" {
		row.Tier1[Tier1ScoreColumn[AxisCost]] = decimal.RequireFromString(cost)
	}
	if speed != "" {
		row.Tier1[Tier1ScoreColumn[AxisSpeed]] = decimal.RequireFromString(speed)
	}
	for name, value := range cats {
		if value != "" {
			row.Categories[name] = decimal.RequireFromString(value)
		}
	}
	return row
}

// TestScoreModelTier1Only (TASKS.md F10-T4 test 1): a complete tier-1 row
// with no category data against balanced_implementation. Every requested
// tier-2 category is blank, so BOTH warnings fire (SPEC D6 — the Python
// ranker appends both unconditionally, rank_models.py:400-408); the missing
// names are joined in CategoryNames order (F10 SPEC D2).
func TestScoreModelTier1Only(t *testing.T) {
	ms := ScoreModel(scoreRow("Model A", "medium", "90", "90", "90", nil), Profiles["balanced_implementation"])
	if !ms.Total.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Total = %v, want 90", ms.Total)
	}
	if !ms.Tier1.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Tier1 = %v, want 90", ms.Tier1)
	}
	if ms.Tier2 != nil {
		t.Errorf("Tier2 = %v, want nil", *ms.Tier2)
	}
	if !ms.Tier1Contribution.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Tier1Contribution = %v, want 90", ms.Tier1Contribution)
	}
	if !ms.Tier2Contribution.Equal(decimal.NewFromInt(0)) {
		t.Errorf("Tier2Contribution = %v, want 0", ms.Tier2Contribution)
	}
	wantWarnings := []string{
		"missing optional category scores: instruction_following, software_engineering, agentic_tools",
		"no optional task-category scores available; Tier 1 score used",
	}
	if len(ms.Warnings) != len(wantWarnings) {
		t.Fatalf("Warnings = %v, want %v", ms.Warnings, wantWarnings)
	}
	for i := range wantWarnings {
		if ms.Warnings[i] != wantWarnings[i] {
			t.Errorf("Warnings[%d] = %q, want %q", i, ms.Warnings[i], wantWarnings[i])
		}
	}
	if len(ms.ExcludedReasons) != 0 {
		t.Errorf("ExcludedReasons = %v, want empty", ms.ExcludedReasons)
	}
}

// TestScoreModelMissingTier1Axis (TASKS.md F10-T4 test 2): missing tier-1
// axis scores exclude the row with the joined axis names in Tier1AxisOrder.
func TestScoreModelMissingTier1Axis(t *testing.T) {
	ms := ScoreModel(scoreRow("Model A", "medium", "90", "90", "", nil), Profiles["balanced_implementation"])
	if len(ms.ExcludedReasons) != 1 || ms.ExcludedReasons[0] != "missing_tier1:speed" {
		t.Errorf("ExcludedReasons = %v, want [missing_tier1:speed]", ms.ExcludedReasons)
	}
	if !ms.Total.IsZero() || !ms.Tier1.IsZero() || ms.Tier2 != nil ||
		!ms.Tier1Contribution.IsZero() || !ms.Tier2Contribution.IsZero() {
		t.Errorf("score fields not zeroed: %+v", ms)
	}
	if len(ms.Warnings) != 0 || len(ms.Categories) != 0 {
		t.Errorf("warnings/categories not zeroed: %+v", ms)
	}

	ms = ScoreModel(scoreRow("Model A", "medium", "", "", "88", nil), Profiles["balanced_implementation"])
	if len(ms.ExcludedReasons) != 1 || ms.ExcludedReasons[0] != "missing_tier1:intelligence,cost" {
		t.Errorf("ExcludedReasons = %v, want [missing_tier1:intelligence,cost]", ms.ExcludedReasons)
	}
}

// TestScoreModelTier2Renormalized (TASKS.md F10-T4 test 3): 87/92/88 with
// software_engineering=91 and agentic_tools=100 under balanced — tier2
// renormalizes over only the categories with data: (5*91+2*100)/7.
func TestScoreModelTier2Renormalized(t *testing.T) {
	cats := map[string]string{"software_engineering": "91", "agentic_tools": "100"}
	ms := ScoreModel(scoreRow("Model A", "medium", "87", "92", "88", cats), Profiles["balanced_implementation"])
	if !ms.Tier1.Equal(decimal.NewFromInt(89)) {
		t.Errorf("Tier1 = %v, want 89", ms.Tier1)
	}
	wantTier2 := "93.5714285714285714285714285714285714"
	if ms.Tier2 == nil {
		t.Fatal("Tier2 = nil, want 655/7")
	}
	if ms.Tier2.String() != wantTier2 {
		t.Errorf("Tier2 = %s, want %s", ms.Tier2.String(), wantTier2)
	}
	tier2 := decimal.NewFromInt(655).Div(decimal.NewFromInt(7))
	wantTotal := decimal.NewFromInt(89).Mul(decimal.NewFromInt(70)).Div(decimal.NewFromInt(100)).
		Add(tier2.Mul(decimal.NewFromInt(30)).Div(decimal.NewFromInt(100)))
	if !ms.Total.Equal(wantTotal) {
		t.Errorf("Total = %v, want %v", ms.Total, wantTotal)
	}
	if !ms.Total.Equal(decimal.RequireFromString("90.3714285714285714285714285714285714")) {
		t.Errorf("Total = %v, want 90.3714285714285714285714285714285714", ms.Total)
	}
	if !ms.Tier1Contribution.Equal(decimal.RequireFromString("62.3")) {
		t.Errorf("Tier1Contribution = %v, want 62.3", ms.Tier1Contribution)
	}
	wantTier2Contribution := tier2.Mul(decimal.NewFromInt(30)).Div(decimal.NewFromInt(100))
	if !ms.Tier2Contribution.Equal(wantTier2Contribution) {
		t.Errorf("Tier2Contribution = %v, want %v", ms.Tier2Contribution, wantTier2Contribution)
	}
	// instruction_following is requested by balanced but blank for this row,
	// so the partial-missing warning fires (rank_models.py:393-394).
	wantWarnings := []string{"missing optional category scores: instruction_following"}
	if len(ms.Warnings) != len(wantWarnings) {
		t.Fatalf("Warnings = %v, want %v", ms.Warnings, wantWarnings)
	}
	for i := range wantWarnings {
		if ms.Warnings[i] != wantWarnings[i] {
			t.Errorf("Warnings[%d] = %q, want %q", i, ms.Warnings[i], wantWarnings[i])
		}
	}
	if len(ms.ExcludedReasons) != 0 {
		t.Errorf("ExcludedReasons = %v, want empty", ms.ExcludedReasons)
	}
}

// TestScoreModelMissingTier2Warns (TASKS.md F10-T4 test 4): all requested
// tier-2 categories blank under simple_action_execution — both warnings
// fire (SPEC D6) and Total stays the RAW un-shared tier-1 score (SPEC §2.6).
// Missing names are joined in CategoryNames order (F10 SPEC D2).
func TestScoreModelMissingTier2Warns(t *testing.T) {
	ms := ScoreModel(scoreRow("Model A", "medium", "90", "90", "90", nil), Profiles["simple_action_execution"])
	if ms.Tier2 != nil {
		t.Errorf("Tier2 = %v, want nil", *ms.Tier2)
	}
	if !ms.Total.Equal(decimal.NewFromInt(90)) || !ms.Tier1.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Total/Tier1 = %v/%v, want raw 90/90 (unscaled)", ms.Total, ms.Tier1)
	}
	if !ms.Tier1Contribution.Equal(decimal.NewFromInt(90)) || !ms.Tier2Contribution.Equal(decimal.NewFromInt(0)) {
		t.Errorf("contributions = %v/%v, want 90/0", ms.Tier1Contribution, ms.Tier2Contribution)
	}
	wantWarnings := []string{
		"missing optional category scores: instruction_following, software_engineering, agentic_tools, evidence_capture",
		"no optional task-category scores available; Tier 1 score used",
	}
	if len(ms.Warnings) != len(wantWarnings) {
		t.Fatalf("Warnings = %v, want %v", ms.Warnings, wantWarnings)
	}
	for i := range wantWarnings {
		if ms.Warnings[i] != wantWarnings[i] {
			t.Errorf("Warnings[%d] = %q, want %q", i, ms.Warnings[i], wantWarnings[i])
		}
	}
}

// TestScoreModelPartialTier2Warning (TASKS.md F10-T4 test 5): one category
// present (software_engineering=90) under balanced — tier2 is the single
// category's score (weight 5/5) and only the partial-missing warning fires.
func TestScoreModelPartialTier2Warning(t *testing.T) {
	cats := map[string]string{"software_engineering": "90"}
	ms := ScoreModel(scoreRow("Model A", "medium", "90", "90", "90", cats), Profiles["balanced_implementation"])
	if ms.Tier2 == nil {
		t.Fatal("Tier2 = nil, want 90")
	}
	if !ms.Tier2.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Tier2 = %v, want 90", *ms.Tier2)
	}
	wantWarnings := []string{"missing optional category scores: instruction_following, agentic_tools"}
	if len(ms.Warnings) != len(wantWarnings) {
		t.Fatalf("Warnings = %v, want %v", ms.Warnings, wantWarnings)
	}
	for i := range wantWarnings {
		if ms.Warnings[i] != wantWarnings[i] {
			t.Errorf("Warnings[%d] = %q, want %q", i, ms.Warnings[i], wantWarnings[i])
		}
	}
	if len(ms.ExcludedReasons) != 0 {
		t.Errorf("ExcludedReasons = %v, want empty", ms.ExcludedReasons)
	}
}

// --- F10-T5: Rank orchestration, tie-break, and exclusion -------------------

// TestRankTieBreakDeterministic (TASKS.md F10-T5 test 1): identical rows
// Beta/high and Alpha/high — the 7-key tuple decides on the casefolded model
// name, so Alpha ranks first.
func TestRankTieBreakDeterministic(t *testing.T) {
	cats := map[string]string{"software_engineering": "90", "instruction_following": "90", "agentic_tools": "90"}
	rows := []catalog.ScoreRow{
		scoreRow("Beta", "high", "90", "90", "90", cats),
		scoreRow("Alpha", "high", "90", "90", "90", cats),
	}
	res, err := Rank(rows, Profiles["balanced_implementation"], nil)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if res.Recommendation.Model != "Alpha" {
		t.Errorf("Recommendation.Model = %q, want Alpha", res.Recommendation.Model)
	}
	if len(res.Alternatives) != 1 || res.Alternatives[0].Model != "Beta" {
		t.Errorf("Alternatives = %+v, want exactly [Beta]", res.Alternatives)
	}
	if res.CandidateCount != 2 {
		t.Errorf("CandidateCount = %d, want 2", res.CandidateCount)
	}
	if res.AvailabilityFilterApplied {
		t.Errorf("AvailabilityFilterApplied = true, want false")
	}
	if len(res.Excluded) != 0 {
		t.Errorf("Excluded = %+v, want empty", res.Excluded)
	}
}

// TestRankTieBreakKeys (TASKS.md F10-T5 test 2): the no-tier-2 asymmetry —
// B (95s, no category data) keeps its raw un-shared total of 95 and outranks
// A (95 intelligence, 85 cost/speed, agentic_tools=100) whose total is
// capped at tier1*70% + 100*30% ≈ 91.83.
func TestRankTieBreakKeys(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("A", "high", "95", "85", "85", map[string]string{"agentic_tools": "100"}),
		scoreRow("B", "high", "95", "95", "95", nil),
	}
	res, err := Rank(rows, Profiles["balanced_implementation"], nil)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if res.Recommendation.Model != "B" {
		t.Errorf("Recommendation.Model = %q, want B (raw un-shared tier-1 total outranks shared total)", res.Recommendation.Model)
	}
	if !res.Recommendation.Total.Equal(decimal.NewFromInt(95)) {
		t.Errorf("Recommendation.Total = %v, want raw 95", res.Recommendation.Total)
	}
}

// TestRankExcludedAndSort (TASKS.md F10-T5 test 3): tier-1 exclusions are
// sorted casefolded by (model, reasoning) ascending, stably; row order
// Zebra-then-Incomplete must sort to Incomplete-then-Zebra.
func TestRankExcludedAndSort(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Good", "high", "80", "80", "80", map[string]string{"software_engineering": "90"}),
		scoreRow("Zebra", "low", "80", "80", "", nil),
		scoreRow("Incomplete", "high", "80", "80", "", nil),
		scoreRow("Alpha", "low", "80", "80", "80", nil),
	}
	res, err := Rank(rows, Profiles["balanced_implementation"], nil)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	rec := res.Recommendation
	if rec.Model != "Good" {
		t.Errorf("Recommendation.Model = %q, want Good", rec.Model)
	}
	if !rec.Total.Equal(decimal.NewFromInt(83)) || !rec.Tier1.Equal(decimal.NewFromInt(80)) {
		t.Errorf("Recommendation Total/Tier1 = %v/%v, want 83/80", rec.Total, rec.Tier1)
	}
	if rec.Tier2 == nil || !rec.Tier2.Equal(decimal.NewFromInt(90)) {
		t.Errorf("Recommendation Tier2 = %v, want 90", rec.Tier2)
	}
	if !rec.Tier1Contribution.Equal(decimal.NewFromInt(56)) || !rec.Tier2Contribution.Equal(decimal.NewFromInt(27)) {
		t.Errorf("Recommendation contributions = %v/%v, want 56/27", rec.Tier1Contribution, rec.Tier2Contribution)
	}
	if len(res.Alternatives) != 1 || res.Alternatives[0].Model != "Alpha" || res.Alternatives[0].Reasoning != "low" {
		t.Errorf("Alternatives = %+v, want exactly [Alpha/low]", res.Alternatives)
	}
	wantExcluded := []ExcludedRow{
		{Model: "Incomplete", Reasoning: "high", Reasons: []string{"missing_tier1:speed"}},
		{Model: "Zebra", Reasoning: "low", Reasons: []string{"missing_tier1:speed"}},
	}
	if len(res.Excluded) != len(wantExcluded) {
		t.Fatalf("Excluded = %+v, want %+v", res.Excluded, wantExcluded)
	}
	for i := range wantExcluded {
		if res.Excluded[i].Model != wantExcluded[i].Model || res.Excluded[i].Reasoning != wantExcluded[i].Reasoning {
			t.Errorf("Excluded[%d] = %+v, want %+v", i, res.Excluded[i], wantExcluded[i])
		}
		if len(res.Excluded[i].Reasons) != 1 || res.Excluded[i].Reasons[0] != "missing_tier1:speed" {
			t.Errorf("Excluded[%d].Reasons = %v, want [missing_tier1:speed]", i, res.Excluded[i].Reasons)
		}
	}
	if res.CandidateCount != 2 {
		t.Errorf("CandidateCount = %d, want 2", res.CandidateCount)
	}
	if res.AvailabilityFilterApplied {
		t.Errorf("AvailabilityFilterApplied = true, want false")
	}
}

// TestRankNoCandidatesDistinctMessages (TASKS.md F10-T5 test 4): zero
// survivors yields the two distinct *NoCandidatesError messages depending
// on whether an availability filter was supplied.
func TestRankNoCandidatesDistinctMessages(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("A", "high", "90", "90", "", nil),
		scoreRow("B", "low", "90", "90", "", nil),
	}
	_, err := Rank(rows, Profiles["balanced_implementation"], nil)
	var nce *NoCandidatesError
	if !errors.As(err, &nce) {
		t.Fatalf("Rank (no filter): error %T, want *NoCandidatesError", err)
	}
	if nce.Message != "no candidates contain all mandatory Tier 1 scores" {
		t.Errorf("no-filter message = %q, want %q", nce.Message, "no candidates contain all mandatory Tier 1 scores")
	}

	_, err = Rank(rows, Profiles["balanced_implementation"], []Identity{{Model: "X", Reasoning: "high"}})
	if !errors.As(err, &nce) {
		t.Fatalf("Rank (with filter): error %T, want *NoCandidatesError", err)
	}
	if nce.Message != "no candidates remain after live model-and-effort availability and Tier 1 filtering" {
		t.Errorf("filter message = %q, want %q", nce.Message, "no candidates remain after live model-and-effort availability and Tier 1 filtering")
	}
}

// TestRankInvalidProfile (TASKS.md F10-T5 test 6): Rank validates the
// profile first and wraps the violation as-is.
func TestRankInvalidProfile(t *testing.T) {
	p := Profiles["balanced_implementation"]
	p.Tier1Share = decimal.NewFromInt(70)
	p.Tier2Share = decimal.NewFromInt(29)
	rows := []catalog.ScoreRow{scoreRow("A", "high", "90", "90", "90", nil)}
	_, err := Rank(rows, p, nil)
	if err == nil || err.Error() != "tier 1 and tier 2 shares must sum to 100" {
		t.Errorf("Rank: error = %v, want %q", err, "tier 1 and tier 2 shares must sum to 100")
	}
}

// A custom category must affect scoring, not merely pass vocabulary validation.
func TestRankWithCategoriesCustomGroupChangesWinner(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Generalist", "high", "90", "90", "90", map[string]string{"custom_group": "0"}),
		scoreRow("Specialist", "high", "80", "80", "80", map[string]string{"custom_group": "100"}),
	}
	profile := catalog.Profile{
		Name: "custom-profile", Tier1Share: decimal.NewFromInt(60), Tier2Share: decimal.NewFromInt(40),
		Tier1Weights: map[string]decimal.Decimal{"intelligence": decimal.NewFromInt(1), "cost": decimal.NewFromInt(1), "speed": decimal.NewFromInt(1)},
	}
	available := []Identity{{Model: "Generalist", Reasoning: "high"}, {Model: "Specialist", Reasoning: "high"}}
	baseline, err := Rank(rows, profile, available)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.CandidateCount != 2 || baseline.Recommendation.Model != "Generalist" || !baseline.Recommendation.Total.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("unexpected tier-1 baseline: %+v", baseline)
	}
	profile.Tier2Weights = map[string]decimal.Decimal{"custom_group": decimal.NewFromInt(5)}
	// The canonical ranker retains its closed vocabulary for existing callers.
	if _, err := Rank(rows, profile, available); err == nil {
		t.Fatal("canonical Rank unexpectedly accepted a custom group")
	}
	ranked, err := RankWithCategories(rows, profile, available, append(append([]string(nil), CategoryNames...), "custom_group"))
	if err != nil {
		t.Fatal(err)
	}
	if ranked.CandidateCount != 2 || len(ranked.Alternatives) != 1 || ranked.Recommendation.Model != "Specialist" || ranked.Alternatives[0].Model != "Generalist" {
		t.Fatalf("custom group did not change ranking: %+v", ranked)
	}
	if !ranked.Recommendation.Total.Equal(decimal.NewFromInt(88)) || !ranked.Alternatives[0].Total.Equal(decimal.NewFromInt(54)) {
		t.Fatalf("custom contribution not applied: winner=%s alternative=%s", ranked.Recommendation.Total, ranked.Alternatives[0].Total)
	}
	if ranked.Recommendation.Tier2 == nil || !ranked.Recommendation.Tier2.Equal(decimal.NewFromInt(100)) || len(ranked.Recommendation.Warnings) != 0 {
		t.Fatalf("custom category evidence missing: %+v", ranked.Recommendation)
	}
}
