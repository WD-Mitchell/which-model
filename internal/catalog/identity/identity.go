package identity

// EffortOrder ranks the reasoning-effort ladder (SPEC §2.6); derived from the
// ParseEffort regexes' ladder (get_benchmarks.py:42-59).
var EffortOrder = map[string]int{
	"minimal": 0,
	"low":     1,
	"medium":  2,
	"high":    3,
	"xhigh":   4,
	"max":     5,
}

// ReasoningLevels is the set of valid reasoning levels including "default"
// (SPEC §2.6). Consumers use it for validation (F09) and explain output (F10).
var ReasoningLevels = map[string]struct{}{
	"default": {},
	"minimal": {},
	"low":     {},
	"medium":  {},
	"high":    {},
	"xhigh":   {},
	"max":     {},
}

// Identity is the canonical catalog row identity: a cleaned model name plus a
// collapsed reasoning level. It is a comparable struct and a legal Go map key
// (SPEC §2.3). csvstore's merge key is this pair after collapse.
type Identity struct {
	Model     string
	Reasoning string
}

// CollapseReasoning maps the "default" reasoning level to "high" and passes
// every other value through unchanged (SPEC.md §2.2, verbatim port of
// _normalise_reasoning_level, get_aa_api_values.py:231-233).
func CollapseReasoning(level string) string {
	if level == "default" {
		return "high"
	}
	return level
}

// IdentityKey returns the canonical identity for a model/reasoning pair:
// cleaned name plus collapsed level (SPEC.md §2.3).
func IdentityKey(model, reasoning string) Identity {
	return Identity{Model: CleanModelName(model), Reasoning: CollapseReasoning(reasoning)}
}
