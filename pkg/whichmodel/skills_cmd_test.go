package whichmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSkillsRepo builds a repo with .git/ and a full skills/ tree, and
// returns the --repo flag value for it.
func fakeSkillsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model-selection", "provider-usage", "usage-aware-dispatch"} {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "agents", "openai.yaml"), []byte("interface:\n  display_name: \""+name+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func skillsArgs(repo string, extra ...string) []string {
	args := []string{"skills", "--repo", repo}
	return append(args, extra...)
}

// TestSkillsCmdInstall covers F28-T8 test cases 4-8.
func TestSkillsCmdInstall(t *testing.T) {
	repo := fakeSkillsRepo(t)

	// Case 4: no args installs all three; stdout mentions model-selection.
	code, stdout, stderr := captureExecuteFresh(t, skillsArgs(repo, "install"))
	if code != 0 {
		t.Fatalf("skills install: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "installed model-selection") {
		t.Errorf("stdout = %q, want it to contain %q", stdout, "installed model-selection")
	}
	for _, name := range []string{"model-selection", "provider-usage", "usage-aware-dispatch"} {
		md := filepath.Join(repo, ".agents", "skills", name, "SKILL.md")
		if _, err := os.Stat(md); err != nil {
			t.Errorf("installed %s missing: %v", md, err)
		}
	}

	// Case 5: second install is idempotent (exit 0, bytes unchanged).
	before, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "model-selection", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr = captureExecuteFresh(t, skillsArgs(repo, "install", "model-selection"))
	if code != 0 {
		t.Fatalf("second install: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	after, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "model-selection", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("re-install changed installed bytes")
	}

	// Case 6: unknown name → exit 2.
	code, _, stderr = captureExecuteFresh(t, skillsArgs(repo, "install", "nope"))
	if code != 2 {
		t.Errorf("skills install nope: exit = %d, want 2 (stderr: %s)", code, stderr)
	}

	// Case 7: bad target value → exit 2.
	code, _, stderr = captureExecuteFresh(t, append(skillsArgs(repo, "install"), "--target", "nonsense"))
	if code != 2 {
		t.Errorf("skills install --target nonsense: exit = %d, want 2 (stderr: %s)", code, stderr)
	}

	// Case 8: --user with generic default → exit 2.
	code, _, stderr = captureExecuteFresh(t, skillsArgs(repo, "install", "--user"))
	if code != 2 {
		t.Errorf("skills install --user: exit = %d, want 2 (stderr: %s)", code, stderr)
	}
}

// TestSkillsCmdRemoveList covers F28-T8 test cases 9-11.
func TestSkillsCmdRemoveList(t *testing.T) {
	repo := fakeSkillsRepo(t)

	code, _, stderr := captureExecuteFresh(t, skillsArgs(repo, "install"))
	if code != 0 {
		t.Fatalf("setup install: exit = %d (stderr: %s)", code, stderr)
	}

	// Case 9: remove one; list shows the remaining two.
	code, stdout, stderr := captureExecuteFresh(t, skillsArgs(repo, "remove", "model-selection"))
	if code != 0 {
		t.Fatalf("skills remove: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "removed model-selection") {
		t.Errorf("remove stdout = %q, want it to contain %q", stdout, "removed model-selection")
	}
	code, stdout, stderr = captureExecuteFresh(t, skillsArgs(repo, "list"))
	if code != 0 {
		t.Fatalf("skills list: exit = %d (stderr: %s)", code, stderr)
	}
	for _, name := range []string{"provider-usage", "usage-aware-dispatch"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("list stdout = %q, missing %q", stdout, name)
		}
	}
	if strings.Contains(stdout, "model-selection") {
		t.Errorf("list stdout = %q, still contains removed skill", stdout)
	}

	// Case 10: --json is a valid object with "installed" in Names order.
	code, stdout, stderr = captureExecuteFresh(t, skillsArgs(repo, "list", "--json"))
	if code != 0 {
		t.Fatalf("skills list --json: exit = %d (stderr: %s)", code, stderr)
	}
	var doc struct {
		Target    string   `json:"target"`
		User      bool     `json:"user"`
		Installed []string `json:"installed"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("list --json is not valid JSON: %v\noutput: %q", err, stdout)
	}
	want := []string{"provider-usage", "usage-aware-dispatch"}
	if strings.Join(doc.Installed, ",") != strings.Join(want, ",") {
		t.Errorf("installed = %v, want %v", doc.Installed, want)
	}

	// Case 11: remove of an unknown name → exit 2.
	code, _, stderr = captureExecuteFresh(t, skillsArgs(repo, "remove", "nope"))
	if code != 2 {
		t.Errorf("skills remove nope: exit = %d, want 2 (stderr: %s)", code, stderr)
	}
}
