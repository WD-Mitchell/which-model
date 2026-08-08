package pick

import "github.com/shopspring/decimal"

// UsageState is the resolution result the pick flow consumes and echoes
// (SPEC §2.4 step 14). Constructed by the command layer from
// toggle.ResolveUsageEnabled.
type UsageState struct {
	Enabled        bool   // resolved usage_enabled
	DisabledReason string // usage_disabled_reason; "" when Enabled
}

// DegradedCandidates rewrites candidates for usage-disabled picks: every
// candidate gets Band = "" and BandWeight = 1.0 (so FinalScore == ModelScore);
// all other fields are copied unchanged; the input slice is not modified.
// [bands] and gate_above_used_percent are inert by construction (SPEC §2.4
// step 13). UsageState with Enabled == true is not this function's concern —
// callers apply it only on the degraded path.
func DegradedCandidates(candidates []Candidate) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		c.Band = ""
		c.BandWeight = decimal.NewFromFloat(1.0)
		c.FinalScore = c.ModelScore
		out = append(out, c)
	}
	return out
}
