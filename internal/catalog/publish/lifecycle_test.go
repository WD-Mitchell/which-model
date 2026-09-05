package publish

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnabledFalseLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-model-data.yml")

	pc := singleBranchPC()
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write(enabled) error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file absent after enabled write: %v", err)
	}

	pc.Enabled = false
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write(disabled) error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after disabled write (stat err = %v)", err)
	}
	if err := Check(pc, path); err != nil {
		t.Errorf("Check(disabled, absent) = %v, want nil", err)
	}
}

func TestEnabledFalseStaleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-model-data.yml")
	if err := os.WriteFile(path, []byte("arbitrary stale bytes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	pc := GoldenPC()
	pc.Enabled = false
	var de *DriftError
	if err := Check(pc, path); !errors.As(err, &de) {
		t.Fatalf("Check(disabled, stale) = %T, want *DriftError", err)
	}
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write(disabled) error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stale file not removed by disabled write (stat err = %v)", err)
	}
}

func TestMultiBranchListedOrder(t *testing.T) {
	pc := GoldenPC()
	pc.Branches = []string{"release", "main", "canary"}
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := `branch: ["release", "main", "canary"] # from [catalog.publish].branches, listed order`
	if !strings.Contains(string(out), want) {
		t.Errorf("matrix line = missing %q in:\n%s", want, out)
	}
}

func TestMultiBranchIsolationStructure(t *testing.T) {
	pc := GoldenPC()
	pc.Branches = []string{"main", "release", "canary"}
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if got := strings.Count(s, "fail-fast: false"); got != 1 {
		t.Errorf("fail-fast: false count = %d, want 1", got)
	}
	if !strings.Contains(s, `BASE_BRANCH: ${{ matrix.branch }}`) {
		t.Error("pr create is not branch-scoped")
	}
	for _, step := range []string{
		`commit -m "chore(data): refresh available model scores"`,
		"gh pr create --base",
		"gh pr merge",
	} {
		if strings.Count(s, step) != 1 {
			t.Errorf("publish step %q count != 1 (one commit per branch, no cross-branch step)", step)
		}
	}
}

func TestOutcomeReportStep(t *testing.T) {
	pc := GoldenPC()
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "- name: Report per-branch outcome\n        if: always()") {
		t.Errorf("outcome step missing or missing if: always():\n%s", s)
	}
	for _, vocab := range []string{"merged", "skipped-no-changes", "failed"} {
		if !strings.Contains(s, vocab) {
			t.Errorf("outcome vocabulary %q missing", vocab)
		}
	}
	if strings.Contains(s, `echo "refresh branch ${{ matrix.branch }}: published"`) {
		t.Error("pull-request mode must not report a deferred auto-merge as published")
	}
	if !strings.Contains(s, `steps.merge.outcome`) {
		t.Error("pull-request outcome must depend on the merge-request step")
	}
	trimmed := strings.TrimRight(s, "\n")
	if !strings.HasSuffix(trimmed, "fi") {
		t.Errorf("outcome step is not last: ...%q", trimmed[len(trimmed)-80:])
	}
	reportIdx := strings.Index(s, "- name: Report per-branch outcome")
	if reportIdx < 0 || strings.Index(s[reportIdx+len("- name: Report per-branch outcome"):], "gh pr") >= 0 {
		t.Error("a publish step follows the outcome report step")
	}
}

func TestSecretPlacement(t *testing.T) {
	pc := GoldenPC()
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if got := strings.Count(s, "secrets.ARTIFICIAL_ANALYSIS_API"); got != 1 {
		t.Fatalf("secrets.ARTIFICIAL_ANALYSIS_API count = %d, want 1", got)
	}
	envBlock := "- run: |\n          python3 .daily-update/refresh-model-data.py --output 'data/available_model_raw_values.csv'\n        env:\n          ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}"
	if !strings.Contains(s, envBlock) {
		t.Errorf("secret not on the catalog refresh step env:\n%s", s)
	}
}

func TestDirectPushMultiBranch(t *testing.T) {
	pc := GoldenPC()
	pc.Mode = "direct-push"
	pc.Branches = []string{"main", "release"}
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if got := strings.Count(s, "run: git push origin HEAD:${{ matrix.branch }}"); got != 1 {
		t.Errorf("push step count = %d, want exactly 1", got)
	}
	for _, forbidden := range []string{"gh pr", "GH_TOKEN"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("direct-push output contains %q", forbidden)
		}
	}
	if got := strings.Count(s, "- name: Report per-branch outcome"); got != 1 {
		t.Errorf("outcome step count = %d, want 1", got)
	}
}

func TestDirectPushDeterminism(t *testing.T) {
	pc := GoldenPC()
	pc.Mode = "direct-push"
	a, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	b, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if string(a) != string(b) {
		t.Error("two direct-push renders differ")
	}
}
