// Package hooks owns the four agent-dispatch lifecycle hooks
// (specs/features/F29-agent-hooks/SPEC.md): a session-start usage cache
// warm, a quota-guard advisory, a pre-dispatch model resolution gate, and a
// post-dispatch evidence recorder (docs/plan/annex-c-agent-integration.md
// §3.1–§3.4). It imports only the Go standard library
// (specs/global/CONTRACTS.md §8).
package hooks

import (
	"os"
)

// Hook is one lifecycle hook (annex-c §3.1–§3.4). Timeout is seconds.
type Hook struct {
	ID      string
	Event   string
	Matcher string
	Timeout int
	// Underlying builds the annexed command argv: defaults first, then
	// passthrough args (later args win in cobra). env overrides
	// os.Getenv for WHICH_MODEL_TASK_PROFILE / WHICH_MODEL_CANDIDATE_ID.
	Underlying func(passthrough []string, env map[string]string) []string
}

// All is the four-hook registry, in annex-c §3 order.
var All = []Hook{
	{
		ID: "usage-refresh", Event: "SessionStart", Matcher: "*", Timeout: 5,
		Underlying: func(p []string, _ map[string]string) []string {
			return append([]string{"usage", "--all", "--json", "--quiet", "--refresh-usage", "--timeout", "5s"}, p...)
		},
	},
	{
		ID: "quota-guard", Event: "SessionStart", Matcher: "*", Timeout: 5,
		Underlying: func(p []string, _ map[string]string) []string {
			return append([]string{"usage", "--all", "--json", "--band-at-or-above", "critical", "--quiet"}, p...)
		},
	},
	{
		ID: "spawn-gate", Event: "PreToolUse", Matcher: "Task", Timeout: 8,
		Underlying: func(p []string, env map[string]string) []string {
			profile := envOr(env, "WHICH_MODEL_TASK_PROFILE", "balanced_implementation")
			return append([]string{"pick", "--profile", profile, "--strategy", "priority", "--json"}, p...)
		},
	},
	{
		ID: "model-audit", Event: "PostToolUse", Matcher: "Task", Timeout: 5,
		Underlying: func(p []string, env map[string]string) []string {
			if id := envOr(env, "WHICH_MODEL_CANDIDATE_ID", ""); id != "" {
				return append([]string{"explain", id, "--json"}, p...)
			}
			return append([]string{"explain", "--last", "--json"}, p...)
		},
	},
}

func envOr(env map[string]string, key, def string) string {
	if v, ok := env[key]; ok {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Get returns the hook with the given id.
func Get(name string) (Hook, bool) {
	for _, h := range All {
		if h.ID == name {
			return h, true
		}
	}
	return Hook{}, false
}
