package csvstore

import (
	"fmt"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/decimal"
)

// ValidateRows: non-empty; uniform headers; Values length == Header length;
// model and reasoning cells non-blank; no duplicate (model, reasoning)
// identity. Errors: ErrMalformedCSV, ErrDuplicateIdentity.
func ValidateRows(rows []Row) error {
	if len(rows) == 0 {
		return fmt.Errorf("%w: no data rows", ErrMalformedCSV)
	}
	header := rows[0].Header
	modelIdx, reasoningIdx := -1, -1
	for i, name := range header {
		switch name {
		case "model":
			modelIdx = i
		case "reasoning":
			reasoningIdx = i
		}
	}
	if modelIdx < 0 || reasoningIdx < 0 {
		return fmt.Errorf("%w: no model/reasoning identity columns", ErrMalformedCSV)
	}
	seen := map[[2]string]bool{}
	for _, row := range rows {
		if len(row.Header) != len(header) || len(row.Values) != len(header) {
			return fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
		}
		for i := range header {
			if row.Header[i] != header[i] {
				return fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
			}
		}
		if row.Values[modelIdx] == "" || row.Values[reasoningIdx] == "" {
			return fmt.Errorf("%w: blank identity", ErrMalformedCSV)
		}
		key := [2]string{row.Values[modelIdx], row.Values[reasoningIdx]}
		if seen[key] {
			return fmt.Errorf("%w: %s / %s", ErrDuplicateIdentity, key[0], key[1])
		}
		seen[key] = true
	}
	return nil
}

// ValidateRawHeader: first 8 columns are exactly RawCoreColumns in order;
// extras all start with BenchmarkColumnPrefix and have non-empty names; no
// duplicate benchmark names. Errors: ErrMalformedCSV.
func ValidateRawHeader(header []string) error {
	if len(header) < len(RawCoreColumns) {
		return fmt.Errorf("%w: unexpected core columns", ErrMalformedCSV)
	}
	for i, want := range RawCoreColumns {
		if header[i] != want {
			return fmt.Errorf("%w: unexpected core columns", ErrMalformedCSV)
		}
	}
	seen := map[string]bool{}
	for _, name := range header[len(RawCoreColumns):] {
		if !strings.HasPrefix(name, BenchmarkColumnPrefix) || name == BenchmarkColumnPrefix {
			return fmt.Errorf("%w: invalid dynamic benchmark column %q", ErrMalformedCSV, name)
		}
		if seen[name] {
			return fmt.Errorf("%w: duplicate benchmark columns %q", ErrMalformedCSV, name)
		}
		seen[name] = true
	}
	return nil
}

// ValidateRawRows: ValidateRows + ValidateRawHeader on rows[0].Header; every
// non-blank cell in a numeric column (raw metric columns 2..7 and every
// benchmark: column) parses via decimal.Parse; the NonNegativeRawColumns
// cells must not be negative. Errors: ErrMalformedCSV.
func ValidateRawRows(rows []Row) error {
	if err := ValidateRows(rows); err != nil {
		return err
	}
	if err := ValidateRawHeader(rows[0].Header); err != nil {
		return err
	}
	modelIdx, reasoningIdx := -1, -1
	for i, name := range rows[0].Header {
		switch name {
		case "model":
			modelIdx = i
		case "reasoning":
			reasoningIdx = i
		}
	}
	for _, row := range rows {
		for i, col := range row.Header {
			cell := row.Values[i]
			if cell == "" {
				continue
			}
			if (i >= 2 && i <= 7) || strings.HasPrefix(col, BenchmarkColumnPrefix) {
				d, err := decimal.Parse(cell)
				if err != nil {
					return fmt.Errorf("%w: non-numeric %s for %s / %s", ErrMalformedCSV, col, row.Values[modelIdx], row.Values[reasoningIdx])
				}
				if NonNegativeRawColumns[col] && d.IsNegative() {
					return fmt.Errorf("%w: negative required metric for %s / %s", ErrMalformedCSV, row.Values[modelIdx], row.Values[reasoningIdx])
				}
			}
		}
	}
	return nil
}

// ResolveBenchmarkColumns expands the dynamic benchmark:<name> column list:
// each group's names in argument order, then direct names, de-duplicated
// keeping first occurrence (pipeline spec §3.3 / model_config.py:70-78).
func ResolveBenchmarkColumns(groupBenchmarks [][]string, direct []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range groupBenchmarks {
		for _, name := range group {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	for _, name := range direct {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
