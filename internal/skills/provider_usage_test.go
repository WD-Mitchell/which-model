package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestProviderUsageSkill covers every F28-T5 test case: the authored
// skills/provider-usage files exist with all required content.
func TestProviderUsageSkill(t *testing.T) {
	const skill = "skills/provider-usage"
	md := readRepoFile(t, filepath.Join(skill, "SKILL.md")) // case 1
	yaml := readRepoFile(t, filepath.Join(skill, "agents", "openai.yaml"))

	substrings := map[string][]string{
		"frontmatter":     {"name: provider-usage", "description:"},
		"usage command":   {"which-model usage claude --json"},
		"flags":           {"--trust-configured-origin https://trusted.example", "--login", "--all"},
		"usage_enabled":   {"which-model config show --json", "usage_enabled"},
		"output fields":   {"windows[].used_percent", "snapshots", "confidence"},
		"exit table":      {"| 0 |", "| 1 |", "| 2 |", "| 3 |", "| 4 |", "| 5 |", "usage_disabled", "usage_compiled_out"},
		"evidence + list": {"sanitized", "## Checklist"},
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
