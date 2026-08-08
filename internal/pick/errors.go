package pick

// RankingError: profile/availability validation or parse failure.
// F22 maps to exit 2 (arguments/config error, global SPEC §5).
type RankingError struct{ Message string }

func (e *RankingError) Error() string { return e.Message }

func (e *RankingError) Unwrap() error { return nil }

// NoCandidatesError: zero candidates survive the tier-1 and availability
// filters. F22 maps to exit 3 (no viable candidate, global SPEC §5).
type NoCandidatesError struct{ Message string }

func (e *NoCandidatesError) Error() string { return e.Message }

func (e *NoCandidatesError) Unwrap() error { return nil }
