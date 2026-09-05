set -euo pipefail
# Require CI to register before watching: an empty/partial rollup is not a pass.
expected_head=$(git rev-parse HEAD)
for attempt in $(seq 1 60); do
  actual_head=$(gh pr view "$HEAD_BRANCH" --json headRefOid --jq .headRefOid)
  [ "$actual_head" = "$expected_head" ] || { echo 'PR head changed; refusing to merge'; exit 1; }
  checks=$(gh pr view "$HEAD_BRANCH" --json statusCheckRollup --jq .statusCheckRollup)
  if jq -e 'any(.[]; .name == "test") and any(.[]; (.name // .context) == "CodeQL")' <<< "$checks" >/dev/null; then
    break
  fi
  [ "$attempt" -lt 60 ] || { echo 'CI or CodeQL did not register'; exit 1; }
  sleep 10
done
gh pr checks "$HEAD_BRANCH" --watch --fail-fast --interval 10
# A second read rejects skipped/cancelled/neutral checks and changed heads.
checks=$(gh pr checks "$HEAD_BRANCH" --json name,state,bucket)
jq -e 'length > 0 and any(.[]; .name == "test") and all(.[]; .bucket == "pass")' <<< "$checks" >/dev/null
actual_head=$(gh pr view "$HEAD_BRANCH" --json headRefOid --jq .headRefOid)
[ "$actual_head" = "$expected_head" ] || { echo 'PR head changed; refusing to merge'; exit 1; }
gh pr merge "$HEAD_BRANCH" "--$MERGE_METHOD" --match-head-commit "$expected_head"
state=$(gh pr view "$HEAD_BRANCH" --json state --jq .state)
[ "$state" = MERGED ] || { echo 'Merge has not completed'; exit 1; }
