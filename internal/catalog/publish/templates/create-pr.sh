set -euo pipefail
[ -n "$GH_TOKEN" ] || { echo 'CSV_UPDATE_TOKEN is required for catalog publication'; exit 1; }
uploader=$(gh api user --jq .login)
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT
cat > "$work_dir/issue.md" <<'BODY'
## Goal
Publish the refreshed model catalog and matching generated scores.

## Scope
The configured raw and score CSV artifacts.

## Engineering Summary
None — existing refresh and score generators are reused.

## Acceptance Criteria
Both artifacts regenerate consistently and every PR check passes before merge.

## Prerequisites
Configured catalog data-source and publishing credentials.

## Required Evidence
Refresh workflow Python validation and linked PR checks.

## Dependencies
None.

## BDD Scenarios
Given updated upstream models and benchmarks, when refresh completes, then the catalog and scores are published together after passing checks.
BODY
issue_url=$(gh issue create --title "$PR_TITLE" --body-file "$work_dir/issue.md" --type Task --assignee "$uploader")
issue_number=${issue_url##*/}
gh issue view "$issue_number" --json assignees --jq '.assignees[].login' | grep -Fx "$uploader"
cat > "$work_dir/pr.md" <<'BODY'
## Summary
Refresh available model values and regenerate the matching scores from current upstream data.

## How this fits together
```mermaid
flowchart LR
  A[Upstream model and benchmark data] --> B[Raw catalog and generated scores]
  B --> C[Validation and PR checks]
  C --> D[Published catalog]
```

## Affected behaviour
Catalog consumers receive the refreshed raw values and corresponding scores together.

## Verification
The refresh completed, score generation succeeded, and `python3 -m unittest discover -s .daily-update/tests -v` passed against the configured artifact pair. PR checks must pass before merging.

## Dependencies and stack position
BODY
printf '\nBase: `%s`. Independent automated catalog refresh.\n\nFixes #%s\n' "$BASE_BRANCH" "$issue_number" >> "$work_dir/pr.md"
cat >> "$work_dir/pr.md" <<'BODY'

## Review notes
Upstream catalog values can change ranking results. Regeneration checks validate raw/score consistency; failed or unavailable checks prevent automatic merge.

## Required delivery checks
- [x] The linked issue is Type Task.
- [x] Catalog regeneration and Python validation passed.
- [x] The Mermaid diagram describes the real data flow.
- [x] The closing reference associates the issue through Development.
- [x] The authenticated publishing human is the assignee.
BODY
git push origin "HEAD:refs/heads/${HEAD_BRANCH}"
# Configured labels are passed as separate, shell-quoted arguments by the renderer.
gh pr create --base "$BASE_BRANCH" --head "$HEAD_BRANCH" --title "$PR_TITLE" --body-file "$work_dir/pr.md" --assignee "$uploader" "$@"
gh pr view "$HEAD_BRANCH" --json assignees --jq '.assignees[].login' | grep -Fx "$uploader"
gh pr view "$HEAD_BRANCH" --json closingIssuesReferences --jq '.closingIssuesReferences[].number' | grep -Fx "$issue_number"
