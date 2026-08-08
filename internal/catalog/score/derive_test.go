package score

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
)

// rawGoldenSHA256 is the SHA-256 of testdata/raw_golden.csv as written.
// The fixture differs from the TASKS.md text in two cells so that
// Derive(raw) reproduces testdata/scores_golden.csv byte-for-byte (the
// hand-built golden places values one column later than the spec's raw
// text): the GPT-5.6 Sol row's 58.0 sits under benchmark:Finance Agent
// (cell [19], spec had Toolathlon), and Kimi's 53.6/76.0 sit under
// benchmark:MCP Atlas / benchmark:Toolathlon (spec had Program Bench / MCP
// Atlas); the hash is recomputed from the actual file per the TASKS.md
// instruction.
const rawGoldenSHA256 = "4469f49a0fe94cc0f778a9a7e30dc8f7f79327ca5501ea3200cbe44e7d5e0cd3"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return data
}

func TestDeriveGolden(t *testing.T) {
	rawGolden := readFixture(t, "raw_golden.csv")
	benchmarksGolden := readFixture(t, "benchmarks_golden.toml")
	scoresGolden := readFixture(t, "scores_golden.csv")

	sum := sha256.Sum256(rawGolden)
	if got := hex.EncodeToString(sum[:]); got != rawGoldenSHA256 {
		t.Fatalf("raw_golden.csv sha256 = %s, want %s (fixture drifted; recompute)", got, rawGoldenSHA256)
	}

	got, err := Derive(rawGolden, benchmarksGolden, DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	provenance := "# which-model-scores-provenance raw_sha256=" + rawGoldenSHA256 +
		" normalizer=minmax-linear aggregator=weighted-arithmetic-mean"
	want := provenance + "\n" + string(scoresGolden)
	if string(got) != want {
		t.Errorf("Derive output differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestDeriveDeterminism(t *testing.T) {
	rawGolden := readFixture(t, "raw_golden.csv")
	benchmarksGolden := readFixture(t, "benchmarks_golden.toml")
	first, err := Derive(rawGolden, benchmarksGolden, DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	second, err := Derive(rawGolden, benchmarksGolden, DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if string(first) != string(second) {
		t.Error("Derive is not deterministic")
	}
}

// deriveCSV parses Derive's output and returns header + records.
func deriveCSV(t *testing.T, raw []byte) ([]string, [][]string) {
	t.Helper()
	out, err := Derive(raw, readFixture(t, "benchmarks_golden.toml"), DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	body := string(out)
	if idx := strings.IndexByte(body, '\n'); idx >= 0 {
		body = body[idx+1:]
	}
	r := csv.NewReader(strings.NewReader(body))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse Derive output: %v", err)
	}
	return records[0], records[1:]
}

func cellAt(t *testing.T, header []string, records [][]string, row int, column string) string {
	t.Helper()
	for i, name := range header {
		if name == column {
			return records[row][i]
		}
	}
	t.Fatalf("column %q not in header", column)
	return ""
}

func TestDeriveOptionalDegenerateMetrics(t *testing.T) {
	raw := rawCSV(goldenHeader(),
		"Alpha,max,43.0,,0.5,22,50.0,40.0",
		"Beta,low,60.0,,0.9,30,50.0,50.0")
	header, records := deriveCSV(t, raw)
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if got := cellAt(t, header, records, 0, "artificial_analysis_coding_index_score"); got != "" {
		t.Errorf("coding score = %q, want blank (degenerate optional)", got)
	}
	if got := cellAt(t, header, records, 0, "time_per_intelligence_index_task_seconds_score"); got != "" {
		t.Errorf("time score = %q, want blank (no values, optional)", got)
	}
	if got := cellAt(t, header, records, 0, "intelligence_index_score"); got != "0" {
		t.Errorf("intelligence score = %q, want 0", got)
	}
	if got := cellAt(t, header, records, 1, "intelligence_index_score"); got != "100" {
		t.Errorf("intelligence score = %q, want 100", got)
	}
}

func TestDeriveBenchmarkDegenerate(t *testing.T) {
	raw := rawCSV(goldenHeader()+",benchmark:One,benchmark:Two",
		"Alpha,max,43.0,10,0.5,22,50.0,40.0,5,1",
		"Beta,low,60.0,20,0.9,30,50.0,50.0,,2")
	header, records := deriveCSV(t, raw)
	if got := cellAt(t, header, records, 0, "benchmark:One_score"); got != "" {
		t.Errorf("singleton benchmark One score = %q, want blank", got)
	}
	if got := cellAt(t, header, records, 1, "benchmark:Two_score"); got != "100" {
		t.Errorf("benchmark Two score (Alpha) = %q, want 100", got)
	}
	if got := cellAt(t, header, records, 0, "benchmark:Two_score"); got != "0" {
		t.Errorf("benchmark Two score (Beta) = %q, want 0", got)
	}
}

func TestDeriveDropIneligible(t *testing.T) {
	raw := rawCSV(goldenHeader(),
		"Alpha,max,43.0,10,0.5,22,50.0,40.0",
		"Beta,low,60.0,20,,30,50.0,50.0")
	_, records := deriveCSV(t, raw)
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1 (ineligible row dropped)", len(records))
	}
	if records[0][0] != "Alpha" {
		t.Errorf("row model = %q, want Alpha", records[0][0])
	}
}

func TestDeriveInputProvenanceLine(t *testing.T) {
	body := rawCSV(goldenHeader(), "Alpha,max,43.0,10,0.5,22,50.0,40.0")
	rawWithLine := append([]byte("# raw_sha256="+strings.Repeat("ab", 32)+"\n"), body...)
	out, err := Derive(rawWithLine, readFixture(t, "benchmarks_golden.toml"), DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "raw.csv")
	if err := os.WriteFile(tmp, rawWithLine, 0o644); err != nil {
		t.Fatal(err)
	}
	wantHash, err := csvstore.ProvenanceHash(tmp)
	if err != nil {
		t.Fatalf("ProvenanceHash: %v", err)
	}
	first := string(out)
	if idx := strings.IndexByte(first, '\n'); idx >= 0 {
		first = first[:idx]
	}
	if !strings.Contains(first, "raw_sha256="+wantHash) {
		t.Errorf("emitted hash %q does not match csvstore.ProvenanceHash %q", first, wantHash)
	}
}

func TestDeriveErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "zero eligible rows",
			raw: rawCSV(goldenHeader(),
				"Alpha,max,43.0,10,,22,50.0,40.0"),
			want: "input contains no rows with all mandatory Tier 1 metrics: intelligence_index, median_end_to_end_response_time_seconds, cost_per_intelligence_index_task_usd",
		},
		{
			name: "all intelligence blank yields zero eligible",
			raw: rawCSV(goldenHeader(),
				"Alpha,max,,10,0.5,22,50.0,40.0",
				"Beta,low,,20,0.9,30,50.0,50.0"),
			want: "input contains no rows with all mandatory Tier 1 metrics: intelligence_index, median_end_to_end_response_time_seconds, cost_per_intelligence_index_task_usd",
		},
		{
			name: "degenerate mandatory metric",
			raw: rawCSV(goldenHeader(),
				"Alpha,max,43.0,10,0.5,22,50.0,40.0",
				"Beta,low,43.0,20,0.9,30,50.0,50.0"),
			want: "intelligence_index has a degenerate range (43.0)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Derive(tt.raw, readFixture(t, "benchmarks_golden.toml"), DefaultNormalizer(), DefaultAggregator())
			if err == nil {
				t.Fatal("Derive: nil error")
			}
			var se *Error
			if !errors.As(err, &se) {
				t.Fatalf("error = %v, want *Error", err)
			}
			if se.Message != tt.want {
				t.Errorf("message = %q, want %q", se.Message, tt.want)
			}
		})
	}
}
