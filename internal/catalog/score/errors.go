// Package score implements catalog scoring: raw CSV derivation (relative
// 0-100 scores, category composites) and scores-CSV parsing for the ranking
// layer (specs/features/F09-scoring). Imports are restricted per
// specs/global/CONTRACTS.md §8 — in particular it MUST NOT import
// internal/catalog/fetch or internal/httpkit (enforced by an import-graph
// test). Pure and deterministic: no network, no filesystem access.
package score

// ErrorCode classifies score-package failures (specs/features/F09-scoring/
// CONTRACTS.md §1.3).
type ErrorCode int

const (
	// ErrInvalidRaw: the raw CSV is invalid (F23 maps to exit 1).
	ErrInvalidRaw ErrorCode = iota
	// ErrInvalidBenchmarkConfig: the benchmarks TOML is invalid (exit 1).
	ErrInvalidBenchmarkConfig
	// ErrInvalidScoresCSV: the scores CSV is invalid (exit 1).
	ErrInvalidScoresCSV
	// ErrUnknownNormalizer: config named an unknown normalizer (exit 2).
	ErrUnknownNormalizer
	// ErrUnknownAggregator: config named an unknown aggregator (exit 2).
	ErrUnknownAggregator
)

// Error is the score package's error type. Message carries the
// Python-verbatim text (specs/features/F09-scoring/CONTRACTS.md §1.3 table).
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return e.Message }

// Unwrap returns nil: these errors are terminal, not wrapping chains.
func (e *Error) Unwrap() error { return nil }
