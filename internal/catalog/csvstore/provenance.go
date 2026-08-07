package csvstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// ProvenanceHash returns the lowercase hex sha256 of path's exact on-disk
// bytes (bounded read). Order-sensitive by design (SPEC §2.7).
// Errors: ErrMissingFile, ErrFileTooLarge.
func ProvenanceHash(path string) (string, error) {
	content, _, err := security.ReadBoundedFile(path, MaxCsvBytes)
	if err != nil {
		return "", mapBoundedReadErr(err, path)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// StaleCheck reports whether the scores CSV at scoresPath was derived from a
// raw CSV different from the current bytes at rawPath. Provenance-unknown
// scores (no comment line) → (false, nil). Missing file on either side → error
// (ErrMissingFile). Never a hard error for callers beyond that: a stale result
// is a warning, not a failure (SPEC §2.7).
func StaleCheck(scoresPath, rawPath string) (stale bool, err error) {
	_, prov, err := Read(scoresPath)
	if err != nil {
		return false, err
	}
	if prov == nil {
		return false, nil
	}
	rawHash, err := ProvenanceHash(rawPath)
	if err != nil {
		return false, err
	}
	return rawHash != prov.RawSHA256, nil
}

// StaleWarning returns the exact single-line warning callers emit when
// StaleCheck reports stale; it names both artifact paths and instructs the
// operator to run --refresh-scores.
func StaleWarning(scoresPath, rawPath string) string {
	return fmt.Sprintf("stale scores CSV %s: its recorded raw CSV hash does not match the current %s; run --refresh-scores to regenerate", scoresPath, rawPath)
}
