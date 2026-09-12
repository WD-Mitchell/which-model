# Advisory quota and audit evidence (#286)

Decision: the requester explicitly does not require missing/stale quota,
usage-authentication errors or audit-write failures to block launch. These are
advisory evidence controls. Protected executable approval and native OS/harness/
provider permissions remain authoritative. Personal behavior remains unchanged.

## Exact action matrix

| Evidence condition | Recommendation/reporting | Otherwise approved company launch / dispatch |
|---|---|---|
| Current computable route windows | Report current evidence, not a future allowance guarantee. Preserve existing ranking weights/strategy rules. | Proceed; evidence does not grant permission. |
| Missing snapshot, missing windows, unknown/synthetic usage or incomplete required window set | Report unavailable or partial evidence; never convert missing values into confirmed zero usage. Each route keeps its own state. | Proceed with an explicit advisory. |
| Stale flag, evidence beyond the applicable freshness budget, invalid/future recording time | Report stale/invalid evidence and unconfirmed allowance. Existing quota ranking remains unchanged; stale values are not described as current. | Proceed with an explicit advisory. |
| Usage-authentication failure | Report authentication evidence unavailable, using a fixed message/code; no raw provider error. | Proceed with an explicit advisory; native agent/provider authentication can still fail independently. |
| Other provider/collection failure | Report provider evidence unavailable; do not borrow another route's healthy result. | Proceed with an explicit advisory. |
| Usage disabled, or score-only ranking requested/used | Clearly label score-only recommendation. Do not claim quota informed the ranking. | Proceed; a model score is not a launch permission. |
| Pre-launch audit intent write fails | Report intent not recorded. Never claim audit success. | Proceed; attempt the post-launch record independently. |
| Actual process start succeeds; post-launch audit/history/log write fails | Report process started and the specific recording stage unavailable. | Return successful launch plus advisory; do not turn a running process into a reported launch failure. |
| Native process start or executable/permission verification fails | Report actual refusal/start failure. Record failure outcome if possible. | Return the real launch failure; evidence cannot bypass it. |
| Copy-command mode | Report command prepared for copying; no process was started. | Return copy result and any recording advisory. Clipboard success belongs to the host UI. |

A company hook's quota/no-pick signal is advisory (`decision=approve`), including
critical-band recommendations. This does not change CLI band filtering or strategy
ranking; no candidate may still be a legitimate recommendation result. Personal
hook decisions remain unchanged. Hook installation/use permissions still apply
before dispatch and remain capable of refusing unauthorized hook use.

## Freshness, retries and mixed routes

No additional live fetch, credential resolution, authentication prompt or retry is
introduced on launch. Desktop reporting uses the latest in-memory collection result
per provider; absent evidence after restart remains absent until normal collection.
Desktop's existing normal 15-minute cache freshness budget and a snapshot's stale
flag inform the advisory. The existing force path uses its one-minute cache-age
request; it does not become a launch prerequisite. CLI reporting uses the actual
returned per-provider snapshot and effective max-age/provider cache budget.
Provider timestamps must be valid and not in the future to claim current evidence.

Collection continues to use existing adapter timeouts/retries and normal refresh
entry points. Report failures on their own routes even when another route is
healthy. Band/strategy algorithms retain their existing rules, including mixed
known windows and auth/provider exclusions. The reporting layer describes missing
or partial evidence rather than silently replacing it with a healthy route's
allowance. A usage-disabled priority recommendation stays a score-only operation;
usage-required strategies retain their existing explicit refusal when usage is off.

Desktop Rank remains score-only and adds an explicit company label. It does not
start applying quota weights. CLI usage-aware picks keep their current weights,
gates and exit classes; `--no-usage` remains the explicit score-only route. Advisory
messages use stable application text and operational IDs, without account identity,
credential material or raw provider/harness payloads.

Stored numeric band evidence requires every selected-route window to be computable, independently of age. Aging a partial/unknown observation must not introduce a numeric band. CLI text explanations display the fixed stored quota message as `quota at pick`, including stale and score-only reports; they do not reinterpret a historical observation as a new live check.

## Intent, outcome and persistence

For an approved actual launch, write a best-effort audit intent immediately before
process start, then independently write the observed started/failed outcome.
Correlate the records with a generated operational launch ID. A successful intent
is not proof that the process started. Copy mode records command preparation,
not a process-start or clipboard-success claim. Keep #284's structured launch log
and its separate retention; audit records use the 30-day audit category.

Returned company launch advisories expose quota and pre/post recording failures
without replacing a successful process-start result. The desktop displays the
result and advisories; an advisory must not disappear solely because automatic
popover closing was enabled. Native start errors remain errors with fixed text.
No failed audit write triggers a fallback to raw text or an alternate storage path.
