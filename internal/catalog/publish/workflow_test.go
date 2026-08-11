package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GoldenPC is the exact config the golden file was rendered from.
func GoldenPC() *PublishConfig {
	return &PublishConfig{
		Enabled:       true,
		Schedule:      "0 6 * * *",
		Timezone:      "Europe/London",
		Environment:   "CSV Update",
		Branches:      []string{"main", "release"},
		Mode:          "pull-request",
		AutoMerge:     true,
		MergeMethod:   "squash",
		CommitMessage: "chore(data): refresh available model scores",
		PRTitle:       "chore(data): refresh available model scores",
		PRLabels:      []string{"data", "automated"},
		RawCSVPath:    "available-model-data-export/available_model_raw_values.csv",
	}
}

func TestRenderGolden(t *testing.T) {
	got, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want, err := os.ReadFile("testdata/refresh-model-data.golden.yml")
	if err != nil {
		t.Fatalf("ReadFile(golden) error = %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Render() != golden file\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderDeterministic(t *testing.T) {
	a, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	b, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if string(a) != string(b) {
		t.Error("two Render calls differ")
	}
}

func TestRenderTrailingBytes(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		t.Errorf("output must end with exactly one \\n")
	}
	if strings.HasSuffix(string(out), "\n\n") {
		t.Error("output ends with two newlines")
	}
	if strings.Contains(string(out), "\r") {
		t.Error("output contains CR bytes")
	}
}

func TestRenderSingleBranch(t *testing.T) {
	pc := GoldenPC()
	pc.Branches = []string{"main"}
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(out), `branch: ["main"] # from [catalog.publish].branches, listed order`) {
		t.Errorf("matrix line missing: %s", out)
	}
	if !strings.Contains(string(out), "  group: refresh-model-data\n") {
		t.Error("concurrency group missing")
	}
	if !strings.Contains(string(out), "  cancel-in-progress: false\n") {
		t.Error("cancel-in-progress missing")
	}
}

func TestRenderEnvironmentOptional(t *testing.T) {
	pc := GoldenPC()
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(out), "    environment: \"CSV Update\"\n") {
		t.Errorf("configured environment missing: %s", out)
	}
	pc.Environment = ""
	out, err = Render(pc)
	if err != nil {
		t.Fatalf("Render() without environment error = %v", err)
	}
	if strings.Contains(string(out), "\n    environment:") {
		t.Errorf("empty environment rendered: %s", out)
	}
}

func TestRenderDirectPush(t *testing.T) {
	pc := GoldenPC()
	pc.Mode = "direct-push"
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(out), "run: git push origin HEAD:${{ matrix.branch }}") {
		t.Errorf("push step missing: %s", out)
	}
	for _, forbidden := range []string{"gh pr", "pull-requests: write", "GH_TOKEN"} {
		if strings.Contains(string(out), forbidden) {
			t.Errorf("direct-push output contains %q", forbidden)
		}
	}
}

func TestRenderUsesStandaloneRawRefresh(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "python3 scripts/refresh-model-data.py") {
		t.Errorf("standalone refresh step missing: %s", out)
	}
	for _, forbidden := range []string{"setup-go", "go build", "go test", "./which-model", "available_model_scores.csv"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("workflow contains app-dependent content %q", forbidden)
		}
	}
}

func TestRenderAutoMergeFalse(t *testing.T) {
	pc := GoldenPC()
	pc.AutoMerge = false
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(out), "gh pr create") {
		t.Errorf("gh pr create missing: %s", out)
	}
	if strings.Contains(string(out), "gh pr merge") {
		t.Errorf("gh pr merge present with auto_merge=false: %s", out)
	}
}

func TestRenderPRTitleWithColonIsValidYAMLScalar(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := "        run: |\n          head_branch=\"refresh-model-data-${{ github.run_id }}-${{ strategy.job-index }}\""
	if !strings.Contains(string(out), want) {
		t.Errorf("PR create command must define a unique pushed head branch: %s", out)
	}
	for _, command := range []string{
		`git push origin "HEAD:refs/heads/${head_branch}"`,
		`gh pr create --base "${{ matrix.branch }}" --head "${head_branch}"`,
		`--body "Automated catalog refresh."`,
	} {
		if !strings.Contains(string(out), command) {
			t.Errorf("PR create command missing %q: %s", command, out)
		}
	}
}

func TestRenderDisabled(t *testing.T) {
	pc := GoldenPC()
	pc.Enabled = false
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if out != nil {
		t.Errorf("Render() = %q, want nil for disabled", out)
	}
}

func TestRenderNoUsageNoExtraSecrets(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	for _, forbidden := range []string{"usage refresh", "--refresh-usage", "usage list"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("output contains usage command %q", forbidden)
		}
	}
	if got := strings.Count(s, "secrets.ARTIFICIAL_ANALYSIS_API"); got != 1 {
		t.Errorf("secrets.ARTIFICIAL_ANALYSIS_API count = %d, want 1", got)
	}
	if got := strings.Count(s, "secrets.CSV_UPDATE_TOKEN"); got != 3 {
		t.Errorf("secrets.CSV_UPDATE_TOKEN count = %d, want 3", got)
	}
	if strings.Contains(s, "secrets.GITHUB_TOKEN") {
		t.Error("workflow must use github.token rather than the secrets alias")
	}
	if got := strings.Count(s, "github.token"); got != 4 {
		t.Errorf("github.token count = %d, want 4", got)
	}
	if !strings.Contains(s, `gh pr review --approve "refresh-model-data-${{ github.run_id }}-${{ strategy.job-index }}"`) {
		t.Error("admin-token path must approve the PAT-authored PR before enabling auto-merge")
	}
}

func TestRenderPins(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "actions/checkout@"+CheckoutPin+" # v6.0.2") {
		t.Errorf("checkout pin missing: %s", s)
	}
}

func TestRenderLabels(t *testing.T) {
	pc := GoldenPC()
	pc.PRLabels = []string{"a", "b"}
	out, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if strings.Count(s, "--label") != 2 {
		t.Errorf("--label count = %d, want 2: %s", strings.Count(s, "--label"), s)
	}
	ai := strings.Index(s, "--label a")
	bi := strings.Index(s, "--label b")
	if ai < 0 || bi < 0 || ai > bi {
		t.Errorf("label order wrong: %s", s)
	}

	pc.PRLabels = nil
	out, err = Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "--label") {
		t.Errorf("--label present with empty pr_labels: %s", out)
	}
}

func TestRepoRoot(t *testing.T) {
	t.Run("finds .git ancestor", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("MkdirAll(.git) error = %v", err)
		}
		t.Chdir(dir)
		root, err := RepoRoot()
		if err != nil {
			t.Fatalf("RepoRoot() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			t.Errorf("RepoRoot() = %q, no .git inside: %v", root, err)
		}
		if filepath.Base(root) != filepath.Base(dir) {
			t.Errorf("RepoRoot() = %q, want base %q", root, filepath.Base(dir))
		}
	})
	t.Run("no .git ancestor", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := RepoRoot(); err == nil {
			t.Fatal("RepoRoot() = nil error, want error")
		}
	})
}

func TestWorkflowPath(t *testing.T) {
	got := WorkflowPath("/repo")
	if got != filepath.Join("/repo", ".github", "workflows", "refresh-model-data.yml") {
		t.Errorf("WorkflowPath() = %q", got)
	}
}
