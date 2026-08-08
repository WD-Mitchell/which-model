//go:build nousage

package toggle

import "github.com/WD-Mitchell/which-model/internal/config"

// Reason constants — MUST equal the canonical usage_disabled_reason values
// (specs/global/CONTRACTS.md §6). Same values as the !nousage file; they are
// build-independent.
const (
	ReasonFlag               = "flag"                 // --no-usage (L0)
	ReasonConfig             = "config"               // [usage] enabled = false (L1)
	ReasonCompiledOut        = "compiled_out"         // -tags nousage (L2)
	ReasonNoProvidersEnabled = "no_providers_enabled" // [usage] auto/true with zero enabled providers
)

// Compiled reports whether the usage subsystem is linked into this binary.
// true in the default build, false under -tags nousage. Same value for the
// whole process (build-time constant; never toggled at runtime).
const Compiled bool = false // nousage file; toggle_usage.go: const Compiled bool = true

// ResolveUsageEnabled in the compiled-out build ignores both arguments and
// always returns (false, ReasonCompiledOut) — L2 cannot be re-enabled at
// runtime by any flag, env var, or config value (SPEC §2.2 step 10).
func ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string) {
	return false, ReasonCompiledOut
}
