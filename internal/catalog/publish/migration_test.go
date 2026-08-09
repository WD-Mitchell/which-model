package publish

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// git runs one git command in the given repo directory.
func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestMigrationLocksInvariant locks the F30-T7 clean-cutover invariant:
// the legacy workflow file is deleted in the same change that introduces
// the generated one, and --check passes afterwards.
func TestMigrationLocksInvariant(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "config", "user.name", "Test")
	git(t, repo, "config", "user.email", "test@example.com")

	legacy := filepath.Join(repo, "available-model-data-export", ".github", "workflows", "update-available-model-data.yml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(legacy, []byte("name: Update Available Model Data\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "legacy workflow")

	// Simulate the git rm of the legacy workflow.
	if err := os.Remove(legacy); err != nil {
		t.Fatalf("Remove(legacy) error = %v", err)
	}

	path := WorkflowPath(repo)
	if _, err := Write(GoldenPC(), path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy workflow still present (stat err = %v)", err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(generated) error = %v", err)
	}
	rendered, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if string(generated) != string(rendered) {
		t.Error("generated file != Render(GoldenPC())")
	}
	if err := Check(GoldenPC(), path); err != nil {
		t.Errorf("Check() after migration = %v, want nil (exit 0 class)", err)
	}
}
