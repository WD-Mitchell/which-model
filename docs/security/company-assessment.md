# Company assessment package — version 1

Prepared 2026-09-11 for #291 against implementation revision
`b0bc8dacd20d95adc50c141433d1d74888222641` (native PR stack #300, through #315).
The accompanying PR changes documentation only. This package is implementation
and test evidence for review; **maintainer sign-off, release approval and company
acceptance are pending**. The product remains pre-release.

Start with the mode you intend to deploy:

| Mode | Product boundary | Evidence and remaining decision |
|---|---|---|
| Restricted offline score-only | Separate `which-model-score-only` executable; embedded repository catalog and profiles; no runtime configuration, credentials, provider network, persistence or execution | [Threat model](company-threat-model.md), [artifact evidence](company-evidence.md), [restricted install](offline-score-only.md). Release owner must approve redistribution; company must approve the exact artifact and ranking use. |
| Managed native usage | Full application with protected administrator profile, allowed providers/sources, native OS stores and minimized local records | [Data inventory](privacy-and-retention.md), [policy](managed-company-profile.md), [credentials](secure-credentials.md). Provider endpoint/client/grant permission and tenant/endpoint controls need external evidence. |
| Approved CodexBar delegation | Adds a separately approved native image and config, verified before credential delegation/execution | [Delegation boundary](approved-codexbar.md). Administrator reviews upstream helpers, stores and destinations; compatible Windows CodexBar distribution is not established. |
| Agent integration | Separately permitted skills/hooks and approved harness execution | [Execution](approved-execution.md), [advisory behavior](advisory-evidence.md). Harness permissions, project trust, tool approvals, data access and egress require company controls. |

The full application's `--no-usage` flag is not the restricted artifact. A managed
profile is optional; these restrictions are not silently applied to personal
users. Managed capabilities are individually permitted, not enabled by selecting
a row in this table. Missing quota/authentication evidence or failed audit writes
remain visible advisories and **do not block otherwise authorized launches**.
Actual provider/source, policy and executable restrictions still apply.

## How to review

1. Read the four [data/trust flows and threat register](company-threat-model.md).
2. Follow each reported concern through the [framework and ownership matrix](company-controls.md).
3. Check source, CI and exact candidate identifiers in [the evidence inventory](company-evidence.md).
4. Record a human product review below; keep company decisions in the company's
   own assessment record. Do not attach credentials, personal account details or
   internal tenant identifiers to this public repository.

## Scope and accepted product decisions

The requester's review findings are the initial checklist; no additional company
control baseline or acceptance criteria have been supplied. Company acceptance is
separate. The agreed platform scope is macOS, Windows and Linux. The profile
retains OS/provider identity; no application RBAC or central broker is introduced.
CodexBar starts disabled and needs exact installation approval. GitHub/Sigstore
provenance is the initial release trust mechanism. No new support/SLA commitment
is in scope.

Company persistence defaults to identity-free snapshots for 24 hours, launch
records for 7 days, and pick history/audit for 30 days, configurable only by the
administrator. Zero retention disables that category and removes existing owned
records. Necessary credential metadata has the credential lifetime. Identity-free
persistence is minimization, not anonymity or a restriction on transient identity
or an explicit live identity display.

The offline snapshot is the repository catalog plus built-in profiles, with exact
hashes in artifact evidence. Data rights remain the release owner's responsibility.
A numeric version, working login, green CI result or signed artifact does not
resolve the separate company/provider/data-rights decisions.

## Product review record

| Field | Current record |
|---|---|
| Review subject | Assessment package version 1 and implementation baseline above; evidence inventory identifies exact test/artifact revisions. |
| Prepared review | Code/spec/data-flow trace and framework reference checks completed by the implementation assistant; reproducible checks are in the evidence inventory. This is not independent assessment. |
| Human maintainer | The human assignee of the linked implementation PR; no completed review is claimed. |
| Human result / date / reviewed SHA | **Pending human review.** Record approval or findings on the PR and amend this record with the reviewed SHA/date before marking issue AC4 complete. |
| Known product/release gaps | Candidate evidence is not a published release; no stable promotion; upstream Windows CodexBar package unestablished; no full end-to-end agent penetration test or live company-account validation; external provider permissions unresolved. |
| Organisation acceptance | **Pending, separate company record.** Product review does not accept risk on the company's behalf. |

## Company reviewer decisions and acceptance checklist

Owner roles identify responsibility, not an appointed person or completed approval.
Each company record needs the selected mode, OS/version/architecture, deployed
artifact digest, policy digest where applicable, reviewer, date, observed result
and expiry/review trigger. Keep sensitive evidence in the company's system.

| ID | Decision or evidence to record | Owner | Current state |
|---|---|---|---|
| D1 | Select deployment modes and exact framework baseline/subcontrols/parameters, including how the original SA-12 concern maps to Rev. 5 SR. | Company security/control assessor | Listed findings adopted as initial scope; detailed tailoring and acceptance pending. |
| D2 | Approve source/artifacts, trusted verifier/roots, protected deployment and prevention of older/unmanaged binaries where required. Archive candidate/release evidence before hosted artifacts expire. | Endpoint and release owners | Technical evidence prepared; actual fleet deployment and final release selection pending. |
| D3 | Approve each provider endpoint/client/grant and company account, SSO/MFA/offboarding, scope and provider terms. | Provider relationship and IAM owners | Unresolved; do not infer permission from a successful request. Native usage can remain disabled while the restricted pilot is assessed. |
| D4 | Accept privacy data flow and 24h/7d/30d defaults; define backups, identity-display, filesystem access, prior-project cleanup and provider/harness data treatment. | Privacy and endpoint owners | Product defaults agreed; no company-specific retention policy supplied; privacy acceptance pending. |
| D5 | If delegation is required, approve exact CodexBar image/config/dependency installation, storage, helpers, credentials and recipients; identify a compatible Windows binary before enabling there. | Development platform and security owners | Default disabled; no real installation approved by this work. |
| D6 | Exercise native harness sandbox/tool/egress controls with synthetic untrusted project and tool-output injection; review permitted skills/hooks, shell/runtime dependencies and direct invocation. | Development platform and security owners | Product invocation tests exist; full agent misuse/exfiltration evaluation pending. |
| D7 | Accept advisory quota/audit semantics, or provide external enforcement and tamper-resistant logging if company requirements demand them. | Security operations and control assessor | Requester chose non-blocking launches; organisation acceptance and external evidence remain pending. |
| D8 | Record repository-catalog upstream terms/redistribution approval, approved snapshot/update route and task-specific ranking suitability. | Release/data owner and company AI owner | Current repository snapshot selected; redistribution and fitness decisions pending. |
| D9 | Accept pre-release lifecycle, existing support limits, supplier/notification arrangements and update/incident/revocation procedures without inventing an SLA. | Company procurement/security and product release owners | Documentation prepared; external supplier acceptance pending. |
| D10 | Decide residual risk acceptance and review schedule; record findings, remediation owner and target before enabling affected capabilities. | Company accountable risk owner | No acceptance recorded. |

Reopen this assessment when credential scopes/endpoints, defaults, data fields or
retention, policy authority, executable/config trust, bundled data, dependencies,
release trust roots/workflow, supported platforms or native harness tool access
change. The changing PR updates the relevant specs, threat/control rows and exact
source/artifact evidence. A passed old run cannot validate new behavior. Do not
create arbitrary remediation deadlines in this repository on behalf of the company.
