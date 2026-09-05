package publish

import (
	"fmt"
	"os"
	"os/exec"
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
		RawCSVPath:    "data/available_model_raw_values.csv",
		ScoresCSVPath: "data/available_model_scores.csv",
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
	if !strings.Contains(s, "python3 .daily-update/refresh-model-data.py") {
		t.Errorf("standalone refresh step missing: %s", out)
	}
	for _, forbidden := range []string{"setup-go", "go build", "go test", "./which-model"} {
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
	for _, command := range []string{
		`HEAD_BRANCH: refresh-model-data-${{ github.run_id }}-${{ strategy.job-index }}`,
		`git push origin "HEAD:refs/heads/${HEAD_BRANCH}"`,
		`gh pr create --base "$BASE_BRANCH" --head "$HEAD_BRANCH"`,
		`--body-file "$work_dir/pr.md"`,
		`PR_TITLE: "chore(data): refresh available model scores"`,
	} {
		if !strings.Contains(string(out), command) {
			t.Errorf("missing %q", command)
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
	for _, required := range []string{"GH_TOKEN: ${{ secrets.CSV_UPDATE_TOKEN }}", "gh pr checks", "--match-head-commit", "gh issue create", "--assignee", "closingIssuesReferences"} {
		if !strings.Contains(s, required) {
			t.Errorf("missing %q", required)
		}
	}
	for _, forbidden := range []string{"gh pr review", "--admin", "gh pr merge --auto", "secrets.GITHUB_TOKEN"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("unexpected %q", forbidden)
		}
	}
}

func TestRenderPins(t *testing.T) {
	out, err := Render(GoldenPC())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "actions/checkout@"+CheckoutPin+" # v7.0.1") {
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
	ai := strings.Index(s, "--label 'a'")
	bi := strings.Index(s, "--label 'b'")
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

func TestRenderPairedArtifactLifecycle(t *testing.T) {
	pc := GoldenPC()
	pc.RawCSVPath = "data/custom raw's $(touch SHOULD_NOT_EXIST).csv"
	pc.ScoresCSVPath = "data/custom scores.csv"
	out, err := Render(pc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	var previous int
	for _, command := range []string{"python3 .daily-update/refresh-model-data.py --output " + shellQuote(pc.RawCSVPath), "python3 .daily-update/generate_scores.py --input " + shellQuote(pc.RawCSVPath) + " --output " + shellQuote(pc.ScoresCSVPath), "python3 -m unittest discover -s .daily-update/tests -v", "git add -- " + shellQuote(pc.RawCSVPath) + " " + shellQuote(pc.ScoresCSVPath)} {
		at := strings.Index(text, command)
		if at < previous || at < 0 {
			t.Fatalf("missing/out-of-order command %q in %s", command, text)
		}
		previous = at
	}
}

// Run the generated collection/derivation/staging commands with inert tools.
// A generator failure must stop before tests and any Git operation, and unusual
// configured filenames must reach the tools as literal individual arguments.
func TestGeneratedWorkflowStopsBeforeStagingOnGeneratorFailure(t *testing.T) {
	pc := GoldenPC()
	pc.RawCSVPath = "raw's $(touch SHOULD_NOT_EXIST).csv"
	pc.ScoresCSVPath = "scores file.csv"
	data, err := Render(pc)
	if err != nil {
		t.Fatal(err)
	}
	prefix := strings.Split(string(data), "      - if: steps.changes.outputs.changed")[0]
	var commands []string
	for _, line := range strings.Split(prefix, "\n") {
		if strings.HasPrefix(line, "          python3 ") || strings.HasPrefix(line, "          git ") {
			commands = append(commands, strings.TrimPrefix(line, "          "))
		}
	}
	for _, fail := range []bool{true, false} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			for _, name := range []string{"python3", "git"} {
				body := "#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\" >> \"$TEST_LOG\"\n"
				if name == "python3" && fail {
					body += "case \"$1\" in *.daily-update/generate_scores.py) exit 42;; esac\n"
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-e", "-c", strings.Join(commands, "\n"))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "TEST_LOG="+log, "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
			out, err := cmd.CombinedOutput()
			if fail && err == nil || !fail && err != nil {
				t.Fatalf("fail=%v: %v %s", fail, err, out)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(calls), dir+"/git") == fail {
				t.Fatalf("staging after failure=%v: %s", fail, calls)
			}
			if !strings.Contains(string(calls), pc.RawCSVPath+"\n") || !strings.Contains(string(calls), pc.ScoresCSVPath+"\n") {
				t.Fatalf("paths were split or expanded: %s", calls)
			}
			if _, err := os.Stat(filepath.Join(dir, "SHOULD_NOT_EXIST")); !os.IsNotExist(err) {
				t.Fatalf("shell expansion occurred: %v", err)
			}
		})
	}
}

func TestWorkflowVerificationUsesConfiguredPair(t *testing.T) {
	for _, paths := range [][2]string{{"custom/raw.csv", "data/available_model_scores.csv"}, {"data/available_model_raw_values.csv", "custom/scores.csv"}} {
		pc := NewDefaults()
		pc.RawCSVPath, pc.ScoresCSVPath = paths[0], paths[1]
		data, err := Render(pc)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			if !strings.Contains(string(data), path) {
				t.Fatalf("missing %s", path)
			}
		}
		if !strings.Contains(string(data), "WHICH_MODEL_TEST_RAW_CSV: "+fmt.Sprintf("%q", paths[0])) || !strings.Contains(string(data), "WHICH_MODEL_TEST_SCORES_CSV: "+fmt.Sprintf("%q", paths[1])) {
			t.Fatal("verification does not select generated pair")
		}
	}
}
