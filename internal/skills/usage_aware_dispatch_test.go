package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestUsageAwareDispatchSkill covers every F28-T6 test case: the authored
// skills/usage-aware-dispatch files exist with all required content.
func TestUsageAwareDispatchSkill(t *testing.T) {
	const skill = "skills/usage-aware-dispatch"
	md := readRepoFile(t, filepath.Join(skill, "SKILL.md")) // case 1
	yaml := readRepoFile(t, filepath.Join(skill, "agents", "openai.yaml"))

	substrings := map[string][]string{
		"frontmatter": {"name: usage-aware-dispatch", "description:"},
		"strategies": {
			"--strategy priority",
			"--strategy least-used",
			"--strategy most-used",
			"--strategy closest-to-reset",
		},
		"explain command": {"which-model explain <candidate-id> --json"},
		"defer rule":      {"usage_enabled", "defer", "score-only"},
		"gating":          {"band_gated", "gate_above_used_percent"},
		"evidence fields": {"evidence.band.{name,used_percent,weight}", "evidence.snapshot_age_seconds", "evidence.confidence"},
		"exit table":      {"| 0 |", "| 1 |", "| 2 |", "| 3 |", "| 4 |", "| 5 |", "do NOT dispatch to a gated provider"},
		"checklist":       {"## Checklist"},
	}
	for name, wants := range substrings {
		for _, want := range wants {
			if !strings.Contains(md, want) {
				t.Errorf("SKILL.md missing %q (%s)", want, name)
			}
		}
	}

	for _, want := range []string{"display_name:", "default_prompt:"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("agents/openai.yaml missing %q", want)
		}
	}
}
