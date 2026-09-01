package score

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	sdecimal "github.com/shopspring/decimal"
	"github.com/WD-Mitchell/which-model/internal/catalog"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// --- helpers ---------------------------------------------------------------

func rawCSV(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

func goldenHeader() string {
	return "model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index"
}

func mustDecimal(t *testing.T, s string) sdecimal.Decimal {
	t.Helper()
	d, err := wdecimal.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func parseMap(t *testing.T, m map[string]string) map[string]sdecimal.Decimal {
	t.Helper()
	out := make(map[string]sdecimal.Decimal, len(m))
	for k, v := range m {
		out[k] = mustDecimal(t, v)
	}
	return out
}

// scoreRowFromMap hand-builds a catalog.ScoreRow with the canonical keys
// (Tier1 keyed by the 6 _score names, Categories by category name, Benchmarks
// by plain benchmark name).
func scoreRowFromMap(t *testing.T, model, reasoning string, tier1, cats map[string]string) catalog.ScoreRow {
	t.Helper()
	return catalog.ScoreRow{
		Model:      model,
		Reasoning:  reasoning,
		Tier1:      parseMap(t, tier1),
		Categories: parseMap(t, cats),
		Benchmarks: map[string]sdecimal.Decimal{},
	}
}

func goldenBenchmarksTOML() []byte {
	return []byte(`[benchmark_selection]
groups = ["software_engineering", "finance"]
benchmarks = []

[benchmark_groups.software_engineering]
benchmarks = [
  "SWE-Bench Verified",
  "SWE-Bench Pro",
  "SWE-Bench Multilingual",
  "SWE-Bench Multimodal",
  "DeepSWE",
  "Terminal-Bench",
  "AutomationBench",
  "FrontierCode",
  "Program Bench",
  "MCP Atlas",
  "Toolathlon",
]

[benchmark_groups.finance]
benchmarks = [
  "Finance Agent",
  "FinanceAgent",
  "τ3 Banking",
  "GDPval",
  "GDPval-AA",
]
`)
}

// --- T1: MinMaxLinear, directionAdjust, errors, import boundary ------------

func TestMinMaxLinear(t *testing.T) {
	tests := []struct {
		name     string
		raw, min, max string
		want     string
	}{
		{"at max", "63.1", "43.0", "63.1", "100"},
		{"middle rounds half-up", "55.6", "43.0", "63.1", "63"},
		{"at min", "43.0", "43.0", "63.1", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinMaxLinear{}.Normalize(
				mustDecimal(t, tt.raw), mustDecimal(t, tt.min), mustDecimal(t, tt.max))
			if !got.Equal(mustDecimal(t, tt.want)) {
				t.Errorf("Normalize(%s, %s, %s) = %s, want %s", tt.raw, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestDirectionReflectionHelper(t *testing.T) {
	if got := directionAdjust(mustDecimal(t, "55.6"), mustDecimal(t, "43.0"), mustDecimal(t, "63.1"), true); !got.Equal(mustDecimal(t, "55.6")) {
		t.Errorf("higher-is-better must leave raw unchanged, got %s", got)
	}
	tests := []struct {
		name string
		raw  string
		want string // final normalized score via MinMaxLinear after reflection
	}{
		{"Kimi cost 0.22", "0.22", "100"},
		{"GPT cost 0.37", "0.37", "93"},
		{"Opus cost 2.34", "2.34", "0"},
	}
	min, max := mustDecimal(t, "0.22"), mustDecimal(t, "2.34")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjusted := directionAdjust(mustDecimal(t, tt.raw), min, max, false)
			got := MinMaxLinear{}.Normalize(adjusted, min, max)
			if !got.Equal(mustDecimal(t, tt.want)) {
				t.Errorf("score = %s, want %s (adjusted %s)", got, tt.want, adjusted)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	err := &Error{Code: ErrInvalidRaw, Message: "x"}
	if got := err.Error(); got != "x" {
		t.Errorf("Error() = %q, want %q", got, "x")
	}
	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
	wrapped := fmt.Errorf("wrap: %w", &Error{Code: ErrUnknownAggregator, Message: "unknown aggregator: sum"})
	var recovered *Error
	if !errors.As(wrapped, &recovered) {
		t.Fatal("errors.As did not recover *Error")
	}
	if recovered.Code != ErrUnknownAggregator {
		t.Errorf("Code = %v, want ErrUnknownAggregator", recovered.Code)
	}
}

func TestScoreDoesNotImportFetch(t *testing.T) {
	// Go runs tests with cwd = package dir; walk the package itself.
	scoreDir := "."
	err := filepath.WalkDir(scoreDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "catalog/fetch") || strings.Contains(p, "httpkit") {
				t.Errorf("%s imports forbidden package %s", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", scoreDir, err)
	}
}

// --- T2: WeightedArithmeticMean, name resolution ---------------------------

func TestWeightedArithmeticMean(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		weights  []string
		want     string
		ok       bool
	}{
		{"equal weights sum", []string{"90", "90", "90"}, []string{"5", "3", "2"}, "90", true},
		{"se=50 golden", []string{"100", "0"}, []string{"1", "1"}, "50", true},
		{"655/7 rounds half-up", []string{"91", "100"}, []string{"5", "2"}, "94", true},
		{"empty", nil, nil, "0", false},
		{"single", []string{"80"}, []string{"5"}, "80", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make([]sdecimal.Decimal, len(tt.values))
			for i, v := range tt.values {
				values[i] = mustDecimal(t, v)
			}
			weights := make([]sdecimal.Decimal, len(tt.weights))
			for i, v := range tt.weights {
				weights[i] = mustDecimal(t, v)
			}
			got, ok := WeightedArithmeticMean{}.Aggregate(values, weights)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !got.Equal(mustDecimal(t, tt.want)) {
				t.Errorf("Aggregate = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNameResolution(t *testing.T) {
	if _, err := ResolveNormalizer(NormalizerNameMinMaxLinear); err != nil {
		t.Errorf("ResolveNormalizer(%q) = %v, want nil", NormalizerNameMinMaxLinear, err)
	}
	if _, err := ResolveAggregator(AggregatorNameWeightedArithmeticMean); err != nil {
		t.Errorf("ResolveAggregator(%q) = %v, want nil", AggregatorNameWeightedArithmeticMean, err)
	}
	tests := []struct {
		name    string
		got     error
		wantMsg string
		want    ErrorCode
	}{
		{"bogus normalizer", func() error { _, err := ResolveNormalizer("bogus"); return err }(), "unknown normalizer: bogus", ErrUnknownNormalizer},
		{"sum aggregator", func() error { _, err := ResolveAggregator("sum"); return err }(), "unknown aggregator: sum", ErrUnknownAggregator},
		{"empty normalizer", func() error { _, err := ResolveNormalizer(""); return err }(), "unknown normalizer: ", ErrUnknownNormalizer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var se *Error
			if !errors.As(tt.got, &se) {
				t.Fatalf("error = %v, want *Error", tt.got)
			}
			if se.Code != tt.want {
				t.Errorf("Code = %v, want %v", se.Code, tt.want)
			}
			if se.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", se.Message, tt.wantMsg)
			}
		})
	}
}

func TestNameConstants(t *testing.T) {
	if NormalizerNameMinMaxLinear != "minmax-linear" {
		t.Errorf("NormalizerNameMinMaxLinear = %q", NormalizerNameMinMaxLinear)
	}
	if AggregatorNameWeightedArithmeticMean != "weighted-arithmetic-mean" {
		t.Errorf("AggregatorNameWeightedArithmeticMean = %q", AggregatorNameWeightedArithmeticMean)
	}
}

func TestDefaultInstancesMatchConcrete(t *testing.T) {
	values := []sdecimal.Decimal{mustDecimal(t, "91"), mustDecimal(t, "100")}
	weights := []sdecimal.Decimal{mustDecimal(t, "5"), mustDecimal(t, "2")}
	got, ok := DefaultAggregator().Aggregate(values, weights)
	want, _ := WeightedArithmeticMean{}.Aggregate(values, weights)
	if !ok || !got.Equal(want) {
		t.Errorf("DefaultAggregator = (%s, %v), want (%s, true)", got, ok, want)
	}
	if got := DefaultNormalizer().Normalize(mustDecimal(t, "55.6"), mustDecimal(t, "43.0"), mustDecimal(t, "63.1")); !got.Equal(mustDecimal(t, "63")) {
		t.Errorf("DefaultNormalizer = %s, want 63", got)
	}
}

// --- T3: raw CSV parsing, validation, merge ---------------------------------

func TestRawParseGoldenHeader(t *testing.T) {
	input := rawCSV(goldenHeader(), "Claude Opus 5,max,63.1,465,2.34,61,78.0,59.2")
	rows, flags, dynamic, err := parseRawCSV(input)
	if err != nil {
		t.Fatalf("parseRawCSV: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if len(dynamic) != 0 {
		t.Errorf("dynamic = %v, want empty", dynamic)
	}
	row := rows[0]
	if row.model != "Claude Opus 5" || row.reasoning != "max" {
		t.Errorf("identity = %q / %q", row.model, row.reasoning)
	}
	for i, want := range []string{"63.1", "465", "2.34", "61", "78.0", "59.2"} {
		if row.core[i].raw != want {
			t.Errorf("core[%d] = %v, want %s", i, row.core[i], want)
		}
	}
	if !flags["intelligence_index"] || flags["cost_per_intelligence_index_task_usd"] {
		t.Errorf("direction flags wrong: %v", flags)
	}
}

func TestRawValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "too few cells",
			input: rawCSV(goldenHeader(), "Claude X,max,63.1"),
			want:  "row 2: too few values",
		},
		{
			name:  "too many cells",
			input: rawCSV(goldenHeader(), "Claude X,max,63.1,465,2.34,61,78.0,59.2,1,2"),
			want:  "row 2: too many values",
		},
		{
			name:  "blank identity cell",
			input: rawCSV(goldenHeader(), "Claude X,,63.1,465,2.34,61,78.0,59.2"),
			want:  "row 2: reasoning must not be blank",
		},
		{
			name:  "non-numeric cost",
			input: rawCSV(goldenHeader(), "Claude X,max,63.1,465,abc,61,78.0,59.2"),
			want:  "row 2: cost_per_intelligence_index_task_usd must be numeric, got 'abc'",
		},
		{
			name:  "non-finite median",
			input: rawCSV(goldenHeader(), "Claude X,max,63.1,465,2.34,NaN,78.0,59.2"),
			want:  "row 2: median_end_to_end_response_time_seconds must be finite, got 'NaN'",
		},
		{
			name:  "negative cost",
			input: rawCSV(goldenHeader(), "Claude X,max,63.1,465,-1,61,78.0,59.2"),
			want:  "row 2: cost_per_intelligence_index_task_usd must not be negative, got '-1'",
		},
		{
			name: "missing intelligence column",
			input: rawCSV("model,reasoning,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index,benchmark:DeepSWE",
				"Claude X,max,465,2.34,61,78.0,59.2,90"),
			want: "unexpected core columns: expected model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index, got model,reasoning,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index,benchmark:DeepSWE",
		},
		{
			name: "dynamic column without prefix",
			input: rawCSV(goldenHeader()+",benchmark:DeepSWE,plainname",
				"Claude X,max,63.1,465,2.34,61,78.0,59.2,90,1"),
			want: "invalid or duplicate dynamic benchmark columns",
		},
		{
			name: "duplicate dynamic column",
			input: rawCSV(goldenHeader()+",benchmark:DeepSWE,benchmark:DeepSWE",
				"Claude X,max,63.1,465,2.34,61,78.0,59.2,90,1"),
			want: "invalid or duplicate dynamic benchmark columns",
		},
		{
			name:  "empty input",
			input: nil,
			want:  "input contains no data rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parseRawCSV(tt.input)
			if err == nil {
				t.Fatal("parseRawCSV: nil error")
			}
			var se *Error
			if !errors.As(err, &se) || se.Code != ErrInvalidRaw {
				t.Fatalf("error = %v, want *Error{ErrInvalidRaw}", err)
			}
			if got := se.Message; got != tt.want {
				t.Errorf("message = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRawMergeDuplicates(t *testing.T) {
	input := rawCSV(goldenHeader()+",benchmark:DeepSWE",
		"Foo (latest),default,41.9,,,22,,,30",
		"Foo,high,55.6,81,0.37,99,,,70")
	rows, _, _, err := parseRawCSV(input)
	if err != nil {
		t.Fatalf("parseRawCSV: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.model != "Foo" || row.reasoning != "high" {
		t.Errorf("identity = %q / %q, want Foo / high", row.model, row.reasoning)
	}
	if row.core[0].value == nil || row.core[0].value.String() != "41.9" {
		t.Errorf("intelligence = %v, want 41.9 (first-wins)", row.core[0])
	}
	if row.core[1].value == nil || row.core[1].value.String() != "81" {
		t.Errorf("time = %v, want 81 (filled)", row.core[1])
	}
	if row.core[2].value == nil || row.core[2].value.String() != "0.37" {
		t.Errorf("cost = %v, want 0.37 (filled)", row.core[2])
	}
	if row.core[3].value == nil || row.core[3].value.String() != "22" {
		t.Errorf("median = %v, want 22 (first-wins)", row.core[3])
	}
	if row.bench[0].value == nil || row.bench[0].value.String() != "70" {
		t.Errorf("benchmark:DeepSWE = %v, want 70 (max of 30, 70)", row.bench[0])
	}

	if _, _, _, err := parseRawCSV(rawCSV(goldenHeader(), "(latest),default,63.1,465,2.34,61,78.0,59.2")); err == nil {
		t.Fatal("blank-cleaning model: nil error")
	} else if se := err.(*Error); se.Code != ErrInvalidRaw || se.Message != "model name is blank after removing annotations" {
		t.Errorf("blank-cleaning model error = %v, want ErrInvalidRaw with verbatim message", err)
	}
}

func TestRawBlankOptional(t *testing.T) {
	input := rawCSV(goldenHeader(),
		"Claude X,max,63.1,,2.34,61,,59.2")
	rows, _, _, err := parseRawCSV(input)
	if err != nil {
		t.Fatalf("parseRawCSV: %v", err)
	}
	row := rows[0]
	for i, col := range []string{"time_per_intelligence_index_task_seconds", "artificial_analysis_coding_index"} {
		if row.core[map[string]int{
			"time_per_intelligence_index_task_seconds": 1,
			"artificial_analysis_coding_index":         4,
		}[col]].value != nil {
			t.Errorf("%s = %v, want nil", col, row.core[i])
		}
	}
}

// --- T5: SourceScores, CategoryScores, PlanningCapabilityScore --------------

func TestSourceScoresAAPreferred(t *testing.T) {
	row := scoreRowFromMap(t, "Claude Opus 5", "max", map[string]string{
		"artificial_analysis_coding_index_score":      "99",
		"artificial_analysis_agentic_index_score":     "88",
	}, nil)
	row.Benchmarks = parseMap(t, map[string]string{
		"Artificial Analysis Coding Index":      "55",
		"Artificial Analysis Coding Agent Index": "44",
	})
	got := SourceScores(row)
	if v, ok := got["artificialanalysiscodingindex"]; !ok || v.String() != "99" {
		t.Errorf("artificialanalysiscodingindex = %v, want 99", v)
	}
	if v, ok := got["artificialanalysiscodingagentindex"]; !ok || v.String() != "88" {
		t.Errorf("artificialanalysiscodingagentindex = %v, want 88", v)
	}
}

func TestSourceScoresCSVOrderFirstWins(t *testing.T) {
	row := scoreRowFromMap(t, "GPT-5.6 Sol", "medium", nil, nil)
	row.Benchmarks = parseMap(t, map[string]string{
		"Finance Agent": "50",
		"FinanceAgent":  "100",
		"GDPval":        "80",
		"GDPval-AA":     "0",
	})
	got := SourceScores(row)
	if v, ok := got["financeagent"]; !ok || v.String() != "50" {
		t.Errorf("financeagent = %v, want 50 (first-wins)", v)
	}
	if v, ok := got["gdpval"]; !ok || v.String() != "80" {
		t.Errorf("gdpval = %v, want 80 (first-wins)", v)
	}
}

func TestCategoryScoresGolden(t *testing.T) {
	cfg, err := ParseBenchmarkConfig(goldenBenchmarksTOML())
	if err != nil {
		t.Fatalf("ParseBenchmarkConfig: %v", err)
	}
	tests := []struct {
		name       string
		benchmarks map[string]string
	}{
		{"Claude Opus 5", map[string]string{"SWE-Bench Pro": "100", "DeepSWE": "0"}},
		{"GPT-5.6 Sol", map[string]string{"SWE-Bench Pro": "0", "DeepSWE": "100"}},
		{"Kimi K2.7 Code", nil},
	}
	wants := []string{"50", "50", ""}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := scoreRowFromMap(t, tt.name, "max", nil, nil)
			row.Benchmarks = parseMap(t, tt.benchmarks)
			got := CategoryScores(row, cfg, DefaultAggregator())
			if wants[i] == "" {
				if _, ok := got["software_engineering"]; ok {
					t.Errorf("software_engineering present, want absent")
				}
			} else if v, ok := got["software_engineering"]; !ok || v.String() != wants[i] {
				t.Errorf("software_engineering = %v, want %s", v, wants[i])
			}
			if len(got) != map[bool]int{true: 1, false: 0}[wants[i] != ""] {
				t.Errorf("categories = %v, want only software_engineering", got)
			}
		})
	}
}

func TestCategoryMinimumEvidence(t *testing.T) {
	cfg, err := ParseBenchmarkConfig([]byte(`[benchmark_selection]
groups = ["security", "reasoning"]
benchmarks = []

[benchmark_groups.security]
benchmarks = ["DeepSWE"]

[benchmark_groups.reasoning]
benchmarks = ["SWE-Bench Pro"]
`))
	if err != nil {
		t.Fatalf("ParseBenchmarkConfig: %v", err)
	}
	row := scoreRowFromMap(t, "Claude Opus 5", "max", nil, nil)
	row.Benchmarks = parseMap(t, map[string]string{
		"DeepSWE":       "50",
		"SWE-Bench Pro": "50",
	})
	got := CategoryScores(row, cfg, DefaultAggregator())
	if v, ok := got["security"]; !ok || v.String() != "50" {
		t.Errorf("security = %v, want 50 (minimum evidence 1)", v)
	}
	if _, ok := got["reasoning"]; ok {
		t.Error("reasoning present, want absent (minimum evidence 2)")
	}
}

func TestPlanningCapability(t *testing.T) {
	cats := parseMap(t, map[string]string{
		"reasoning":    "80",
		"knowledge":    "85",
		"agentic_tools": "70",
		"research":     "90",
	})
	got := PlanningCapabilityScore(cats)
	if !got.Equal(mustDecimal(t, "81")) {
		t.Errorf("PlanningCapabilityScore = %s, want 81", got)
	}
	missing := parseMap(t, map[string]string{
		"reasoning": "80", "knowledge": "85", "agentic_tools": "70",
	})
	if got := PlanningCapabilityScore(missing); !got.Equal(sdecimal.Zero) {
		t.Errorf("missing research = %s, want 0 (blank)", got)
	}
}

// --- T8: ScoringConfig ------------------------------------------------------

func TestScoringConfigDefaults(t *testing.T) {
	got := DefaultScoringConfig()
	if got.Normalizer != NormalizerNameMinMaxLinear || got.Aggregator != AggregatorNameWeightedArithmeticMean {
		t.Errorf("DefaultScoringConfig() = %+v, want {minmax-linear weighted-arithmetic-mean}", got)
	}
}

// decodeScoringSection decodes a full TOML document the way F01 UnmarshalKey
// does (section table -> re-encode -> decode into the struct).
func decodeScoringSection(t *testing.T, doc string, into *ScoringConfig) {
	t.Helper()
	var table map[string]any
	if _, err := toml.Decode(doc, &table); err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	text, err := toml.Marshal(table["scoring"])
	if err != nil {
		t.Fatalf("toml.Marshal: %v", err)
	}
	if err := toml.Unmarshal(text, into); err != nil {
		t.Fatalf("toml.Unmarshal: %v", err)
	}
}

func TestScoringConfigTOMLTags(t *testing.T) {
	cfg := DefaultScoringConfig()
	decodeScoringSection(t, "[scoring]\nnormalizer = \"minmax-linear\"\naggregator = \"weighted-arithmetic-mean\"\n", &cfg)
	if cfg.Normalizer != "minmax-linear" || cfg.Aggregator != "weighted-arithmetic-mean" {
		t.Errorf("decoded = %+v, want both fields set", cfg)
	}

	empty := DefaultScoringConfig()
	decodeScoringSection(t, "[scoring]\n", &empty)
	if empty.Normalizer != NormalizerNameMinMaxLinear || empty.Aggregator != AggregatorNameWeightedArithmeticMean {
		t.Errorf("empty section decoded = %+v, want defaults", empty)
	}
}

func TestScoringConfigUnknownValue(t *testing.T) {
	cfg := ScoringConfig{Normalizer: "bogus", Aggregator: AggregatorNameWeightedArithmeticMean}
	_, err := ResolveNormalizer(cfg.Normalizer)
	if err == nil || err.Error() != "unknown normalizer: bogus" {
		t.Errorf("ResolveNormalizer(%q) = %v, want %q", cfg.Normalizer, err, "unknown normalizer: bogus")
	}
}
