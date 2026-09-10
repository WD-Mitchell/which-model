package publish

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedMergeGate(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq unavailable")
	}
	for _, scenario := range []string{"pass", "pending", "failed", "missing", "changed", "changed-after-checks", "skipped", "read-error", "merge-rejected", "queued"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			write("git", "#!/bin/sh\necho tested-head\n")
			write("sleep", "#!/bin/sh\nexit 0\n")
			write("gh", `#!/bin/bash
set -eu
case "$*" in
 *headRefOid*)
  if [ "$SCENARIO" = changed ] || { [ "$SCENARIO" = changed-after-checks ] && [ -f "$EVIDENCE/watched" ]; }; then echo newer-head; else echo tested-head; fi;;
 *statusCheckRollup*)
  [ "$SCENARIO" != read-error ] || exit 1
  if [ "$SCENARIO" = missing ]; then echo '[]'; else echo '[{"name":"test","status":"IN_PROGRESS"},{"name":"CodeQL","status":"COMPLETED"}]'; fi;;
 *--watch*)
  touch "$EVIDENCE/watched"
  [ "$SCENARIO" != failed ] && [ "$SCENARIO" != pending ];;
 'pr checks '*)
  if [ "$SCENARIO" = skipped ]; then echo '[{"name":"test","bucket":"skipping"}]'; else echo '[{"name":"test","bucket":"pass"},{"name":"other","bucket":"pass"}]'; fi;;
 'pr merge '*)
  [ "$SCENARIO" != merge-rejected ] || exit 1
  printf '%s\n' "$*" > "$EVIDENCE/merged";;
 *'--json state'*) if [ "$SCENARIO" = queued ]; then echo OPEN; else echo MERGED; fi;;
 *) echo "unexpected gh invocation: $*" >&2; exit 2;;
esac
`)
			cmd := exec.Command("bash", "-c", mergePRScript)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "SCENARIO="+scenario, "EVIDENCE="+dir, "HEAD_BRANCH=refresh-test", "MERGE_METHOD=squash")
			out, err := cmd.CombinedOutput()
			if (err == nil) != (scenario == "pass") {
				t.Fatalf("scenario %s: err=%v output=%s", scenario, err, out)
			}
			merged, _ := os.ReadFile(filepath.Join(dir, "merged"))
			if scenario == "pass" || scenario == "queued" {
				if !strings.Contains(string(merged), "--match-head-commit tested-head") {
					t.Fatalf("missing commit guard: %s", merged)
				}
			} else if len(merged) > 0 {
				t.Fatalf("unsafe merge: %s", merged)
			}
		})
	}
}

func TestCreatePRSeparatesPublishingAndMetadataTokens(t *testing.T) {
	dir := t.TempDir()
	stub := `#!/bin/bash
set -eu
printf '%s %s\n' "$GH_TOKEN" "$*" >> "$EVIDENCE/calls"
case "$*" in
 'api user '*) echo human;;
 'issue create '*) [ "$GH_TOKEN" = metadata ]; echo https://github.com/owner/repo/issues/42;;
 'issue view '*) echo human;;
 'pr create '*) [ "$GH_TOKEN" = publish ]; echo https://github.com/owner/repo/pull/43;;
 'pr edit '*) [ "$GH_TOKEN" = metadata ];;
 *closingIssuesReferences*) echo 42;;
 *'--json state'*) echo CLOSED;;
 *'--json number'*) echo 43;;
 'api --method GET repos/'*) echo '[]';;
 'pr view '*) echo human;;
 *) exit 2;;
esac
`
	for name, body := range map[string]string{"gh": stub, "git": "#!/bin/sh\nexit 0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", "-c", createPRScript)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "EVIDENCE="+dir, "GH_TOKEN=publish", "METADATA_TOKEN=metadata", "HEAD_BRANCH=refresh-test", "BASE_BRANCH=main", "PR_TITLE=refresh")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create PR: %v\n%s", err, out)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	for _, want := range []string{"metadata issue create", "publish pr create", "metadata pr edit refresh-test --add-assignee human"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("missing %q in %s", want, calls)
		}
	}
}

func TestCreatePRClosesSupersededRefreshes(t *testing.T) {
	for _, scenario := range []string{"pass", "create-failed", "verification-failed", "list-failed", "close-failed"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			stub := `#!/bin/bash
set -eu
case "$*" in
 'api user '*) echo human;;
 'issue create '*) echo https://github.com/owner/repo/issues/42;;
 'issue view '*) echo human;;
 'pr create '*) [ "$SCENARIO" != create-failed ]; echo https://github.com/owner/repo/pull/43;;
 'pr edit '*) :;;
 *closingIssuesReferences*) [ "$SCENARIO" != verification-failed ]; echo 42;;
 *'--json state'*) echo CLOSED;;
 *'--json number'*) echo 43;;
 'pr view '*) echo human;;
 'api --method GET repos/'*)
  [ "$SCENARIO" != list-failed ] || exit 1
  [[ "$*" == *'--paginate'* && "$*" == *'base=main'* && "$*" == *'state=open'* ]] || exit 2
  cat "$EVIDENCE/prs.json";;
 'pr close '*)
  [ "$SCENARIO" != close-failed ] || exit 1
  echo "$3" >> "$EVIDENCE/closed";;
 *) echo "unexpected: $*" >&2; exit 2;;
esac
`
			// Include another page, a fork, another base, a human branch, and newer PRs.
			prs := `[
{"number":10,"state":"open","head":{"ref":"refresh-model-data-100-0","repo":{"full_name":"owner/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}},
{"number":11,"state":"open","head":{"ref":"refresh-model-data-101-0","repo":{"full_name":"fork/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}},
{"number":12,"state":"open","head":{"ref":"refresh-model-data-102-0","repo":{"full_name":"owner/repo"}},"base":{"ref":"release","repo":{"full_name":"owner/repo"}}},
{"number":13,"state":"open","head":{"ref":"refresh-model-data-manual","repo":{"full_name":"owner/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}},
{"number":43,"state":"open","head":{"ref":"refresh-model-data-143-0","repo":{"full_name":"owner/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}},
{"number":44,"state":"open","head":{"ref":"refresh-model-data-144-0","repo":{"full_name":"owner/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}}
]
[{"number":9,"state":"open","head":{"ref":"refresh-model-data-99-0","repo":{"full_name":"owner/repo"}},"base":{"ref":"main","repo":{"full_name":"owner/repo"}}}]
`
			for name, body := range map[string]string{"gh": stub, "git": "#!/bin/sh\nexit 0\n", "prs.json": prs} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-c", createPRScript)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "EVIDENCE="+dir, "SCENARIO="+scenario, "GH_TOKEN=publish", "METADATA_TOKEN=metadata", "HEAD_BRANCH=refresh-model-data-143-0", "BASE_BRANCH=main", "PR_TITLE=refresh")
			out, err := cmd.CombinedOutput()
			if (err == nil) != (scenario == "pass") {
				t.Fatalf("err=%v output=%s", err, out)
			}
			closed, _ := os.ReadFile(filepath.Join(dir, "closed"))
			want := ""
			if scenario == "pass" {
				want = "10\n9\n"
			}
			if string(closed) != want {
				t.Fatalf("closed=%q want=%q", closed, want)
			}
		})
	}
}
