package score

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scoreCSV(header string, rows ...string) []byte {
	return []byte(header + "\n" + strings.Join(rows, "\n") + "\n")
}

const scoresMinimalHeader = "model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score"

func TestParseScoresGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "scores_golden.csv"))
	if err != nil {
		t.Fatalf("read scores_golden.csv: %v", err)
	}
	rows, err := ParseScoresCSV(data)
	if err != nil {
		t.Fatalf("ParseScoresCSV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	opus := rows[0]
	if opus.Model != "Claude Opus 5" || opus.Reasoning != "max" {
		t.Errorf("identity = %q / %q", opus.Model, opus.Reasoning)
	}
	if got := opus.Tier1["intelligence_index_score"]; got.String() != "100" {
		t.Errorf("intelligence_index_score = %s, want 100", got)
	}
	if got := opus.Tier1["cost_per_intelligence_index_task_usd_score"]; got.String() != "0" {
		t.Errorf("cost score = %s, want 0", got)
	}
	if got := opus.Tier1["median_end_to_end_response_time_seconds_score"]; got.String() != "12" {
		t.Errorf("median score = %s, want 12", got)
	}
	if got := opus.Categories["software_engineering"]; got.String() != "50" {
		t.Errorf("software_engineering = %s, want 50", got)
	}
	if _, ok := opus.Categories["finance"]; ok {
		t.Error("Categories[finance] present, want absent")
	}
	if got := opus.Benchmarks["SWE-Bench Pro"]; got.String() != "100" {
		t.Errorf("SWE-Bench Pro = %s, want 100", got)
	}
	if got := opus.Benchmarks["DeepSWE"]; got.String() != "0" {
		t.Errorf("DeepSWE = %s, want 0", got)
	}
	if _, ok := opus.Benchmarks["SWE-Bench Verified"]; ok {
		t.Error("Benchmarks[SWE-Bench Verified] present, want absent (blank cell)")
	}
	if rows[2].Reasoning != "high" {
		t.Errorf("Kimi reasoning = %q, want high", rows[2].Reasoning)
	}
}

func TestParseScoresErrors(t *testing.T) {
	tests := []struct {
		name string
		csv  []byte
		want string
	}{
		{
			name: "missing median column",
			csv:  scoreCSV("model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score", "Claude Opus 5,max,100,0,0,100,100"),
			want: "score CSV is missing required columns: median_end_to_end_response_time_seconds_score",
		},
		{
			name: "extra cell",
			csv:  scoreCSV(scoresMinimalHeader, "Claude Opus 5,max,100,0,0,0,0,0,x"),
			want: "score CSV row 2 has extra cells",
		},
		{
			name: "blank model",
			csv:  scoreCSV(scoresMinimalHeader, ",high,100,0,0,0,0,0"),
			want: "score CSV row 2 has a blank model/reasoning identity",
		},
		{
			name: "duplicate identity",
			csv:  scoreCSV(scoresMinimalHeader, "Claude Opus 5,max,100,0,0,0,0,0", "Claude Opus 5,max,100,0,0,0,0,0"),
			want: "score CSV has duplicate identity: Claude Opus 5 / max",
		},
		{
			name: "out of range cell",
			csv:  scoreCSV(scoresMinimalHeader, "Claude Opus 5,max,101,0,0,0,0,0"),
			want: "score CSV row 2 intelligence_index_score must be between 0 and 100",
		},
		{
			name: "empty input",
			csv:  nil,
			want: "score CSV contains no model rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseScoresCSV(tt.csv)
			if err == nil {
				t.Fatal("ParseScoresCSV: nil error")
			}
			var se *Error
			if !errors.As(err, &se) || se.Code != ErrInvalidScoresCSV {
				t.Fatalf("error = %v, want *Error{ErrInvalidScoresCSV}", err)
			}
			if se.Message != tt.want {
				t.Errorf("message = %q, want %q", se.Message, tt.want)
			}
		})
	}
}

func TestParseScoresProvenanceShape(t *testing.T) {
	valid := "# which-model-scores-provenance raw_sha256=" + strings.Repeat("ab", 32) +
		" normalizer=minmax-linear aggregator=weighted-arithmetic-mean\n" +
		scoresMinimalHeader + "\n" +
		"Claude Opus 5,max,100,0,0,0,0,0\n"
	rows, err := ParseScoresCSV([]byte(valid))
	if err != nil {
		t.Fatalf("ParseScoresCSV with provenance: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows = %d, want 1", len(rows))
	}

	twoLines := "# which-model-scores-provenance raw_sha256=" + strings.Repeat("ab", 32) + "\n" +
		"# another comment\n" +
		scoresMinimalHeader + "\n" +
		"Claude Opus 5,max,100,0,0,0,0,0\n"
	if _, err := ParseScoresCSV([]byte(twoLines)); err == nil {
		t.Error("two leading # lines: nil error")
	}

	malformed := "# which-model-scores-provenance raw_sha256=zz\n" +
		scoresMinimalHeader + "\n" +
		"Claude Opus 5,max,100,0,0,0,0,0\n"
	if _, err := ParseScoresCSV([]byte(malformed)); err == nil {
		t.Error("malformed raw_sha256: nil error")
	}
}
