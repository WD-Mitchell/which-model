# Release maturity and readiness

**Product maturity: pre-release.** Version numbers, package channels, passing CI
and signed provenance are not a stable-release decision or company approval.
Commands, configuration and integrations may still change. Stable promotion
requires a maintainer decision against the criteria below, recorded in a reviewed PR.
No stable promotion is made by this document.

## Current publication and support

Review date: 2026-09-11. GitHub's newest published release is
[v2.5.5](https://github.com/WD-Mitchell/which-model/releases/tag/v2.5.5), published
2026-09-05. Its GitHub prerelease flag is false; npm `latest` points to `2.5.5` and
`beta` to `2.0.0-beta.1`. These are historical distribution labels, not recorded
stable-readiness approvals. The company changes in native PR stack #300 are not
included in that published release.

The updated release workflow marks **all new GitHub releases as pre-release**,
including plain numeric tags, and does not promote them to GitHub Latest. npm
keeps its existing channels: numeric versions use `latest`; versions with a
prerelease suffix use `beta`. These npm tags select packages, not maturity or
company approval. Existing releases/dist-tags are not rewritten by this PR.
A client following only GitHub Latest may continue seeing the historical release;
preview adopters and company administrators must select the intended version
explicitly from the release list and verify its evidence.

The source-built desktop's `Check for updates…` action includes published
prereleases and compares semantic versions. It offers only a newer version and
opens that version's release page; an older historical Latest release is never
offered as an update. Development and unrecognized builds open the release list
with manual-selection guidance. The action does not install a binary or approve
the selected version for a company deployment.

Security fixes target `main` and the newest published release. Older versions may
require an upgrade; there is no maintained backport series or LTS promise. Preserve
configuration before upgrading, review changes to specs/configuration, and test the
selected version with your providers and harnesses. Managed deployments must pin
an approved binary and update protected policy/input hashes as needed. An old
binary that predates managed policy is not governed by that policy; endpoint
application controls must enforce the deployed version where required.

The [security policy](../../SECURITY.md) describes the existing private reporting
route and practical handling. No new response deadline, remediation SLA, support
contract or funded support investigation is introduced.

## Platform and build evidence

| Surface | Distribution/build targets | Observed validation and limits |
|---|---|---|
| Full CLI and npm binaries | macOS arm64/x64, Linux arm64/x64, Windows x64 | Release build targets are explicit. Native CI covers macOS, Ubuntu Linux and Windows; an OS runner is not proof of every architecture/OS-version combination. |
| Restricted offline score-only CLI | Same five release targets, separate binary | Embedded catalog/profiles, import/symbol/endpoint audits, native OS checks and Linux-isolated syscall evidence. It is separate from the full npm executable. |
| Managed policy, owned credential storage and approved execution | macOS, Windows, Linux | Native protected-file/ACL, OS-store and execution fixtures on all three OS families. Live customer provider accounts and company endpoint settings remain deployment validation. |
| Approved CodexBar delegation | which-model approval boundary on all three OS families | Native synthetic image/config invocation on all three. Reviewed upstream CLI distributions are macOS/Linux; a compatible Windows CodexBar binary is not established. |
| Desktop UI | Source-built Wails app; macOS backend build | Browser fixture/component evidence and macOS build are available. This CLI release workflow does not publish desktop application bundles or establish complete Windows/Linux desktop distribution support. |

`go.mod` declares Go 1.25.0 as the module floor; verified release builds use pinned
Go 1.26.8 and govulncheck 1.8.0. The npm wrapper declares Node >=18 compatibility;
release publication uses Node 24. These build facts do not claim every permitted
runtime is covered by CI or promise security maintenance for external runtimes.

## Stable promotion checklist

Every row requires evidence at the **exact proposed release revision and artifacts**.
Earlier candidate runs demonstrate a control, not proof that a later release passed.

| Criterion | Required evidence | Status at this review |
|---|---|---|
| Source and behavior | Reviewed spec/code consistency, passed CI, no unresolved release-blocking regressions; install/upgrade/rollback scenarios on claimed targets | PRs provide implementation evidence; release revision and maintainer review pending. |
| Artifact integrity | Verified source-bound GitHub/Sigstore provenance, artifact checksums, per-binary SBOMs, successful vulnerability scans and npm provenance when npm is published | Verified candidate pipeline exists; no final company release has been published. |
| Restricted pilot boundary | Exact embedded catalog/profile hashes, capability refusals, import/symbol/network isolation checks, redistribution decision | Technical candidate evidence exists; redistribution/release approval remains owner work. |
| Managed deployment | Protected policy, native stores, minimization/retention, approved execution and delegated-input tests on each claimed OS | Native CI passes for the implementation stack; customer enrollment/account/egress/upgrade acceptance is separate. |
| Release metadata and installation | Product maturity consistent across docs, GitHub release flag and package description; version-pinned installation guidance and verified candidate smoke checks | This PR aligns future metadata. Historical flags and unpublished company features remain disclosed. |
| Operational ownership | Current support/version/platform matrix, private vulnerability channel, named release reviewer, unresolved gaps recorded | Reporting is enabled; matrix and evidence prepared. Maintainer review/sign-off remains pending. |
| Stable decision | Maintainer approval referencing these rows, known limits, chosen release and required follow-up work; update governing spec and release metadata together | **Not approved. Product stays pre-release.** |

Company security/privacy/control acceptance is a separate record owned by the
company. Neither completing this checklist nor publishing a stable tag constitutes
that acceptance. The [advisory evidence policy](../security/advisory-evidence.md)
remains non-blocking by the requester's decision; stable promotion cannot silently
turn it into an enforcement control.

## Review record for #289

Prepared for maintainer review; **not a maintainer sign-off**. Human reviewer:
the assigned uploader on the implementation PR. Decision: retain pre-release status
pending an explicit promotion record. No new support commitments are proposed.

Evidence prepared:

- Windows checkout and native CLI portability: [PR #298](https://github.com/WD-Mitchell/which-model/pull/298).
- Verified release provenance/installer/SBOM controls: [PR #301](https://github.com/WD-Mitchell/which-model/pull/301).
- Restricted score-only candidate at `7301bfbb19220ad26dcc52d2daa23bcbd0afbb13`:
  [candidate run 34521085309](https://github.com/WD-Mitchell/which-model/actions/runs/34521085309),
  [PR #302](https://github.com/WD-Mitchell/which-model/pull/302).
- Managed policy/credentials/retention/approved execution: native checks in
  [PR #305](https://github.com/WD-Mitchell/which-model/pull/305),
  [PR #306](https://github.com/WD-Mitchell/which-model/pull/306),
  [PR #307](https://github.com/WD-Mitchell/which-model/pull/307),
  [PR #308](https://github.com/WD-Mitchell/which-model/pull/308).
- Advisory launch evidence at `ba3aed68cf32a40fd84a3ebd0517b886466503d9`:
  [CI 34541527876](https://github.com/WD-Mitchell/which-model/actions/runs/34541527876).
- Approved delegation at `318198bd07c5866000b1aec817ae42bb633a56a0`:
  [CI 34543206353](https://github.com/WD-Mitchell/which-model/actions/runs/34543206353).
- Private vulnerability reporting: repository API reported `enabled: true` on
  2026-09-11; the existing Security-tab route and fallback are documented in SECURITY.md.

Open release decisions: select the reviewed release revision/version; retain or
refresh candidate evidence for it; approve redistribution; complete maintainer
review and actual deployment validation; decide whether stable criteria have been
met. Do not check these off based on PR creation or CI alone. #291 consolidates the
security/control evidence and company-owned acceptance decisions.
