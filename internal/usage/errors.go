package usage

import "errors"

// ErrUsageCompiledOut is the sentinel returned by every nousage stub entry
// point (specs/features/F21-usage-toggle/SPEC.md §2.2 step 8, D4). It lives
// in a tag-free file so BOTH builds carry the symbol and callers can compare
// with errors.Is from either build (annex-a §1a.2).
var ErrUsageCompiledOut = errors.New("usage subsystem compiled out (-tags nousage)")
