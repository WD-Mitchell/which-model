package csvstore

import (
	"fmt"
	"strings"

	sdecimal "github.com/shopspring/decimal"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// collapseReasoning maps "default" → "high" (pipeline spec §4.2).
func collapseReasoning(level string) string {
	if level == "default" {
		return "high"
	}
	return level
}

// columnIndex returns the index of name in header, or -1.
func columnIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

// cellByIndex returns "" when idx < 0 or out of range, else row.Values[idx].
func cellByIndex(row Row, idx int) string {
	if idx < 0 || idx >= len(row.Values) {
		return ""
	}
	return row.Values[idx]
}

// valuesByName maps Header[i] → Values[i] for all i.
func valuesByName(row Row) map[string]string {
	out := make(map[string]string, len(row.Header))
	for i, name := range row.Header {
		if i < len(row.Values) {
			out[name] = row.Values[i]
		}
	}
	return out
}

// identityOf returns the collapsed (model, reasoning) identity of a row.
// Missing or blank identity cells → ErrMalformedCSV.
func identityOf(row Row) ([2]string, error) {
	model := cellByIndex(row, columnIndex(row.Header, "model"))
	reasoning := cellByIndex(row, columnIndex(row.Header, "reasoning"))
	if model == "" || reasoning == "" {
		return [2]string{}, fmt.Errorf("%w: blank identity", ErrMalformedCSV)
	}
	return [2]string{model, collapseReasoning(reasoning)}, nil
}

// CollapseRows ports _collapse_default_reasoning minus name cleaning
// (SPEC §2.5): groups rows by (model, collapseReasoning(reasoning)) in
// first-seen order; the output row is the first non-"default" member (else the
// first member) with, per non-identity non-benchmark column, its own value if
// non-blank else the first non-blank member value; per benchmark column the
// max of the members' non-blank values (decimal comparison via decimal.Parse);
// Authoritative is the union. Blank model cell → ErrMalformedCSV.
func CollapseRows(rows []Row) ([]Row, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	header := rows[0].Header
	for _, row := range rows {
		if len(row.Header) != len(header) {
			return nil, fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
		}
		for i := range header {
			if row.Header[i] != header[i] {
				return nil, fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
			}
		}
	}

	reasoningIdx := columnIndex(header, "reasoning")
	grouped := map[[2]string][]Row{}
	var order [][2]string
	for _, row := range rows {
		id, err := identityOf(row)
		if err != nil {
			return nil, err
		}
		if _, ok := grouped[id]; !ok {
			order = append(order, id)
		}
		grouped[id] = append(grouped[id], row)
	}

	out := make([]Row, 0, len(order))
	for _, id := range order {
		members := grouped[id]
		base := members[0]
		for _, m := range members {
			if cellByIndex(m, reasoningIdx) != "default" {
				base = m
				break
			}
		}
		values := append([]string(nil), base.Values...)

		for i, name := range header {
			if name == "model" || name == "reasoning" {
				continue
			}
			if strings.HasPrefix(name, BenchmarkColumnPrefix) {
				best := ""
				var bestVal sdecimal.Decimal
				for _, m := range members {
					cell := cellByIndex(m, i)
					if cell == "" {
						continue
					}
					d, err := wdecimal.Parse(cell)
					if err != nil {
						return nil, fmt.Errorf("%w: non-numeric benchmark cell", ErrMalformedCSV)
					}
					if best == "" || d.GreaterThan(bestVal) {
						best, bestVal = cell, d
					}
				}
				values[i] = best
			} else if values[i] == "" {
				for _, m := range members {
					if cell := cellByIndex(m, i); cell != "" {
						values[i] = cell
						break
					}
				}
			}
		}

		auth := map[string]bool{}
		for _, m := range members {
			for k, v := range m.Authoritative {
				auth[k] = v
			}
		}
		out = append(out, Row{Header: header, Values: values, Authoritative: auth})
	}
	return out, nil
}

// MergeRows ports merge_rows (SPEC §2.6): CollapseRows on both inputs; index
// existing by identity (model, collapseReasoning(reasoning)); for each fresh
// row, non-identity non-benchmark columns take fresh if non-blank else
// existing; benchmark cells take fresh if non-blank, else existing when the
// fresh cell is blank and the name is not in fresh's Authoritative set, else
// blank. Fresh-only identities appended as-is; existing-only identities
// dropped. Output rows carry the fresh dataset's header.
func MergeRows(existing, fresh []Row) ([]Row, error) {
	var err error
	existing, err = CollapseRows(existing)
	if err != nil {
		return nil, err
	}
	fresh, err = CollapseRows(fresh)
	if err != nil {
		return nil, err
	}

	byID := map[[2]string]Row{}
	for _, row := range existing {
		id, err := identityOf(row)
		if err != nil {
			return nil, err
		}
		byID[id] = row
	}

	var out []Row
	for _, f := range fresh {
		id, err := identityOf(f)
		if err != nil {
			return nil, err
		}
		auth := map[string]bool{}
		for k, v := range f.Authoritative {
			auth[k] = v
		}
		row := Row{Header: f.Header, Values: append([]string(nil), f.Values...), Authoritative: auth}
		if cur, ok := byID[id]; ok {
			curVals := valuesByName(cur)
			for i, name := range f.Header {
				if name == "model" || name == "reasoning" {
					// Matched rows keep the existing row's identity cells
					// (the store's canonical spelling; SPEC §2.6 case 5).
					row.Values[i] = curVals[name]
					continue
				}
				if strings.HasPrefix(name, BenchmarkColumnPrefix) {
					if row.Values[i] == "" && !auth[name] {
						row.Values[i] = curVals[name]
					}
				} else if row.Values[i] == "" {
					row.Values[i] = curVals[name]
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// MergePartialRefresh ports merge_partial_refresh: MergeRows, then, only when
// preserveUnselected is true, appends every collapsed existing row whose model
// is not in refreshedModels, re-mapped onto the fresh header by column name
// (names absent from the fresh header become blank).
func MergePartialRefresh(existing, fresh []Row, refreshedModels []string, preserveUnselected bool) ([]Row, error) {
	if len(fresh) == 0 {
		return MergeRows(existing, fresh)
	}
	collapsed, err := CollapseRows(existing)
	if err != nil {
		return nil, err
	}
	merged, err := MergeRows(existing, fresh)
	if err != nil {
		return nil, err
	}
	if !preserveUnselected {
		return merged, nil
	}

	names := map[string]bool{}
	for _, m := range refreshedModels {
		names[m] = true
	}
	header := fresh[0].Header
	for _, row := range collapsed {
		model := cellByIndex(row, columnIndex(row.Header, "model"))
		if names[model] {
			continue
		}
		byName := valuesByName(row)
		values := make([]string, len(header))
		for i, name := range header {
			values[i] = byName[name]
		}
		merged = append(merged, Row{Header: header, Values: values, Authoritative: row.Authoritative})
	}
	return merged, nil
}
