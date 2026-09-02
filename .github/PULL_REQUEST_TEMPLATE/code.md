<!-- Tracking fields: associate this PR with its GitHub issue through the Development/linked-issue field. Add the PR and issue to project 82; set both project items to `In review` when the PR opens, and set this PR's Sprint field to the current sprint. These are fields, not body sections. -->

## Summary

<!-- State what changed and why it matters. -->

## How this fits together

<!-- Required: describe the real runtime/data flow. Do not list changed files. Replace the placeholder graph with grounded nodes and edges. -->
```mermaid
flowchart LR
  source[Real input or trigger]
  change[Real decision or changed component]
  outcome[Observed output or outcome]
  source --> change
  change --> outcome
```

## Affected behaviour

<!-- Describe changed behaviour, failure handling, compatibility, and deliberately unchanged behaviour. -->

## Verification

<!-- List exact commands or scenarios and observed results. Include Red-before-Green evidence when applicable. -->


## Dependencies and stack position

<!-- State the current base, predecessors, migrations, and incremental scope. -->

- Base:
- Predecessors:
- Dependencies:

## Review notes

<!-- List bounded risks, security/performance/compatibility concerns, and focused questions. -->

## Required delivery checks

- [ ] The linked issue exists and its Type is `Task` or `Bug`.
- [ ] The applicable `specs/` scope was updated with the correct disposition.
- [ ] New or updated tests defend the changed behavior or contract.
- [ ] The Mermaid diagram models the real flow and is not a file inventory.
- [ ] The PR's Development/linked-issue field associates it with the issue.
- [ ] The PR and linked issue are in project 82 with status `In review`.
- [ ] The PR's Sprint field is set to the current sprint.
- [ ] The authenticated human uploader is the PR assignee.
