package score

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	sdecimal "github.com/shopspring/decimal"
	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
)

// tier1ScoreColumns are the six Tier-1 score column names (the same
// constants F10 uses).
var tier1ScoreColumns = []string{
	"intelligence_index_score",
	"time_per_intelligence_index_task_seconds_score",
	"cost_per_intelligence_index_task_usd_score",
	"median_end_to_end_response_time_seconds_score",
	"artificial_analysis_coding_index_score",
	"artificial_analysis_agentic_index_score",
}

func scoresError(format string, args ...any) error {
	return &Error{Code: ErrInvalidScoresCSV, Message: fmt.Sprintf(format, args...)}
}

// ParseScoresCSV parses a Derive-produced scores CSV into rows for F10.
// Rejects duplicate identities HERE (rank_models.py:359-401). A leading
// provenance line is validated per F06 rules (ProvenancePrefix shape,
// raw_sha256 64-hex; second '#' line or malformed token -> ErrInvalidScoresCSV)
// but not exposed. Empty input -> ErrInvalidScoresCSV.
func ParseScoresCSV(data []byte) ([]catalog.ScoreRow, error) {
	text, err := stripScoresProvenance(data)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader([]byte(text)))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err == io.EOF {
		return nil, scoresError("score CSV contains no model rows")
	}
	if err != nil {
		return nil, scoresError("%v", err)
	}

	required := map[string]bool{"model": true, "reasoning": true}
	for _, column := range tier1ScoreColumns {
		required[column] = true
	}
	present := make(map[string]bool, len(header))
	for _, name := range header {
		present[name] = true
	}
	var missing []string
	for name := range required {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, scoresError("score CSV is missing required columns: %s", strings.Join(missing, ", "))
	}

	columnIndex := make(map[string]int, len(header))
	for i, name := range header {
		columnIndex[name] = i
	}
	cell := func(fields []string, column string) string {
		idx, ok := columnIndex[column]
		if !ok || idx >= len(fields) {
			return ""
		}
		return strings.TrimSpace(fields[idx])
	}

	var rows []catalog.ScoreRow
	seen := make(map[[2]string]bool)
	for rowNumber := 2; ; rowNumber++ {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, scoresError("score CSV row %d: %v", rowNumber, err)
		}
		if len(fields) > len(header) {
			return nil, scoresError("score CSV row %d has extra cells", rowNumber)
		}
		model := identity.CleanModelName(cell(fields, "model"))
		reasoning := identity.CollapseReasoning(cell(fields, "reasoning"))
		if model == "" || reasoning == "" {
			return nil, scoresError("score CSV row %d has a blank model/reasoning identity", rowNumber)
		}
		key := [2]string{model, reasoning}
		if seen[key] {
			return nil, scoresError("score CSV has duplicate identity: %s / %s", model, reasoning)
		}
		seen[key] = true

		row := catalog.ScoreRow{
			Model:      model,
			Reasoning:  reasoning,
			Tier1:      make(map[string]sdecimal.Decimal),
			Categories: make(map[string]sdecimal.Decimal),
			Benchmarks: make(map[string]sdecimal.Decimal),
		}
		for _, column := range header {
			if !strings.HasSuffix(column, "_score") {
				continue
			}
			raw := cell(fields, column)
			if raw == "" {
				continue
			}
			value, err := wdecimal.Parse(raw)
			if err != nil {
				return nil, scoresError("score CSV row %d %s must be numeric", rowNumber, column)
			}
			if value.IsNegative() || value.GreaterThan(hundred) {
				return nil, scoresError("score CSV row %d %s must be between 0 and 100", rowNumber, column)
			}
			switch {
			case strings.HasPrefix(column, csvstore.BenchmarkColumnPrefix):
				name := strings.TrimSuffix(strings.TrimPrefix(column, csvstore.BenchmarkColumnPrefix), "_score")
				row.Benchmarks[name] = value
			case required[column]:
				row.Tier1[column] = value
			default:
				row.Categories[strings.TrimSuffix(column, "_score")] = value
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, scoresError("score CSV contains no model rows")
	}
	return rows, nil
}

// stripScoresProvenance validates and removes a leading provenance line per
// the F06 shape rules (csvstore.Read): the line must start with
// ProvenancePrefix, tokens are key=value with raw_sha256 required as 64
// lowercase hex, unknown tokens are skipped, and a second leading '#' line is
// an error. A file without a leading '#' passes through unchanged.
func stripScoresProvenance(data []byte) (string, error) {
	if len(data) == 0 || data[0] != '#' {
		return string(data), nil
	}
	line := string(data)
	rest := ""
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		line, rest = string(data[:idx]), string(data[idx+1:])
	}
	if !strings.HasPrefix(line, csvstore.ProvenancePrefix) {
		return "", scoresError("bad provenance line")
	}
	var rawSHA256 string
	for _, token := range strings.Fields(line[len(csvstore.ProvenancePrefix):]) {
		kv := strings.SplitN(token, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return "", scoresError("bad provenance token %q", token)
		}
		switch kv[0] {
		case "raw_sha256":
			if len(kv[1]) != 64 {
				return "", scoresError("bad raw_sha256")
			}
			if _, err := hex.DecodeString(kv[1]); err != nil {
				return "", scoresError("bad raw_sha256")
			}
			rawSHA256 = strings.ToLower(kv[1])
		case "normalizer", "aggregator":
			// Tolerated but not exposed (CONTRACTS §2.2).
		default:
			// Unknown tokens are skipped (F06 rule).
		}
	}
	if rawSHA256 == "" {
		return "", scoresError("provenance line missing raw_sha256")
	}
	if strings.HasPrefix(rest, "#") {
		return "", scoresError("second comment line")
	}
	return rest, nil
}
