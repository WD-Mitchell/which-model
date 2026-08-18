package pick

import (
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// ModelScore is one fully scored candidate row (annex-b §5.8). Decimal
// fields serialize via decimal.Decimal.MarshalJSON (text-based,
// precision-preserving); ExcludedReasons is never serialized.
type ModelScore struct {
	Model             string                     `json:"model"`
	Reasoning         string                     `json:"reasoning"`
	Total             decimal.Decimal            `json:"total_score"`
	Tier1             decimal.Decimal            `json:"tier1_score"`
	Tier2             *decimal.Decimal           `json:"tier2_score"` // null when no tier-2 evidence
	Tier1Contribution decimal.Decimal            `json:"tier1_contribution"`
	Tier2Contribution decimal.Decimal            `json:"tier2_contribution"`
	Categories        map[string]decimal.Decimal `json:"category_scores"` // only populated categories
	Warnings          []string                   `json:"warnings"`
	ExcludedReasons   []string                   `json:"-"` // never serialized
}

// ExcludedRow is one row removed by the tier-1 or availability filters.
type ExcludedRow struct {
	Model     string   `json:"model"`
	Reasoning string   `json:"reasoning"`
	Reasons   []string `json:"reasons"`
}

// ScoreModel computes tier1/tier2/total for ONE row (rank_models.py:371-427,
// annex-b §5.3 verbatim). Missing tier-1 axis scores produce
// ExcludedReasons=["missing_tier1:<joined missing axes in Tier1AxisOrder>"]
// with all score fields zeroed; otherwise ExcludedReasons is empty.
func ScoreModel(row catalog.ScoreRow, profile catalog.Profile) ModelScore {
	ms := ModelScore{
		Model:      row.Model,
		Reasoning:  row.Reasoning,
		Categories: map[string]decimal.Decimal{},
		Warnings:   []string{},
	}
	// Steps 1-2: read the 3 axis scores; a row missing any axis is excluded
	// (no imputation, hard cut) before any availability filtering.
	var values, weights []decimal.Decimal
	var missing []string
	for _, axis := range Tier1AxisOrder {
		v, ok := row.Tier1[Tier1ScoreColumn[axis]]
		if !ok {
			missing = append(missing, string(axis))
			continue
		}
		values = append(values, v)
		weights = append(weights, profile.Tier1Weights[string(axis)])
	}
	if len(missing) > 0 {
		ms.ExcludedReasons = []string{"missing_tier1:" + strings.Join(missing, ",")}
		return ms
	}
	// Step 3: weighted mean over exactly the 3 fixed axes, independent of
	// Tier1Share.
	tier1, ok := wdecimal.WeightedMean(values, weights)
	if !ok {
		panic("pick: tier1 WeightedMean produced no value for a complete row")
	}
	ms.Tier1 = tier1
	// Steps 4-6: tier-2 categories with positive weight, iterated in
	// CategoryNames order (F10 SPEC D2), then any remaining tier-2 keys
	// (custom benchmark-group slugs, B05 SPEC §2.11) in sorted order. A
	// blank category never excludes.
	var catValues, catWeights []decimal.Decimal
	var missingOptional []string
	for _, name := range categoryIterationOrder(profile) {
		w, ok := profile.Tier2Weights[name]
		if !ok || w.Sign() <= 0 {
			continue
		}
		v, ok := row.Categories[name]
		if !ok {
			missingOptional = append(missingOptional, name)
			continue
		}
		catValues = append(catValues, v)
		catWeights = append(catWeights, w)
		ms.Categories[name] = v
	}
	if len(missingOptional) > 0 {
		ms.Warnings = append(ms.Warnings, "missing optional category scores: "+strings.Join(missingOptional, ", "))
	}
	if len(catValues) > 0 {
		// Renormalize over only the categories with data — missing
		// categories are excluded from both numerator and denominator.
		tier2, ok := wdecimal.WeightedMean(catValues, catWeights)
		if !ok {
			panic("pick: tier2 WeightedMean produced no value with category data")
		}
		ms.Tier2 = &tier2
	} else if len(profile.Tier2Weights) > 0 {
		ms.Warnings = append(ms.Warnings, "no optional task-category scores available; Tier 1 score used")
	}
	// Step 7: combination — the documented no-tier-2 asymmetry. A row with
	// zero tier-2 evidence keeps the RAW, un-shared tier-1 score.
	if ms.Tier2 == nil {
		ms.Total = tier1
		ms.Tier1Contribution = tier1
		ms.Tier2Contribution = decimal.Zero
	} else {
		ms.Tier1Contribution = tier1.Mul(profile.Tier1Share).Div(decimal.NewFromInt(100))
		ms.Tier2Contribution = ms.Tier2.Mul(profile.Tier2Share).Div(decimal.NewFromInt(100))
		ms.Total = ms.Tier1Contribution.Add(ms.Tier2Contribution)
	}
	return ms
}

// categoryIterationOrder returns the deterministic tier-2 iteration order
// for one profile: CategoryNames first (F10 SPEC D2), then any remaining
// Tier2Weights keys (custom benchmark-group slugs, B05 SPEC §2.11) sorted
// ascending. Custom slugs are intentionally excluded from CategoryNames' own
// order so the canonical 12 keep their fixed sequence.
func categoryIterationOrder(p catalog.Profile) []string {
	if len(p.Tier2Weights) <= len(CategoryNames) {
		allNamed := true
		for key := range p.Tier2Weights {
			if !categoryNameSet[key] {
				allNamed = false
				break
			}
		}
		if allNamed {
			return CategoryNames
		}
	}
	out := make([]string, 0, len(CategoryNames)+len(p.Tier2Weights))
	out = append(out, CategoryNames...)
	var customs []string
	for key := range p.Tier2Weights {
		if !categoryNameSet[key] {
			customs = append(customs, key)
		}
	}
	sort.Strings(customs)
	out = append(out, customs...)
	return out
}

// categoryNameSet is the membership test backing categoryIterationOrder.
var categoryNameSet = func() map[string]bool {
	m := make(map[string]bool, len(CategoryNames))
	for _, name := range CategoryNames {
		m[name] = true
	}
	return m
}()

// init serializes decimal fields as unquoted JSON numbers: TASKS F10-T7
// test 1 pins json.Number for total_score/contributions (and annex-b §5.8
// numbers, unlike Python's _json_safe float conversion). Still
// decimal.Decimal.MarshalJSON — the package flag only changes the emitted
// form, never the precision. No other package in the tree asserts quoted
// decimal output, so the flag flip is inert outside pick.
func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

// Result mirrors the Python output object (annex-b §5.8): recommendation =
// first ranked survivor, alternatives = every other ranked survivor
// (top-N truncation is F22 CLI scope), candidate_count = survivors of both
// filters, availability_filter_applied = whether a filter was supplied.
type Result struct {
	Profile                   string        `json:"profile"`
	Recommendation            ModelScore    `json:"recommendation"`
	Alternatives              []ModelScore  `json:"alternatives"`
	Excluded                  []ExcludedRow `json:"excluded"`
	CandidateCount            int           `json:"candidate_count"`
	AvailabilityFilterApplied bool          `json:"availability_filter_applied"`
}

// rankedEntry pairs a scored model with its source row so the tie-break
// comparator can read the raw tier-1 axis scores (the _tie_* sort keys of
// rank_models.py; never serialized).
type rankedEntry struct {
	ms  ModelScore
	row catalog.ScoreRow
}

// Rank ranks all rows: tier-1 exclusion, scoring, availability filter
// (applied last), 7-key tie-break sort, excluded-row sort (SPEC D7).
// available == nil means no filter. Returns *NoCandidatesError when zero
// candidates remain (two distinct messages, SPEC §2.11). Precondition:
// rows have unique (model, reasoning) identities (enforced by
// score.ParseScoresCSV, F09).
func Rank(rows []catalog.ScoreRow, profile catalog.Profile, available []Identity) (Result, error) {
	if err := ValidateProfile(profile); err != nil {
		return Result{}, err
	}
	// A supplied-but-empty filter matches nothing (parse_availability's
	// combined-set check, rank_models.py:471-474).
	if available != nil && len(available) == 0 {
		return Result{}, &RankingError{Message: "availability filter was supplied but contains no identities"}
	}
	excluded := []ExcludedRow{}
	entries := []rankedEntry{}
	for _, row := range rows {
		ms := ScoreModel(row, profile)
		if len(ms.ExcludedReasons) > 0 {
			excluded = append(excluded, ExcludedRow{Model: ms.Model, Reasoning: ms.Reasoning, Reasons: ms.ExcludedReasons})
			continue
		}
		entries = append(entries, rankedEntry{ms: ms, row: row})
	}
	// Availability is the final eligibility filter: exact tuple membership.
	if available != nil {
		availableSet := make(map[Identity]bool, len(available))
		for _, id := range available {
			availableSet[id] = true
		}
		kept := entries[:0]
		for _, entry := range entries {
			if availableSet[Identity{Model: entry.ms.Model, Reasoning: entry.ms.Reasoning}] {
				kept = append(kept, entry)
			} else {
				excluded = append(excluded, ExcludedRow{
					Model:     entry.ms.Model,
					Reasoning: entry.ms.Reasoning,
					Reasons:   []string{"not_live_available"},
				})
			}
		}
		entries = kept
	}
	if len(entries) == 0 {
		if available != nil {
			return Result{}, &NoCandidatesError{Message: "no candidates remain after live model-and-effort availability and Tier 1 filtering"}
		}
		return Result{}, &NoCandidatesError{Message: "no candidates contain all mandatory Tier 1 scores"}
	}
	sort.Slice(entries, func(i, j int) bool { return rankLess(entries[i], entries[j]) })
	sort.SliceStable(excluded, func(i, j int) bool {
		return excludedLess(excluded[i], excluded[j])
	})
	ranked := make([]ModelScore, len(entries))
	for i := range entries {
		ranked[i] = entries[i].ms
	}
	return Result{
		Profile:                   profile.Name,
		Recommendation:            ranked[0],
		Alternatives:              ranked[1:],
		Excluded:                  excluded,
		CandidateCount:            len(ranked),
		AvailabilityFilterApplied: available != nil,
	}, nil
}

// rankLess is the exact 7-key tie-break tuple of rank_models.py:439-449
// (annex-b §5.4), with early return on the first non-equal key: total DESC,
// raw tier-1 intelligence DESC, tier-2 contribution DESC, raw tier-1 speed
// DESC, raw tier-1 cost DESC, casefolded model ASC, casefolded reasoning ASC.
func rankLess(a, b rankedEntry) bool {
	if c := a.ms.Total.Cmp(b.ms.Total); c != 0 {
		return c > 0
	}
	if c := a.row.Tier1[Tier1ScoreColumn[AxisIntelligence]].Cmp(b.row.Tier1[Tier1ScoreColumn[AxisIntelligence]]); c != 0 {
		return c > 0
	}
	if c := a.ms.Tier2Contribution.Cmp(b.ms.Tier2Contribution); c != 0 {
		return c > 0
	}
	if c := a.row.Tier1[Tier1ScoreColumn[AxisSpeed]].Cmp(b.row.Tier1[Tier1ScoreColumn[AxisSpeed]]); c != 0 {
		return c > 0
	}
	if c := a.row.Tier1[Tier1ScoreColumn[AxisCost]].Cmp(b.row.Tier1[Tier1ScoreColumn[AxisCost]]); c != 0 {
		return c > 0
	}
	if c := strings.Compare(strings.ToLower(a.ms.Model), strings.ToLower(b.ms.Model)); c != 0 {
		return c < 0
	}
	return strings.ToLower(a.ms.Reasoning) < strings.ToLower(b.ms.Reasoning)
}

// excludedLess orders excluded rows by casefolded (model, reasoning)
// ascending (SPEC D7; sort.SliceStable preserves application order for
// ties, matching Python's stable sorted()).
func excludedLess(a, b ExcludedRow) bool {
	if c := strings.Compare(strings.ToLower(a.Model), strings.ToLower(b.Model)); c != 0 {
		return c < 0
	}
	return strings.ToLower(a.Reasoning) < strings.ToLower(b.Reasoning)
}
