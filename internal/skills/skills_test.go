package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestNames covers F28-T3 test 1.
func TestNames(t *testing.T) {
	want := []string{"model-selection", "provider-usage", "usage-aware-dispatch"}
	if !reflect.DeepEqual(Names, want) {
		t.Fatalf("Names = %v, want %v", Names, want)
	}
}

// TestTargets covers F28-T3 test 5.
func TestTargets(t *testing.T) {
	if TargetGeneric != "generic" {
		t.Errorf("TargetGeneric = %q, want %q", TargetGeneric, "generic")
	}
	if TargetClaude != "claude" {
		t.Errorf("TargetClaude = %q, want %q", TargetClaude, "claude")
	}
}

// TestRepoRoot covers F28-T3 tests 2-4: .git upward walk, --repo override,
// and the no-repo error naming --repo.
func TestRepoRoot(t *testing.T) {
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(origCwd)
		SetRepoDir("")
	})
	SetRepoDir("")

	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, err = filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}

	// Test 2: cwd inside the repo -> RepoRoot() == repo.
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() from repo dir: %v", err)
	}
	if got != repo {
		t.Errorf("RepoRoot() = %q, want %q", got, repo)
	}
	// Test 2b: worktree checkout with .git file -> RepoRoot() == worktree.
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /fake/path\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree, err = filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(worktree); err != nil {
		t.Fatal(err)
	}
	gotWorktree, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() from worktree dir: %v", err)
	}
	if gotWorktree != worktree {
		t.Errorf("RepoRoot() worktree = %q, want %q", gotWorktree, worktree)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	// Test 3: cwd in a nested subdir -> upward walk still finds repo.
	sub := filepath.Join(repo, "nonexistent-subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	got, err = RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() from subdir: %v", err)
	}
	if got != repo {
		t.Errorf("RepoRoot() from subdir = %q, want %q", got, repo)
	}

	// Test 4a: cwd with no .git ancestor -> error naming --repo.
	nowhere := t.TempDir()
	if err := os.Chdir(nowhere); err != nil {
		t.Fatal(err)
	}
	_, err = RepoRoot()
	if err == nil {
		t.Fatal("RepoRoot() outside a repo: nil error, want error")
	}
	if !strings.Contains(err.Error(), "--repo") {
		t.Errorf("RepoRoot() error %q does not mention --repo", err)
	}

	// Test 4b: SetRepoDir wins even from a .git-less cwd.
	override := t.TempDir()
	SetRepoDir(override)
	got, err = RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() with SetRepoDir: %v", err)
	}
	if got != override {
		t.Errorf("RepoRoot() with override = %q, want %q", got, override)
	}
}

// fakeRepo builds a fake repo (with .git/ and a full skills/ tree of fixed
// byte content) and points SetRepoDir at it. Returns the root and the source
// bytes keyed by "<name>/<rel>".
func fakeRepo(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := map[string][]byte{}
	for _, name := range Names {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		md := []byte("# skill " + name + "\n")
		yaml := []byte("interface:\n  display_name: \"" + name + "\"\n")
		source[name+"/SKILL.md"] = md
		source[name+"/agents/openai.yaml"] = yaml
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), md, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "agents", "openai.yaml"), yaml, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { SetRepoDir("") })
	SetRepoDir(root)
	return root, source
}

