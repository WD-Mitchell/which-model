package pick

import (
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

func identitySet(ids []Identity) map[Identity]bool {
	set := make(map[Identity]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func assertIdentities(t *testing.T, got []Identity, want ...Identity) {
	t.Helper()
	gotSet := identitySet(got)
	if len(gotSet) != len(want) {
		t.Fatalf("identities = %v, want %v", got, want)
	}
	for _, id := range want {
		if !gotSet[id] {
			t.Errorf("identity %+v missing from %v", id, got)
		}
	}
}

// TestParseAvailabilityJSON (TASKS.md F10-T6 test 1): string, object, and
// pair elements; duplicates collapse; strings are trimmed.
func TestParseAvailabilityJSON(t *testing.T) {
	data := []byte(`["Model A|low", {"model":"Model A","reasoning":"high"}, ["Model B","high"], "Model A|low"]`)
	got, err := ParseAvailability(data)
	if err != nil {
		t.Fatalf("ParseAvailability: %v", err)
	}
	assertIdentities(t, got,
		Identity{Model: "Model A", Reasoning: "low"},
		Identity{Model: "Model A", Reasoning: "high"},
		Identity{Model: "Model B", Reasoning: "high"},
	)

	got, err = ParseAvailability([]byte(`[" Model A :: low "]`))
	if err != nil {
		t.Fatalf("ParseAvailability (spaces): %v", err)
	}
	assertIdentities(t, got, Identity{Model: "Model A", Reasoning: "low"})
}

// TestParseAvailabilityPlainText (TASKS.md F10-T6 test 2): # comments and
// blank lines skipped; the case/space-insensitive header line
// (model,reasoning | model|reasoning) is skipped when it is line 1.
func TestParseAvailabilityPlainText(t *testing.T) {
	// Header skipping applies to the physical line 1 only (line_number == 1
	// in rank_models.py counts every line), so the header comes first.
	text := "model,reasoning\n# comment\n\nModel A|low\nModel B::high\nModel C,medium\nModel D/high\n"
	got, err := ParseAvailability([]byte(text))
	if err != nil {
		t.Fatalf("ParseAvailability: %v", err)
	}
	assertIdentities(t, got,
		Identity{Model: "Model A", Reasoning: "low"},
		Identity{Model: "Model B", Reasoning: "high"},
		Identity{Model: "Model C", Reasoning: "medium"},
		Identity{Model: "Model D", Reasoning: "high"},
	)

	got, err = ParseAvailability([]byte("MODEL | REASONING\nModel A|low\n"))
	if err != nil {
		t.Fatalf("ParseAvailability (MODEL | REASONING header): %v", err)
	}
	assertIdentities(t, got, Identity{Model: "Model A", Reasoning: "low"})
}

// TestParseAvailabilitySeparatorPriority (TASKS.md F10-T6 test 3):
// separators tried in priority order "|", "::", ",", "/" with a
// last-occurrence split.
func TestParseAvailabilitySeparatorPriority(t *testing.T) {
	got, err := ParseAvailability([]byte("Model A|low:extra"))
	if err != nil {
		t.Fatalf("ParseAvailability: %v", err)
	}
	assertIdentities(t, got, Identity{Model: "Model A", Reasoning: "low:extra"})

	got, err = ParseAvailability([]byte("a,b::c"))
	if err != nil {
		t.Fatalf("ParseAvailability: %v", err)
	}
	assertIdentities(t, got, Identity{Model: "a,b", Reasoning: "c"})

	got, err = ParseAvailability([]byte("a|b::c"))
	if err != nil {
		t.Fatalf("ParseAvailability: %v", err)
	}
	assertIdentities(t, got, Identity{Model: "a", Reasoning: "b::c"})
}

// TestParseAvailabilityErrors (TASKS.md F10-T6 tests 4/5): empty input is
// (nil, nil); malformed identities and JSON entries error verbatim.
func TestParseAvailabilityErrors(t *testing.T) {
	got, err := ParseAvailability(nil)
	if got != nil || err != nil {
		t.Errorf("ParseAvailability(nil) = %v, %v; want nil, nil", got, err)
	}
	got, err = ParseAvailability([]byte(""))
	if got != nil || err != nil {
		t.Errorf("ParseAvailability(\"\") = %v, %v; want nil, nil", got, err)
	}
	got, err = ParseAvailability([]byte("   "))
	if got != nil || err != nil {
		t.Errorf("ParseAvailability(\"   \") = %v, %v; want nil, nil", got, err)
	}

	wantIdentityErr := `availability identity "Model A" must use model|reasoning, model::reasoning, model,reasoning, or model/reasoning`
	_, err = ParseAvailability([]byte("Model A"))
	if err == nil || err.Error() != wantIdentityErr {
		t.Errorf("ParseAvailability(\"Model A\") error = %v, want %q", err, wantIdentityErr)
	}
	var re *RankingError
	if !errors.As(err, &re) {
		t.Errorf("ParseAvailability(\"Model A\") error type %T, want *RankingError", err)
	}
	_, err = ParseAvailability([]byte("Model A|"))
	if err == nil || err.Error() != `availability identity "Model A|" must use model|reasoning, model::reasoning, model,reasoning, or model/reasoning` {
		t.Errorf("ParseAvailability(\"Model A|\") error = %v, want %q", err, wantIdentityErr)
	}

	_, err = ParseAvailability([]byte(`[{"model":1}]`))
	if err == nil || !strings.HasPrefix(err.Error(), "invalid availability entry:") {
		t.Errorf("ParseAvailability([{\"model\":1}]) error = %v, want prefix %q", err, "invalid availability entry:")
	}

	_, err = ParseAvailability([]byte(`{}`))
	if err == nil || err.Error() != "availability JSON must be a list" {
		t.Errorf("ParseAvailability({}) error = %v, want %q", err, "availability JSON must be a list")
	}

	_, err = ParseAvailability([]byte("# only a comment\n# another\n"))
	if err == nil || err.Error() != "availability list contains no identities" {
		t.Errorf("ParseAvailability(comments only) error = %v, want %q", err, "availability list contains no identities")
	}
}

// TestRankAvailabilityFilter (TASKS.md F10-T6 test 6): exact-tuple
// membership only — with available = {(Model A, low)}, Model A/high (which
// would rank first without the filter) is excluded with not_live_available.
func TestRankAvailabilityFilter(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Model A", "high", "80", "80", "80", nil),
		scoreRow("Model A", "low", "80", "80", "80", nil),
		scoreRow("Model B", "high", "80", "80", "80", nil),
	}
	res, err := Rank(rows, Profiles["simple_implementation"], []Identity{{Model: "Model A", Reasoning: "low"}})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if res.Recommendation.Model != "Model A" || res.Recommendation.Reasoning != "low" {
		t.Errorf("Recommendation = %s/%s, want Model A/low", res.Recommendation.Model, res.Recommendation.Reasoning)
	}
	if res.CandidateCount != 1 {
		t.Errorf("CandidateCount = %d, want 1", res.CandidateCount)
	}
	if !res.AvailabilityFilterApplied {
		t.Errorf("AvailabilityFilterApplied = false, want true")
	}
	if len(res.Excluded) != 2 {
		t.Fatalf("Excluded = %+v, want 2 entries", res.Excluded)
	}
	for _, row := range res.Excluded {
		if len(row.Reasons) != 1 || row.Reasons[0] != "not_live_available" {
			t.Errorf("Excluded %s/%s Reasons = %v, want [not_live_available]", row.Model, row.Reasoning, row.Reasons)
		}
	}
}

// TestRankNoFilterNil (TASKS.md F10-T6 test 7): available == nil means no
// filter — no availability exclusions; the casefold tie-break picks
// Model A/high over Model A/low.
func TestRankNoFilterNil(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Model A", "high", "80", "80", "80", nil),
		scoreRow("Model A", "low", "80", "80", "80", nil),
		scoreRow("Model B", "high", "80", "80", "80", nil),
	}
	res, err := Rank(rows, Profiles["simple_implementation"], nil)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if res.Recommendation.Model != "Model A" || res.Recommendation.Reasoning != "high" {
		t.Errorf("Recommendation = %s/%s, want Model A/high", res.Recommendation.Model, res.Recommendation.Reasoning)
	}
	if res.AvailabilityFilterApplied {
		t.Errorf("AvailabilityFilterApplied = true, want false")
	}
	if len(res.Excluded) != 0 {
		t.Errorf("Excluded = %+v, want empty", res.Excluded)
	}
	if res.CandidateCount != 3 {
		t.Errorf("CandidateCount = %d, want 3", res.CandidateCount)
	}
}
