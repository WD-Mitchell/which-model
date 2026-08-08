// Package fetch holds the collector layer for catalog data sources: models.dev
// (providers + benchmarks), the Artificial Analysis v2 API, and the AA model
// page scraper. Collectors produce catalog types (internal/catalog/identity)
// and raw-CSV values; they never touch credentials beyond the AA API key.
//
// Import boundaries (specs/global/CONTRACTS.md §8): this package and its
// subpackages must not import internal/config, internal/usage,
// internal/routing, internal/pick, internal/catalog/csvstore, or
// internal/catalog/score.
package fetch

import "errors"

// Error is the canonical collector failure. Code is a global Failure.Code
// value (specs/global/CONTRACTS.md §1.6) plus the F08-owned
// "missing_api_key"; Err carries the underlying cause (typically an
// *httpkit.Error).
type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// MissingAPIKeyError reports that the AA API key was found nowhere: the
// ARTIFICIAL_ANALYSIS_API environment variable is blank and the repo-root
// .env has no usable entry. F23 maps this code to exit 2 (missing
// configuration).
func MissingAPIKeyError() *Error {
	return &Error{
		Code: "missing_api_key",
		Err:  errors.New("missing ARTIFICIAL_ANALYSIS_API environment variable or .env entry"),
	}
}