// TestInstall covers F28-T7 test cases 1-6: install, idempotence, unknown
// names, --user validation, and overwrite protection.
func TestInstall(t *testing.T) {
	root, source := fakeRepo(t)
	installed := func(name string) (string, string) {
		t.Helper()
		dir := filepath.Join(root, ".agents", "skills", name)
		md, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			t.Fatalf("installed SKILL.md: %v", err)
		}
		yaml, err := os.ReadFile(filepath.Join(dir, "agents", "openai.yaml"))
		if err != nil {
			t.Fatalf("installed agents/openai.yaml: %v", err)
		}
		return string(md), string(yaml)
	}

	// Case 1: fresh install copies both files byte-exactly.
	msg, err := Install("model-selection", TargetGeneric, false, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	wantMsg := "installed model-selection to " + filepath.Join(root, ".agents", "skills", "model-selection")
	if msg != wantMsg {
		t.Errorf("message = %q, want %q", msg, wantMsg)
	}
	md, yaml := installed("model-selection")
	if md != string(source["model-selection/SKILL.md"]) || yaml != string(source["model-selection/agents/openai.yaml"]) {
		t.Errorf("installed bytes differ from source")
	}

	// Case 2: idempotent re-install.
	if _, err := Install("model-selection", TargetGeneric, false, false); err != nil {
		t.Errorf("re-Install: %v", err)
	}
	if md2, yaml2 := installed("model-selection"); md2 != md || yaml2 != yaml {
		t.Errorf("re-Install changed files")
	}

	// Case 3: unknown skill name.
	if _, err := Install("nope", TargetGeneric, false, false); err == nil || !strings.Contains(err.Error(), "unknown skill") {
		t.Errorf("Install(\"nope\") error = %v, want containing %q", err, "unknown skill")
	}

	// Case 4: --user with the generic target.
	if _, err := Install("model-selection", TargetGeneric, true, false); err == nil || !strings.Contains(err.Error(), "--user is only supported with --target claude") {
		t.Errorf("Install --user generic error = %v", err)
	}

	// Case 5: modified destination, no force -> refused, file unchanged.
	dst := filepath.Join(root, ".agents", "skills", "model-selection", "SKILL.md")
	if err := os.WriteFile(dst, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("model-selection", TargetGeneric, false, false); err == nil || !strings.Contains(err.Error(), "refusing to overwrite modified file") {
		t.Errorf("Install over modified file error = %v, want refusal", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "user edit\n" {
		t.Errorf("refused install modified the file: %q", b)
	}

	// Case 6: --force overwrites.
	if _, err := Install("model-selection", TargetGeneric, false, true); err != nil {
		t.Errorf("Install --force: %v", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != string(source["model-selection/SKILL.md"]) {
		t.Errorf("--force did not restore source bytes: %q", b)
	}
}

// TestRemove covers F28-T7 test cases 7-10: removal, no-op, protection.
func TestRemove(t *testing.T) {
	root, source := fakeRepo(t)
	dir := filepath.Join(root, ".agents", "skills", "model-selection")
	if _, err := Install("model-selection", TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}

	// Case 7: remove deletes files and now-empty dirs.
	msg, err := Remove("model-selection", TargetGeneric, false, false)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !strings.Contains(msg, "removed model-selection") {
		t.Errorf("message = %q, want containing %q", msg, "removed model-selection")
	}
	for _, rel := range []string{"SKILL.md", "agents", filepath.Join("agents", "openai.yaml")} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("after remove, %s still exists (stat err = %v)", rel, err)
		}
	}

	// Case 8: remove again is a no-op success.
	msg, err = Remove("model-selection", TargetGeneric, false, false)
	if err != nil {
		t.Fatalf("second Remove: %v", err)
	}
	if !strings.Contains(msg, "not installed (nothing to remove)") {
		t.Errorf("second Remove message = %q", msg)
	}

	// Case 9: modified installed file, no force -> refused, file kept.
	if _, err := Install("model-selection", TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(dst, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove("model-selection", TargetGeneric, false, false); err == nil || !strings.Contains(err.Error(), "refusing to delete modified file") {
		t.Errorf("Remove over modified file error = %v, want refusal", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "user edit\n" {
		t.Errorf("refused remove modified the file: %q", b)
	}

	// Case 10: --force deletes regardless.
	if _, err := Remove("model-selection", TargetGeneric, false, true); err != nil {
		t.Errorf("Remove --force: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("--force remove left the file (stat err = %v)", err)
	}
	_ = source
}

// TestList covers F28-T7 test case 11: installed names, in Names order.
func TestList(t *testing.T) {
	root, _ := fakeRepo(t)
	if _, err := Install("provider-usage", TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("model-selection", TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := List(TargetGeneric, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"model-selection", "provider-usage"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
	_ = root
}

// TestInstallUserTarget covers F28-T7 test case 12: --target claude --user
// installs under $HOME/.claude/skills.
func TestInstallUserTarget(t *testing.T) {
	root, source := fakeRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := Install("model-selection", TargetClaude, true, false); err != nil {
		t.Fatalf("Install claude --user: %v", err)
	}
	dir := filepath.Join(home, ".claude", "skills", "model-selection")
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("user install SKILL.md: %v", err)
	}
	if string(b) != string(source["model-selection/SKILL.md"]) {
		t.Errorf("user install bytes differ from source")
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "openai.yaml")); err != nil {
		t.Errorf("user install agents/openai.yaml: %v", err)
	}
	_ = root
}
