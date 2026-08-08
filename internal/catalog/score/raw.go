package score

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	sdecimal "github.com/shopspring/decimal"
	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// Core metric directions (generate_scores.py CORE_METRICS, SPEC §2.2):
// higher-is-better for intelligence/coding/agentic, lower-is-better for
// the three latency/cost metrics.
var coreMetricDirections = map[string]bool{
	"intelligence_index":                            true,
	"time_per_intelligence_index_task_seconds":      false,
	"cost_per_intelligence_index_task_usd":          false,
	"median_end_to_end_response_time_seconds":       false,
	"artificial_analysis_coding_index":              true,
	"artificial_analysis_agentic_index":             true,
}

// optionalMetrics are the columns whose range problems (no values or a
// degenerate min==max) blank the score column instead of erroring
// (generate_scores.py OPTIONAL_METRICS).
var optionalMetrics = map[string]bool{
	"time_per_intelligence_index_task_seconds": true,
	"artificial_analysis_coding_index":         true,
	"artificial_analysis_agentic_index":        true,
}

// nullableMetrics are the columns whose blank cells parse to nil instead of
// erroring (generate_scores.py NULLABLE_METRICS: every core metric).
var nullableMetrics = map[string]bool{
	"intelligence_index":                       true,
	"time_per_intelligence_index_task_seconds": true,
	"cost_per_intelligence_index_task_usd":     true,
	"median_end_to_end_response_time_seconds":  true,
	"artificial_analysis_coding_index":         true,
	"artificial_analysis_agentic_index":        true,
}

// requiredTier1Metrics are the three metrics every eligible row must have
// (generate_scores.py REQUIRED_TIER1_METRICS).
var requiredTier1Metrics = []string{
	"intelligence_index",
	"median_end_to_end_response_time_seconds",
	"cost_per_intelligence_index_task_usd",
}

// rawCell is one parsed raw cell: the verbatim text (blank = "") and the
// parsed value (nil when blank).
type rawCell struct {
	raw   string
	value *sdecimal.Decimal
}

// rawRow is one merged raw data row. core holds the six metric cells in
// RawCoreColumns[2:] order; bench is parallel to the dynamic column list.
type rawRow struct {
	model     string
	reasoning string
	core      []rawCell
	bench     []rawCell
}

func rawError(format string, args ...any) error {
	return &Error{Code: ErrInvalidRaw, Message: fmt.Sprintf(format, args...)}
}

// parseRawCSV parses, validates, and duplicate-merges a raw CSV. Returns the
// merged rows, the higher-is-better flag per metric column (core + dynamic),
// and the dynamic column names in header order. A single leading '#' line
// (F06 ProvenancePrefix) is stripped before parsing.
func parseRawCSV(data []byte) ([]rawRow, map[string]bool, []string, error) {
	data = stripRawProvenanceLine(data)
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		// No header at all: empty input.
		return nil, nil, nil, rawError("input contains no data rows")
	}

	core, err := checkRawHeader(header)
	if err != nil {
		return nil, nil, nil, err
	}
	dynamic := header[len(core):]

	flags := make(map[string]bool, len(core)+len(dynamic))
	for _, name := range core {
		flags[name] = coreMetricDirections[name]
	}
	for _, name := range dynamic {
		flags[name] = true // benchmarks are higher-is-better
	}

	columnIndex := make(map[string]int, len(header))
	for i, name := range header {
		columnIndex[name] = i
	}

	var rows []rawRow
	for rowNumber := 2; ; rowNumber++ {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, rawError("row %d: %v", rowNumber, err)
		}
		if len(fields) > len(header) {
			return nil, nil, nil, rawError("row %d: too many values", rowNumber)
		}
		if len(fields) < len(header) {
			return nil, nil, nil, rawError("row %d: too few values", rowNumber)
		}

		cell := func(column string) string {
			return fields[columnIndex[column]]
		}
		row := rawRow{
			core:  make([]rawCell, len(core)-2),
			bench: make([]rawCell, len(dynamic)),
		}
		model := strings.TrimSpace(cell("model"))
		reasoning := strings.TrimSpace(cell("reasoning"))
		if model == "" {
			return nil, nil, nil, rawError("row %d: model must not be blank", rowNumber)
		}
		if reasoning == "" {
			return nil, nil, nil, rawError("row %d: reasoning must not be blank", rowNumber)
		}
		row.model, row.reasoning = model, reasoning

		for i, name := range core[2:] {
			value, err := parseRawNumber(cell(name), name, rowNumber)
			if err != nil {
				return nil, nil, nil, err
			}
			row.core[i] = rawCell{raw: cell(name), value: value}
		}
		for i, name := range dynamic {
			value, err := parseRawNumber(cell(name), name, rowNumber)
			if err != nil {
				return nil, nil, nil, err
			}
			row.bench[i] = rawCell{raw: cell(name), value: value}
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, nil, nil, rawError("input contains no data rows")
	}
	merged, err := mergeRawRows(rows)
	if err != nil {
		return nil, nil, nil, err
	}
	return merged, flags, dynamic, nil
}

