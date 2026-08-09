package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLegacySkillsRemoved covers F28-T9 test cases 1-5: the legacy prototype
// skill paths are gone, the new authored skills exist, and no stray SKILL.md
// remains outside the three skills/<name>/ dirs.
func TestLegacySkillsRemoved(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot(): %v", err)
	}

	// Cases 1-3: deleted paths do not exist.
	deleted := []string{
		filepath.Join(root, "usage-allowance-checks", "SKILL.md"),
		filepath.Join(root, "usage-allowance-checks", "agents", "openai.yaml"),
		filepath.Join(root, "available-model-data-export", ".agents", "skills"),
	}
	for _, path := range deleted {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still exists (stat err = %v)", path, err)
		}
	}

	// Case 4: each shipped skill's SKILL.md exists.
	for _, name := range Names {
		path := filepath.Join(root, "skills", name, "SKILL.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("%s is empty", path)
		}
	}

	// Case 5: no SKILL.md anywhere directly under skills/ outside the
	// three skills/<name>/ directories (no alias/redirect files).
	skillsRoot := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read skills/: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			if entry.Name() == "SKILL.md" {
				t.Errorf("stray %s at skills/ root", entry.Name())
			}
			continue
		}
		name := entry.Name()
		if !validName(name) {
			t.Errorf("unexpected directory skills/%s", name)
			continue
		}
	}
}
