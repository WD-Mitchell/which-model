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
