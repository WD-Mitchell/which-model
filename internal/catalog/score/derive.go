package score

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	sdecimal "github.com/shopspring/decimal"
)

// coreIndex maps a core metric name to its core[] position
// (RawCoreColumns[2:] order).
func coreIndex(column string) int {
	switch column {
	case "intelligence_index":
		return 0
	case "time_per_intelligence_index_task_seconds":
		return 1
	case "cost_per_intelligence_index_task_usd":
		return 2
	case "median_end_to_end_response_time_seconds":
		return 3
	case "artificial_analysis_coding_index":
		return 4
	case "artificial_analysis_agentic_index":
		return 5
	}
	return -1
}

// Derive is the single public entry point: raw CSV bytes + benchmarks TOML
// bytes -> scores CSV bytes (dual-column schema, annex-b §4.0a), including
// the provenance header line. Pure, offline, deterministic (SPEC D6).
// rawCSV may carry a leading "# raw_sha256=..." line (F06 ProvenancePrefix),
// which is stripped before parsing; the header line emitted uses the SHA-256
// of the raw bytes AS GIVEN (including the stripped line, matching F06
// semantics).
//
// Processing order (generate_scores.py read_rows + _merge_input_rows +
// generate): parse + validate (row numbers start at 2) -> merge duplicate
// identities (CleanModelName + default->high collapse; first-wins fill-in for
// core cells, max for benchmark cells, SPEC D8) -> measured-row filter (at least one published metric; SPEC D9) -> per-column
// min/max ranges over eligible rows -> relative scores (direction-aware
// MinMaxLinear, ROUND_HALF_UP) -> category composites -> provenance header.
func Derive(rawCSV []byte, benchmarksTOML []byte, normalizer Normalizer, aggregator Aggregator) ([]byte, error) {
	sum := sha256.Sum256(rawCSV)
	rawHash := hex.EncodeToString(sum[:])

	rows, flags, dynamic, err := parseRawCSV(rawCSV)
	if err != nil {
		return nil, err
	}
	cfg, err := ParseBenchmarkConfig(benchmarksTOML)
	if err != nil {
		return nil, err
	}

	eligible := measuredRows(rows)
	if len(eligible) == 0 {
		return nil, &Error{Code: ErrInvalidRaw, Message: "input contains no published metric values"}
	}
	ranges := columnRanges(eligible, dynamic)

	state := deriveState{
		dynamic:    dynamic,
		ranges:     ranges,
		flags:      flags,
		cfg:        cfg,
		aggregator: aggregator,
	}
	coreColumns := csvstore.RawCoreColumns[2:]

	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	writer.Write(deriveHeader(coreColumns, dynamic))
	for i := range eligible {
		row := &eligible[i]
		scoreRow := state.scoreRow(row, coreColumns, normalizer)
		record := state.record(row, coreColumns, normalizer, scoreRow)
		writer.Write(record)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("derive: write csv: %w", err)
	}

	provenance := fmt.Sprintf("%s raw_sha256=%s normalizer=%s aggregator=%s",
		csvstore.ProvenancePrefix, rawHash, normalizerName(normalizer), aggregatorName(aggregator))
	return append([]byte(provenance+"\n"), out.Bytes()...), nil
}

// deriveState carries the per-call column layout so row rendering stays
// deterministic (no map iteration).
type deriveState struct {
	dynamic    []string
	ranges     map[string]*[2]sdecimal.Decimal
	flags      map[string]bool
	cfg        *BenchmarkConfig
	aggregator Aggregator
}

// normalizerName/aggregatorName extract a component's canonical name for
// provenance rendering (issue #45): components report their own name via
// Name() so the header reflects the ACTUAL components used, with the
// canonical constants as fallback for nameless custom implementations.
func normalizerName(n Normalizer) string {
	if named, ok := n.(interface{ Name() string }); ok {
		return named.Name()
	}
	return NormalizerNameMinMaxLinear
}

func aggregatorName(a Aggregator) string {
	if named, ok := a.(interface{ Name() string }); ok {
		return named.Name()
	}
	return AggregatorNameWeightedArithmeticMean
}

// measuredRows retains any row with published evidence, including benchmark-only
// rows. Missing unrelated measurements never discard known values.
func measuredRows(rows []rawRow) []rawRow {
	var measured []rawRow
	for _, row := range rows {
		present := false
		for _, cells := range [][]rawCell{row.core, row.bench} {
			for _, cell := range cells {
				if cell.value != nil {
					present = true
					break
				}
			}
		}
		if present {
			measured = append(measured, row)
		}
	}
	return measured
}

// columnRanges uses all published values independently per column. Empty,
// singleton, or constant ranges have no relative score; absolute values survive.
func columnRanges(rows []rawRow, dynamic []string) map[string]*[2]sdecimal.Decimal {
	benchIndex := make(map[string]int, len(dynamic))
	for i, name := range dynamic {
		benchIndex[name] = i
	}
	columns := append(append([]string{}, csvstore.RawCoreColumns[2:]...), dynamic...)
	result := make(map[string]*[2]sdecimal.Decimal, len(columns))
	for _, column := range columns {
		cells := metricCells(rows, column, benchIndex)
		if len(cells) == 0 {
			continue
		}
		min, max := *cells[0].value, *cells[0].value
		for _, cell := range cells[1:] {
			if cell.value.LessThan(min) {
				min = *cell.value
			}
			if cell.value.GreaterThan(max) {
				max = *cell.value
			}
		}
		if !min.Equal(max) {
			result[column] = &[2]sdecimal.Decimal{min, max}
		}
	}
	return result
}

