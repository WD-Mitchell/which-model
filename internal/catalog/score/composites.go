package score

import (
	"sort"

	sdecimal "github.com/shopspring/decimal"
	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// aaSourceColumns are the authoritative AA-index score columns and the
// canonical source names they seed (generate_scores.py _source_scores).
var aaSourceColumns = []struct{ column, sourceName string }{
	{"artificial_analysis_coding_index_score", "Artificial Analysis Coding Index"},
	{"artificial_analysis_agentic_index_score", "Artificial Analysis Coding Agent Index"},
}

// SourceScores resolves a row's evidence map (annex-b §4.5): the two AA
// index score columns first seed canonical keys ("Artificial Analysis Coding
// Index", "Artificial Analysis Coding Agent Index" via identity.BenchmarkKey),
// then each benchmark _score column setdefaults its key (D3 layer a;
// AA-index-preferred over models.dev). Benchmark keys are visited in sorted
// name order for determinism; the derive layer supplies the derived scores,
// so CSV header order is already folded into the map values.
func SourceScores(row catalog.ScoreRow) map[string]sdecimal.Decimal {
	result := make(map[string]sdecimal.Decimal, len(row.Tier1)+len(row.Benchmarks))
	for _, pair := range aaSourceColumns {
		if value, ok := row.Tier1[pair.column]; ok {
			result[identity.BenchmarkKey(pair.sourceName)] = value
		}
	}
	names := make([]string, 0, len(row.Benchmarks))
	for name := range row.Benchmarks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		key := identity.BenchmarkKey(name)
		if _, exists := result[key]; !exists {
			result[key] = row.Benchmarks[name]
		}
	}
	return result
}

// CategoryScores computes the 11 grouped category composites for one row
// (generate_scores.py _category_score): per-group unweighted mean
// (sum/len, ROUND_HALF_UP) of populated evidence in benchmarks.toml group
// list order, deduped by BenchmarkKey (D3 layer b); blank when the number of
// populated evidences is below CategoryMinimumEvidence[group]. Returns a map
// keyed by group id with absent keys for blanks.
func CategoryScores(row catalog.ScoreRow, cfg *BenchmarkConfig) map[string]sdecimal.Decimal {
	if cfg == nil {
		return nil
	}
	sources := SourceScores(row)
	result := make(map[string]sdecimal.Decimal)
	for _, group := range cfg.EvidenceGroups {
		minimum := CategoryMinimumEvidence[group.Category]
		seen := make(map[string]bool)
		var values []sdecimal.Decimal
		for _, name := range group.Benchmarks {
			key := identity.BenchmarkKey(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if value, ok := sources[key]; ok {
				values = append(values, value)
			}
		}
		if len(values) == 0 || len(values) < minimum {
			continue
		}
		sum := sdecimal.Zero
		for _, value := range values {
			sum = sum.Add(value)
		}
		result[group.Category] = wdecimal.RoundHalfUp(
			sum.Div(sdecimal.NewFromInt(int64(len(values)))), 0)
	}
	return result
}

// planningComponents are the fixed PlanningCapability composition weights
// (generate_scores.py _category_score planning branch).
var planningComponents = []struct {
	name   string
	weight sdecimal.Decimal
}{
	{"reasoning", sdecimal.RequireFromString("0.40")},
	{"knowledge", sdecimal.RequireFromString("0.30")},
	{"agentic_tools", sdecimal.RequireFromString("0.20")},
	{"research", sdecimal.RequireFromString("0.10")},
}

// PlanningCapabilityScore = 0.4*reasoning + 0.3*knowledge + 0.2*agentic_tools
// + 0.1*research, ROUND_HALF_UP; zero (blank) when ANY input category score
// is absent. Inputs are the row's computed category scores.
func PlanningCapabilityScore(categoryScores map[string]sdecimal.Decimal) sdecimal.Decimal {
	sum := sdecimal.Zero
	for _, component := range planningComponents {
		value, ok := categoryScores[component.name]
		if !ok {
			return sdecimal.Zero
		}
		sum = sum.Add(value.Mul(component.weight))
	}
	return wdecimal.RoundHalfUp(sum, 0)
}
