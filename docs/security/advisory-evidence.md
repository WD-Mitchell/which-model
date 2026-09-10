# Quota and audit evidence in the company profile

The approved company policy treats quota and audit evidence as advisory. Missing,
stale or authentication-failed allowance data and failed recording attempts do
not block an otherwise approved harness launch. Native OS, harness and provider
permissions and [approved executable controls](approved-execution.md) still apply.
Personal-user behavior remains unchanged.

Desktop recommendations are labelled **Score-only recommendation** because their
ranking uses model scores. The selected route's allowance report appears beside
that label. Another provider's healthy observation cannot stand in for this route.
The CLI retains its existing usage-aware ranking and exclusions; an explicit
`pick --no-usage --strategy priority` labels the score-only result. Usage-required
strategies continue to require usage. Company hooks report unavailable quota or
recommendation evidence and approve dispatch, including when no pick survives.

Examples of fixed notices:

- `Quota evidence is stale; allowance is unconfirmed.`
- `Usage authentication evidence is unavailable; allowance is unconfirmed.`
- `Pre-launch audit intent was not recorded.`
- `Post-launch audit was not recorded; the process started.`

A successful process start stays successful when a later record fails. The desktop
shows **Process started** and keeps the recording notice visible until dismissed.
Copy mode says **Command prepared for copying**, which does not claim a process
started or the clipboard operation succeeded. Actual process-start failures remain
launch errors. There is no new launch-time fetch, login prompt or retry.

Audit intent and outcome records share a generated launch ID and include only
allowed operational fields. A successful intent is not proof of execution. Audit
retention defaults to 30 days and launch summaries to seven days; zero-day audit
retention reports that no evidence was recorded. See the
[retention inventory](privacy-and-retention.md) for cleanup and ownership boundaries.

The [normative action matrix](../../specs/desktop/backend/features/B07-harnesses/ADVISORY-EVIDENCE.md)
defines freshness, mixed-route handling, failure outcomes and regression coverage.
These reports are not security-enforcement controls or a complete audit trail.
Company acceptance of that advisory limitation is an external decision.
