package csvstore

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// renderContent renders rows (and the optional provenance line) to CSV bytes
// (SPEC §2.3, annex-b §6.2a). Go's csv.Writer emits \n terminators only.
func renderContent(rows []Row, provenance *Provenance) ([]byte, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: no data rows", ErrMalformedCSV)
	}
	header := rows[0].Header
	if len(header) == 0 {
		return nil, fmt.Errorf("%w: empty header", ErrMalformedCSV)
	}
	for _, row := range rows {
		if len(row.Header) != len(header) || len(row.Values) != len(header) {
			return nil, fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
		}
		for i := range header {
			if row.Header[i] != header[i] {
				return nil, fmt.Errorf("%w: inconsistent row headers", ErrMalformedCSV)
			}
		}
	}

	var buf bytes.Buffer
	if provenance != nil {
		hash := provenance.RawSHA256
		if len(hash) != 64 || hex.EncodeToString(decodeMust(hash)) != hash || strings.ToLower(hash) != hash {
			return nil, fmt.Errorf("%w: bad provenance raw_sha256", ErrMalformedCSV)
		}
		buf.WriteString(ProvenancePrefix + " raw_sha256=" + hash)
		if provenance.Normalizer != "" {
			buf.WriteString(" normalizer=" + provenance.Normalizer)
		}
		if provenance.Aggregator != "" {
			buf.WriteString(" aggregator=" + provenance.Aggregator)
		}
		buf.WriteString("\n")
	}

	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCSV, err)
	}
	for _, row := range rows {
		if err := w.Write(row.Values); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedCSV, err)
		}
	}
	w.Flush()
	if w.Error() != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCSV, w.Error())
	}
	return buf.Bytes(), nil
}

// decodeMust decodes a 64-char hex string; callers validate length first.
// Returns an empty slice on malformed input so the re-encode check fails.
func decodeMust(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

// atomicReplace is the step-1..step-5 replace primitive shared by WriteAtomic
// and WriteAtomicBytes (SPEC §2.3): read original, write temp in the same
// directory, fsync, verify the target is unchanged, rename over it. Any
// failure removes the temp file and leaves the target untouched.
func atomicReplace(path string, content []byte) error {
	original, _, err := security.ReadBoundedFile(path, MaxCsvBytes)
	if err != nil {
		return mapBoundedReadErr(err, path)
	}

	dir, base := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".")
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			os.Remove(tmpName) // ignore "not exist"
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	reread, _, err := security.ReadBoundedFile(path, MaxCsvBytes)
	if err != nil {
		return mapBoundedReadErr(err, path)
	}
	if !bytes.Equal(reread, original) {
		return fmt.Errorf("%w: %s", ErrChangedDuringWrite, path)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	renamed = true
	return nil
}

// WriteAtomic atomically replaces path with the rendered rows. If provenance
// is non-nil its comment line is written as the first line before the header:
// "# which-model-scores-provenance raw_sha256=<hex>" plus " normalizer=<n>" /
// " aggregator=<a>" when those fields are non-empty; RawSHA256 must be 64
// lowercase hex (ErrMalformedCSV). On any error the temp file is removed and
// path is untouched. Errors: ErrMissingFile, ErrMalformedCSV,
// ErrChangedDuringWrite.
func WriteAtomic(path string, rows []Row, provenance *Provenance) error {
	content, err := renderContent(rows, provenance)
	if err != nil {
		return err
	}
	return atomicReplace(path, content)
}

// WriteAtomicBytes is WriteAtomic with opaque content: no provenance parsing,
// no header rendering, no validation of content. Callers (F09 scores Derive
// via F23) supply the complete bytes including the §6.2a provenance line.
// Errors: ErrMissingFile, ErrChangedDuringWrite.
func WriteAtomicBytes(path string, content []byte) error {
	return atomicReplace(path, content)
}
