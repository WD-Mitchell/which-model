package identity

import (
	"regexp"
	"strings"
)

var (
	effortPattern          = regexp.MustCompile(`^(minimal|low|medium|high|xhigh|max)(?: effort| reasoning)?(?:, (?:context compaction|with tools))?$`)
	reasoningEffortPattern = regexp.MustCompile(`^reasoning effort (none|minimal|low|medium|high|xhigh|max)(?:, (?:context compaction|with tools))?$`)
)

// ParseEffort extracts a reasoning-effort level from a model variant string
// (SPEC.md §2.4, verbatim port of _effort, get_benchmarks.py:42-59).
// ok == false means "no effort annotation" (Python None) — a normal outcome,
// never an error (SPEC.md §3).
func ParseEffort(variant string) (level string, ok bool) {
	normalized := strings.ToLower(variant)
	normalized = strings.NewReplacer("_", " ", "-", " ").Replace(normalized)
	normalized = strings.TrimSpace(normalized)
	m := effortPattern.FindStringSubmatch(normalized)
	if m == nil {
		m = reasoningEffortPattern.FindStringSubmatch(normalized)
	}
	if m == nil {
		return "", false
	}
	level = m[1]
	if level == "none" {
		return "default", true
	}
	return level, true
}