// metricCells collects the non-nil cells of one column across eligible rows,
// in input order. Dynamic columns are located via benchIndex.
func metricCells(eligible []rawRow, column string, benchIndex map[string]int) []rawCell {
	var cells []rawCell
	if idx := coreIndex(column); idx >= 0 {
		for i := range eligible {
			if eligible[i].core[idx].value != nil {
				cells = append(cells, eligible[i].core[idx])
			}
		}
		return cells
	}
	idx := benchIndex[column]
	for i := range eligible {
		if eligible[i].bench[idx].value != nil {
			cells = append(cells, eligible[i].bench[idx])
		}
	}
	return cells
}

// deriveHeader builds the dual-column header: model,reasoning, then core
// pairs (<metric>, <metric>_score), then the 12 category _score columns,
// then benchmark pairs in raw dynamic order.
func deriveHeader(coreColumns, dynamic []string) []string {
	header := make([]string, 0, 2+2*len(coreColumns)+len(csvstore.CategoryScoreColumns)+2*len(dynamic))
	header = append(header, "model", "reasoning")
	for _, name := range coreColumns {
		header = append(header, name, name+"_score")
	}
	header = append(header, csvstore.CategoryScoreColumns...)
	for _, name := range dynamic {
		header = append(header, name, name+"_score")
	}
	return header
}

// scoreRow builds the catalog.ScoreRow consumed by the composites: Tier1
// keyed by the 6 _score names, Benchmarks by plain name; absent keys for
// blank cells.
func (s *deriveState) scoreRow(row *rawRow, coreColumns []string, normalizer Normalizer) catalog.ScoreRow {
	scoreRow := catalog.ScoreRow{
		Model:      row.model,
		Reasoning:  row.reasoning,
		Tier1:      make(map[string]sdecimal.Decimal),
		Categories: make(map[string]sdecimal.Decimal),
		Benchmarks: make(map[string]sdecimal.Decimal),
	}
	for i, name := range coreColumns {
		if value := s.scoreValue(&row.core[i], s.ranges[name], s.flags[name], normalizer); value != nil {
			scoreRow.Tier1[name+"_score"] = *value
		}
	}
	for i, name := range s.dynamic {
		if value := s.scoreValue(&row.bench[i], s.ranges[name], true, normalizer); value != nil {
			scoreRow.Benchmarks[strings.TrimPrefix(name, csvstore.BenchmarkColumnPrefix)] = *value
		}
	}
	return scoreRow
}

// record renders one output row: identity, core pairs (abs verbatim + score),
// category cells, benchmark pairs.
func (s *deriveState) record(row *rawRow, coreColumns []string, normalizer Normalizer, scoreRow catalog.ScoreRow) []string {
	header := deriveHeader(coreColumns, s.dynamic)
	record := make([]string, 0, len(header))
	record = append(record, row.model, row.reasoning)

	categories := CategoryScores(scoreRow, s.cfg, s.aggregator)
	planning := PlanningCapabilityScore(categories)

	for i, name := range coreColumns {
		record = append(record, rawText(&row.core[i]), s.scoreText(&row.core[i], s.ranges[name], s.flags[name], normalizer))
	}
	for _, column := range csvstore.CategoryScoreColumns {
		if column == "planning_capability_score" {
			if planningComplete(categories) {
				record = append(record, planning.String())
			} else {
				record = append(record, "")
			}
			continue
		}
		group := strings.TrimSuffix(column, "_score")
		if value, ok := categories[group]; ok {
			record = append(record, value.String())
		} else {
			record = append(record, "")
		}
	}
	for i, name := range s.dynamic {
		record = append(record, rawText(&row.bench[i]), s.scoreText(&row.bench[i], s.ranges[name], true, normalizer))
	}
	return record
}

// planningComplete reports whether all four planning components are present
// in the row's category scores (PlanningCapabilityScore returns zero when
// any is missing; the output cell must be blank in that case).
func planningComplete(categories map[string]sdecimal.Decimal) bool {
	for _, component := range planningComponents {
		if _, ok := categories[component.name]; !ok {
			return false
		}
	}
	return true
}

// rawText renders an absolute cell verbatim: the raw cell text (blank
// preserved), matching Python str(Decimal) for plain-decimal inputs (trailing
// zeros kept, e.g. "78.0"; shopspring's String drops them).
func rawText(cell *rawCell) string {
	return strings.TrimSpace(cell.raw)
}

// scoreText renders the relative cell: "" when the raw value or the column
// range is absent.
func (s *deriveState) scoreText(cell *rawCell, columnRange *[2]sdecimal.Decimal, higherIsBetter bool, normalizer Normalizer) string {
	value := s.scoreValue(cell, columnRange, higherIsBetter, normalizer)
	if value == nil {
		return ""
	}
	return value.String()
}

// scoreValue computes the direction-adjusted normalized score, or nil when
// the cell or range is absent.
func (s *deriveState) scoreValue(cell *rawCell, columnRange *[2]sdecimal.Decimal, higherIsBetter bool, normalizer Normalizer) *sdecimal.Decimal {
	if cell.value == nil || columnRange == nil {
		return nil
	}
	adjusted := directionAdjust(*cell.value, columnRange[0], columnRange[1], higherIsBetter)
	value := normalizer.Normalize(adjusted, columnRange[0], columnRange[1])
	return &value
}