// stripRawProvenanceLine removes one leading '#' line if present (F06
// ProvenancePrefix semantics: Derive accepts a provenance-carrying raw).
func stripRawProvenanceLine(data []byte) []byte {
	if len(data) == 0 || data[0] != '#' {
		return data
	}
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[idx+1:]
	}
	return nil
}

// checkRawHeader validates the core columns and the dynamic benchmark
// columns, returning the core column names.
func checkRawHeader(header []string) ([]string, error) {
	core := csvstore.RawCoreColumns
	prefix := header
	if len(prefix) > len(core) {
		prefix = prefix[:len(core)]
	}
	if len(prefix) != len(core) {
		return nil, rawError("unexpected core columns: expected %s, got %s",
			strings.Join(core, ","), strings.Join(header, ","))
	}
	for i, name := range prefix {
		if name != core[i] {
			return nil, rawError("unexpected core columns: expected %s, got %s",
				strings.Join(core, ","), strings.Join(header, ","))
		}
	}
	dynamic := header[len(core):]
	seen := make(map[string]bool, len(dynamic))
	for _, name := range dynamic {
		if !strings.HasPrefix(name, csvstore.BenchmarkColumnPrefix) || name == csvstore.BenchmarkColumnPrefix {
			return nil, rawError("invalid or duplicate dynamic benchmark columns")
		}
		if seen[name] {
			return nil, rawError("invalid or duplicate dynamic benchmark columns")
		}
		seen[name] = true
	}
	return core, nil
}

// isNonFiniteToken reports whether s is one of the non-finite forms
// Python's Decimal accepts (NaN, sNaN, ±Inf, ±Infinity, case-insensitive).
// shopspring cannot represent these, so they are rejected before parsing so
// the Python-verbatim "must be finite" message fires.
func isNonFiniteToken(s string) bool {
	switch strings.ToLower(s) {
	case "nan", "+nan", "-nan", "snan", "+snan", "-snan",
		"inf", "+inf", "-inf", "infinity", "+infinity", "-infinity":
		return true
	}
	return false
}

// parseRawNumber parses one metric cell: blank -> nil (nullable columns) or
// "must not be blank"; non-numeric -> "must be numeric"; non-finite ->
// "must be finite"; negative latency/cost -> "must not be negative".
func parseRawNumber(cell, column string, rowNumber int) (*sdecimal.Decimal, error) {
	stripped := strings.TrimSpace(cell)
	if stripped == "" {
		if nullableMetrics[column] || strings.HasPrefix(column, csvstore.BenchmarkColumnPrefix) {
			return nil, nil
		}
		return nil, rawError("row %d: %s must not be blank", rowNumber, column)
	}
	if isNonFiniteToken(stripped) {
		return nil, rawError("row %d: %s must be finite, got '%s'", rowNumber, column, cell)
	}
	value, err := wdecimal.Parse(stripped)
	if err != nil {
		return nil, rawError("row %d: %s must be numeric, got '%s'", rowNumber, column, cell)
	}
	if csvstore.NonNegativeRawColumns[column] && value.IsNegative() {
		return nil, rawError("row %d: %s must not be negative, got '%s'", rowNumber, column, cell)
	}
	return &value, nil
}

// mergeRawRows ports _merge_input_rows (generate_scores.py:163-206):
// CleanModelName + "default"->"high" reasoning collapse, first-wins fill-in
// for core cells, max for benchmark cells, in input order.
func mergeRawRows(rows []rawRow) ([]rawRow, error) {
	type identityKey struct{ model, reasoning string }
	grouped := make(map[identityKey]int)
	var merged []rawRow

	for i := range rows {
		row := &rows[i]
		model := identity.CleanModelName(row.model)
		if model == "" {
			return nil, rawError("model name is blank after removing annotations")
		}
		reasoning := identity.CollapseReasoning(row.reasoning)
		key := identityKey{model: model, reasoning: reasoning}
		if _, ok := grouped[key]; !ok {
			grouped[key] = len(merged)
			row.model, row.reasoning = model, reasoning
			merged = append(merged, *row)
			continue
		}
		target := &merged[grouped[key]]
		for j := range row.core {
			if row.core[j].value == nil {
				continue
			}
			if target.core[j].value == nil {
				target.core[j] = row.core[j]
			}
		}
		for j := range row.bench {
			if row.bench[j].value == nil {
				continue
			}
			if target.bench[j].value == nil {
				target.bench[j] = row.bench[j]
			} else if row.bench[j].value.GreaterThan(*target.bench[j].value) {
				target.bench[j] = row.bench[j]
			}
		}
	}
	return merged, nil
}
