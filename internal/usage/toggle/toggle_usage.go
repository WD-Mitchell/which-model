//go:build !nousage

// Package toggle resolves the three usage-toggle levels
// (specs/features/F21-usage-toggle/SPEC.md §2.1): --no-usage (L0),
// [usage] enabled (L1), and the compile-time -tags nousage build (L2).
package toggle

import "github.com/WD-Mitchell/which-model/internal/config"

// Reason constants — MUST equal the canonical usage_disabled_reason values
// (specs/global/CONTRACTS.md §6).
const (
	ReasonFlag               = "flag"                 // --no-usage (L0)
	ReasonConfig             = "config"               // [usage] enabled = false (L1)
	ReasonBackendOff         = "backend_off"          // [usage] backend = "off"
	ReasonCompiledOut        = "compiled_out"         // -tags nousage (L2)
	ReasonNoProvidersEnabled = "no_providers_enabled" // [usage] auto/true with zero enabled providers
)

// Compiled reports whether the usage subsystem is linked into this binary.
// true in the default build, false under -tags nousage. Same value for the
// whole process (build-time constant; never toggled at runtime).
const Compiled bool = true // !nousage file; toggle_nousage.go: const Compiled bool = false

// ResolveUsageEnabled resolves the three toggle levels (docs/plan/README.md §6,
// docs/plan/annex-d-cli-reference.md §3.4) with precedence
// compiled_out > flag > config.
//
// Returns (enabled, disabledReason). enabled == false ⇒ reason is one of the
// four constants; enabled == true ⇒ reason == "".
//
// Strict rule (SPEC §2.1 step 4): when the result is
// (false, ReasonNoProvidersEnabled) AND cfg.Usage.Enabled == config.UsageTrue,
// the caller (command layer) MUST exit 2 with a message naming
// "[usage] enabled" — it must NOT degrade.
//
// The stub build (nousage) ignores both arguments and always returns
// (false, ReasonCompiledOut) — L2 cannot be re-enabled at runtime.
func ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string) {
	// L0 (--no-usage) short-circuits before any config dereference, so a
	// nil cfg is safe on this path (SPEC D9).
	if flagNoUsage {
		return false, ReasonFlag
	}
	if cfg.Usage.Enabled == config.UsageFalse {
		return false, ReasonConfig
	}
	if cfg.Usage.Backend == config.UsageBackendOff || cfg.Usage.Backend == "" {
		return false, ReasonBackendOff
	}
	// auto/true (three-state config; only these values are reachable via
	// F01's parser): default-deny — only explicit Enabled: true entries
	// count (SPEC §2.1 step 5).
	for _, p := range cfg.Providers {
		if p.Enabled {
			return true, ""
		}
	}
	return false, ReasonNoProvidersEnabled
}
