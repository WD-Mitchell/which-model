<!-- Tracking fields: associate this PR with its GitHub issue through the Development/linked-issue field. This is a field, not a body section. -->

## Summary

<!-- State the durable product truth this specification establishes or clarifies. -->

## How this fits together

<!-- Required: show how requirements/contracts flow into governed behavior. Do not list changed files. Avoid bloat: omit nodes and edges that do not change a reader's understanding of the flow. Diagrams with four or fewer boxes MUST use a horizontal layout. For more than four boxes, use available space in both dimensions when the real flow permits it; do not default to a single chain. Every horizontal row has a hard cap of four boxes, with three preferred; vertical depth has no hard cap. -->
```mermaid
flowchart LR
  request[Approved product outcome]
  contract[Requirement or contract]
  behavior[Governed behavior and acceptance]
  request --> contract
  contract --> behavior
```

## Affected behaviour

<!-- State the scope package, coverage, terminology, requirements, contracts, decisions, and non-goals affected. -->

## Verification

<!-- Cite the independent spec-review verdict, scope/integrity checks, and resolved baseline or all-REFERENCE rationale. -->


## Dependencies and stack position

<!-- State the current base, predecessor spec PRs, imported contracts, and incremental scope. -->

- Base:
- Predecessors:
- Dependencies:

## Review notes

<!-- List terminology, inheritance, authority, duplication, and baseline risks. -->

## Required delivery checks

- [ ] The linked issue exists and its Type is `Task` or `Bug`.
- [ ] Every affected scope has exactly one CREATE, EXTEND, or REFERENCE disposition.
- [ ] Independent `spec-review` returned `APPROVE` for this exact draft.
- [ ] The Mermaid diagram shows requirement/contract flow into behavior.
- [ ] No implementation plan, task breakdown, ticket archive, or per-ticket acceptance definition was added to `specs/`.
- [ ] New or updated tests defend the specification contract.
- [ ] The PR's Development/linked-issue field associates it with the issue.
- [ ] The authenticated human uploader is the PR assignee.
