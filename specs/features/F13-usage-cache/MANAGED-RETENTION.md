# Managed privacy and retention (#284)

This corrects F13's no-retention, identity-preserving and read-only-offline rules
for the optional company profile. Personal behavior remains unchanged. The
requester's defaults are identity-free persistence, usage snapshots for 24 hours,
launch records for 7 days, and pick history/audit records for 30 days. Protected
administrator settings may change these finite durations.

## Storage and transition

The retention clock is the application's original recording time, independent of
provider freshness/TTL. Rewriting a record to remove identity never refreshes that
clock. Expired records are physically removed when maintenance runs, including
when usage is read offline. Missing, invalid or future recording timestamps cannot
create immortal records. Malformed/oversized owned records are removed during
maintenance; redirected or unsafe filesystem entries are refused and reported.
Concurrent company append/prune operations use the same bounded OS file lock.

Maintenance runs on normal full-CLI startup, desktop startup and periodic desktop
maintenance, before affected reads/writes, and through an explicit privacy command.
No process can delete files while it is not running: deployments needing deletion
at a wall-clock deadline must schedule the documented maintenance command with
endpoint tooling. The standalone offline score-only package remains independent
of this work and does not perform maintenance or access application data.

The current CLI usage-cache root and desktop cache root are both included where
they differ. Pick history remains `state/pick/history.jsonl`. Company launch
records use `state/launch.jsonl`; captured stdout/stderr is never persisted by
which-model, and legacy `state/launch.log` is removed during managed transition.
Company hook evidence moves to `state/audit/evidence.jsonl` and
`state/audit/mismatches.jsonl`, with explicit recording timestamps. Maintenance
removes the two owned legacy audit files in the selected/current project. It does
not scan every repository on disk; endpoint rollout must name other prior project
roots. Provider-owned files, OS credential records, user-authored configuration,
backups, synchronized copies and provider-side retention are outside this cleanup.

## Identity and payload minimization

Identity-free usage persistence omits account and plan, free-form window labels
and reset hints. Operational provider/window/model identifiers, numeric usage,
expiry/reset times and source/confidence remain. Histories and audits retain only
explicit typed selection evidence: profile/strategy/model identifiers, scores,
bands, timestamps and stable reason codes. Unknown fields, account identity,
workspace paths, prompts, raw errors and provider/harness payloads are excluded.
Company launch logs contain only timestamp, operational IDs and a fixed outcome.
These operational IDs are not an anonymization guarantee: administrators should
choose nonpersonal profile and model aliases.

Necessary provider routing/expiry metadata stays within #283's OS-protected
credential record. User-authored configuration, including aliases deliberately
chosen there, is not rewritten or deleted by a data-retention operation. Output
redaction does not prevent provider collection; the data-flow inventory distinguishes
transient collection, requested identity display and persisted fields.

## Failure and deletion reporting

The explicit command reports per-category retained, removed, scrubbed and failed
counts without printing record contents or identity-bearing paths. A failed
cleanup exits unsuccessfully and never claims deletion. Automatic maintenance
reports a fixed warning; bookkeeping failure does not become a harness-launch
gate. A record that cannot be made safe is not returned as an eligible cached
snapshot. A failed write does not authorize an alternative persistence location.
Company hook audit failures remain advisory and visible, consistent with the
requester's #286 decision. No raw diagnostic payload is substituted for a failed
structured write.


## Command cadence and bounded processing

The full CLI skips startup maintenance for config, privacy, version and
--no-usage; the explicit privacy command performs its requested operation once.
Desktop startup here means StartDataRefresher, then a one-minute tick until its
context ends. A newly enrolled running desktop must restart. Maintenance is not
a background service while the application is stopped.

Each owned JSONL store is bounded at 64 MiB, each record/cache at 4 MiB and each
cache directory enumeration at 1,024 entries. Oversized owned record files are
deleted without retaining their raw content; an overfull directory reports
incomplete work rather than claiming complete inspection. Empty stores are
removed. An adjacent empty .privacy.lock file coordinates append/read/prune with
a two-second lock timeout; it carries no record data and is not an audit record.

Managed FetchAll diagnostics retain canonical error codes and fixed messages;
unknown provider codes become provider_status. Only exact application-owned
native secure-store messages may pass through to preserve lock/denial guidance.
Personal diagnostics are unchanged. This applies after either backend returns;
#287 remains responsible for approving and containing the delegated executable.

Zero retention is supported: an administrator value of `0` disables persistence
for that category and removes its existing owned records when maintenance or a
write is attempted. It never means unlimited retention.


## Advisory launch audit fields (#286)

The central audit store additionally permits generated `launch_id`, operational `profile`, `quota_state` (global §15 enum), and phase `launch_intent`, `launch_started`, `launch_failed` or `copy_prepared`. They retain the approved 30-day audit default; structured launch summaries retain seven days. A zero-day audit policy persists no evidence and cannot report audit success. Intent is not a process-start claim; neither a missing audit record nor a recording failure proves launch was blocked.
