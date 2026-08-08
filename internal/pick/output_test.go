package pick

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

func rankJSON(t *testing.T, rows []catalog.ScoreRow, profile catalog.Profile, available []Identity) []byte {
	t.Helper()
	res, err := Rank(rows, profile, available)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return out
}

// TestResultJSONSchema (TASKS.md F10-T7 test 1): the Result JSON shape —
// decimals are unquoted json.Number (annex-b §5.8 numbers, not Python's
// rounded _json_safe floats), recommendation carries the shared 63/27/90,
// category_scores is exactly the three requested categories, alternatives
// holds the runner-up, excluded serializes as [] (not null), and the
// availability flags read false for an unfiltered run.
func TestResultJSONSchema(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Model A", "medium", "90", "90", "90", map[string]string{
			"software_engineering":  "90",
			"instruction_following": "90",
			"agentic_tools":         "90",
		}),
		scoreRow("Model B", "medium", "80", "80", "80", map[string]string{
			"software_engineering":  "80",
			"instruction_following": "80",
			"agentic_tools":         "80",
		}),
	}
	out := rankJSON(t, rows, Profiles["balanced_implementation"], nil)

	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc["profile"] != "balanced_implementation" {
		t.Errorf("profile = %v, want balanced_implementation", doc["profile"])
	}
	rec, ok := doc["recommendation"].(map[string]any)
	if !ok {
		t.Fatalf("recommendation not an object: %v", doc["recommendation"])
	}
	if rec["model"] != "Model A" || rec["reasoning"] != "medium" {
		t.Errorf("recommendation model/reasoning = %v/%v, want Model A/medium", rec["model"], rec["reasoning"])
	}
	for field, want := range map[string]json.Number{
		"total_score":        json.Number("90"),
		"tier1_contribution": json.Number("63"),
		"tier2_contribution": json.Number("27"),
		"tier2_score":        json.Number("90"),
	} {
		got, isNum := rec[field].(json.Number)
		if !isNum {
			t.Errorf("recommendation.%s = %T (%v), want json.Number", field, rec[field], rec[field])
			continue
		}
		if got != want {
			t.Errorf("recommendation.%s = %v, want %v", field, got, want)
		}
	}
	cats, ok := rec["category_scores"].(map[string]any)
	if !ok {
		t.Fatalf("category_scores not an object: %v", rec["category_scores"])
	}
	if len(cats) != 3 {
		t.Errorf("category_scores = %v, want exactly 3 categories", cats)
	}
	for _, name := range []string{"software_engineering", "instruction_following", "agentic_tools"} {
		if _, ok := cats[name]; !ok {
			t.Errorf("category_scores missing %q: %v", name, cats)
		}
	}
	alts, ok := doc["alternatives"].([]any)
	if !ok || len(alts) != 1 {
		t.Fatalf("alternatives = %v, want exactly one entry", doc["alternatives"])
	}
	alt, ok := alts[0].(map[string]any)
	if !ok || alt["model"] != "Model B" || alt["reasoning"] != "medium" {
		t.Errorf("alternatives[0] = %v, want Model B/medium", alts[0])
	}
	excluded, ok := doc["excluded"].([]any)
	if !ok || len(excluded) != 0 {
		t.Errorf("excluded = %v, want empty array", doc["excluded"])
	}
	if doc["candidate_count"] != json.Number("2") {
		t.Errorf("candidate_count = %v, want 2", doc["candidate_count"])
	}
	if doc["availability_filter_applied"] != false {
		t.Errorf("availability_filter_applied = %v, want false", doc["availability_filter_applied"])
	}
}

// TestResultJSONPrecision (TASKS.md F10-T7 test 2): the 655/7 tier-2 value
// round-trips at full 34-digit precision — a raw number token in the
// marshaled output, no float rounding.
func TestResultJSONPrecision(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Model A", "medium", "87", "92", "88", map[string]string{
			"software_engineering": "91",
			"agentic_tools":        "100",
		}),
	}
	out := rankJSON(t, rows, Profiles["balanced_implementation"], nil)
	wantTier2 := `93.5714285714285714285714285714285714`
	if !bytes.Contains(out, []byte(wantTier2)) {
		t.Errorf("raw JSON %s does not contain %s", out, wantTier2)
	}
}

// TestResultJSONNoInternalKeys (TASKS.md F10-T7 test 3): no _tie_* sort key
// and no ExcludedReasons/excluded_reasons field ever reach the serialized
// JSON — a ModelScore with non-empty ExcludedReasons still serializes
// without it.
func TestResultJSONNoInternalKeys(t *testing.T) {
	rows := []catalog.ScoreRow{
		scoreRow("Model A", "medium", "90", "90", "90", map[string]string{
			"software_engineering":  "90",
			"instruction_following": "90",
			"agentic_tools":         "90",
		}),
		scoreRow("Model B", "medium", "80", "80", "", nil),
	}
	out := rankJSON(t, rows, Profiles["balanced_implementation"], nil)
	// The internal sort keys are the rank_models.py `_tie_*` tuple members
	// (e.g. _tie_total_score); "missing_tier1" legitimately contains "_tie"
	// as a substring of "_tier", so match the sort-key form `_tie_`.
	for _, hidden := range []string{"_tie_", "ExcludedReasons", "excluded_reasons"} {
		if bytes.Contains(out, []byte(hidden)) {
			t.Errorf("raw JSON %s must not contain %q", out, hidden)
		}
	}
	// Direct check: ExcludedReasons is json:"-" on ModelScore itself.
	excludedScore := ModelScore{Model: "M", Reasoning: "r", ExcludedReasons: []string{"missing_tier1:speed"}}
	solo, err := json.Marshal(excludedScore)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if bytes.Contains(solo, []byte("excluded_reasons")) || bytes.Contains(solo, []byte("ExcludedReasons")) {
		t.Errorf("ModelScore JSON %s must not contain ExcludedReasons", solo)
	}
}

// TestTier2NullWhenAbsent (TASKS.md F10-T7 test 4): a row with no category
// data serializes tier2_score as JSON null.
func TestTier2NullWhenAbsent(t *testing.T) {
	rows := []catalog.ScoreRow{scoreRow("Model A", "medium", "80", "80", "80", nil)}
	out := rankJSON(t, rows, Profiles["balanced_implementation"], nil)
	if !bytes.Contains(out, []byte(`"tier2_score":null`)) {
		t.Errorf("raw JSON %s does not contain \"tier2_score\":null", out)
	}
}
