package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readRepoFile resolves the repo root and reads a file under it.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot(): %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if len(b) == 0 {
		t.Fatalf("%s is empty", rel)
	}
	return string(b)
}

// TestModelSelectionSkill covers every F28-T4 test case: the authored
// skills/model-selection files exist with all required content.
func TestModelSelectionSkill(t *testing.T) {
	const skill = "skills/model-selection"
	md := readRepoFile(t, filepath.Join(skill, "SKILL.md")) // case 1
	yaml := readRepoFile(t, filepath.Join(skill, "agents", "openai.yaml"))

	substrings := map[string][]string{
		"frontmatter":            {"name: model-selection", "description:"},
		"rank command":           {"which-model pick --profile balanced_implementation --top 5 --json"},
		"availability + explain": {"--available .tmp/live-model-efforts.txt", "which-model explain <candidate-id> --json"},
		"usage_enabled first":    {"usage_enabled"},
		"output fields":          {"candidates[0].candidate_id", "excluded_candidates[].reason_code", "not_in_availability_list"},
		"exit table":             {"| 0 |", "| 1 |", "| 2 |", "| 3 |", "| 4 |", "| 5 |", "Failure.message"},
		"evidence + checklist":   {"availability proof", "## Checklist"},
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
