package csvstore

import (
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvschema"
	"github.com/WD-Mitchell/which-model/internal/security"
)

const (
	BenchmarkColumnPrefix = csvschema.BenchmarkColumnPrefix // pipeline spec §3.1; Python BENCHMARK_COLUMN_PREFIX
	ProvenancePrefix      = csvschema.ProvenancePrefix      // annex-b §6.2a comment-line keyword
	MaxCsvBytes           = 16 << 20                        // 16 MiB; csvstore's own read bound (SPEC §4)
	DefaultBackupKeep     = 5                               // SPEC §4 backup rotation default
)

// Compatibility exports retain the existing csvstore vocabulary. The shared
// definitions live in a leaf package so in-memory scoring does not import I/O.
var RawCoreColumns = csvschema.RawCoreColumns
var CategoryScoreColumns = csvschema.CategoryScoreColumns
var NonNegativeRawColumns = csvschema.NonNegativeRawColumns

// Row is one CSV data row. Header and Values are positionally aligned:
// Values[i] is the cell for column Header[i]. A blank cell is "".
// Authoritative records benchmark names this row's producer explicitly
// re-scoped/cleared this refresh; MergeRows uses it for the clear-vs-fallback
// rule (SPEC §2.6). May be nil.
type Row struct {
	Header        []string
	Values        []string
	Authoritative map[string]bool
}

// Provenance is the parsed `# which-model-scores-provenance …` comment line of
// a scores CSV (SPEC §2.7, annex-b §6.2a). nil means provenance-unknown,
// never stale. Normalizer and Aggregator are "" when the tokens were absent.
type Provenance struct {
	RawSHA256  string // lowercase hex sha256, 64 chars; required token
	Normalizer string // optional `normalizer=` token, verbatim
	Aggregator string // optional `aggregator=` token, verbatim
}

// mapBoundedReadErr maps security.ReadBoundedFile's sanitized errors onto the
// csvstore sentinels (F06-T1 step 8): missing → ErrMissingFile, size
// violation → ErrFileTooLarge, anything else passes through unwrapped.
func mapBoundedReadErr(err error, path string) error {
	var se *security.Error
	if errors.As(err, &se) {
		switch se.Message {
		case "The credential file was not found.":
			return fmt.Errorf("%w: %s", ErrMissingFile, path)
		case "The credential file has an invalid size.":
			return fmt.Errorf("%w: %s", ErrFileTooLarge, path)
		}
	}
	return err
}

// Read reads the CSV at path (bounded by MaxCsvBytes via
// security.ReadBoundedFile). A single leading `#` comment line, if present,
// must start with ProvenancePrefix and parse as whitespace-separated
// key=value tokens: raw_sha256 required (64 lowercase hex), normalizer and
// aggregator optional verbatim strings, unknown keys skipped; any other shape
// or a second leading `#` line is ErrMalformedCSV. No comment line →
// provenance == nil. All data rows share header; each row's Header is that
// header. Cell text is verbatim.
// Errors: ErrMissingFile, ErrFileTooLarge, ErrMalformedCSV.
func Read(path string) (rows []Row, provenance *Provenance, err error) {
	content, _, err := security.ReadBoundedFile(path, MaxCsvBytes)
	if err != nil {
		return nil, nil, mapBoundedReadErr(err, path)
	}
	if !utf8.Valid(content) {
		return nil, nil, fmt.Errorf("%w: %s is not valid UTF-8", ErrMalformedCSV, path)
	}
	text := string(content)

	// Provenance handling (SPEC.md §2.7).
	if strings.HasPrefix(text, "#") {
		line := text
		rest := ""
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			line, rest = text[:idx], text[idx+1:]
		}
		if !strings.HasPrefix(line, ProvenancePrefix) {
			return nil, nil, fmt.Errorf("%w: bad provenance line in %s", ErrMalformedCSV, path)
		}
		prov := &Provenance{}
		for _, token := range strings.Fields(line[len(ProvenancePrefix):]) {
			kv := strings.SplitN(token, "=", 2)
			if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
				return nil, nil, fmt.Errorf("%w: bad provenance token %q in %s", ErrMalformedCSV, token, path)
			}
			key, value := kv[0], kv[1]
			switch key {
			case "raw_sha256":
				// F06 CONTRACTS: 64 LOWERCASE hex — the same rule the
				// writer enforces (issue #46: Read used to accept and
				// silently lowercase uppercase hex that WriteAtomic then
				// rejected, so a readable CSV was unwritable).
				if len(value) != 64 || value != strings.ToLower(value) {
					return nil, nil, fmt.Errorf("%w: bad raw_sha256 in %s", ErrMalformedCSV, path)
				}
				if _, err := hex.DecodeString(value); err != nil {
					return nil, nil, fmt.Errorf("%w: bad raw_sha256 in %s", ErrMalformedCSV, path)
				}
				prov.RawSHA256 = value
			case "normalizer":
				prov.Normalizer = value
			case "aggregator":
				prov.Aggregator = value
			default:
				// Unknown tokens are ignored (SPEC.md §4).
			}
		}
		if prov.RawSHA256 == "" {
			return nil, nil, fmt.Errorf("%w: provenance line missing raw_sha256 in %s", ErrMalformedCSV, path)
		}
		if strings.HasPrefix(rest, "#") {
			return nil, nil, fmt.Errorf("%w: second comment line in %s", ErrMalformedCSV, path)
		}
		provenance = prov
		text = rest
	}

	r := csv.NewReader(strings.NewReader(text))
	header, err := r.Read()
	if err != nil || len(header) == 0 {
		return nil, nil, fmt.Errorf("%w: no header row in %s", ErrMalformedCSV, path)
	}
	r.FieldsPerRecord = len(header)
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %s row: %v", ErrMalformedCSV, path, err)
		}
		rows = append(rows, Row{Header: header, Values: record})
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("%w: no data rows in %s", ErrMalformedCSV, path)
	}
	return rows, provenance, nil
}
