package score

import "testing"

func TestDerivePreservesPartialBenchmarks(t *testing.T) {
	raw := rawCSV(goldenHeader()+",benchmark:GPQA Diamond",
		"Baseline,high,20,10,1,10,40,20,30",
		"GPT-6 Astra,max,60,,3,,80,60,90",
		"Benchmark only,high,,,,,,,60",
		"Undisclosed,high,,,,,,,")
	header, rows := deriveCSV(t, raw)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want all 3 measured models (omit identity-only row)", len(rows))
	}
	for column, want := range map[string]string{
		"intelligence_index": "60", "intelligence_index_score": "100",
		"cost_per_intelligence_index_task_usd_score":    "0",
		"artificial_analysis_coding_index_score":        "100",
		"median_end_to_end_response_time_seconds":       "",
		"median_end_to_end_response_time_seconds_score": "",
		"benchmark:GPQA Diamond_score":                  "100",
	} {
		if got := cellAt(t, header, rows, 1, column); got != want {
			t.Errorf("Astra %s = %q, want %q", column, got, want)
		}
	}
	if got := cellAt(t, header, rows, 2, "benchmark:GPQA Diamond_score"); got != "50" {
		t.Errorf("benchmark-only score = %q, want 50", got)
	}
}

func TestDerivePartialOnlyAndConstantRanges(t *testing.T) {
	raw := rawCSV(goldenHeader(), "Alpha,high,60,,,,,", "Beta,high,60,,,,,")
	header, rows := deriveCSV(t, raw)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if got := cellAt(t, header, rows, 0, "intelligence_index"); got != "60" {
		t.Errorf("absolute intelligence = %q, want 60", got)
	}
	if got := cellAt(t, header, rows, 0, "intelligence_index_score"); got != "" {
		t.Errorf("constant relative intelligence = %q, want blank", got)
	}
}
