# Company evidence inventory — version 1

Assessment state and human review: [company assessment](company-assessment.md).
Product baseline: `b0bc8dacd20d95adc50c141433d1d74888222641`, through PR #315.
The #291 PR adds documentation only. This inventory does not claim an artifact
built from a different source SHA is identical to the baseline.

## Source and CI

[CI run 34544150578](https://github.com/WD-Mitchell/which-model/actions/runs/34544150578)
uses that exact source SHA. Native policy, credential, privacy, execution, CodexBar,
restricted-score and release-verifier jobs passed on macOS, Windows and Ubuntu.
The workflow's source is [pinned here](https://github.com/WD-Mitchell/which-model/blob/b0bc8dacd20d95adc50c141433d1d74888222641/.github/workflows/ci.yml).
A platform runner covers its actual environment, not every OS version or CPU.
Native credentials/delegation use synthetic fixtures; no customer credentials or
real CodexBar installation were used. CI's macOS desktop backend compilation and
browser/component tests do not establish shipped Windows/Linux desktop bundles.

The evidence IDs below bind tests to that source and CI. Relative source links
resolve to the assessment checkout; use the pinned source above when reviewing a
later branch. Linked implementation PRs show incremental code/spec changes, not
proof of merge or deployment.

| ID | Behavior / negative evidence | Test or reproducible command | Delivery reference and limits |
|---|---|---|---|
| E1 | Restricted offline capabilities, deterministic ranking, hostile config/env/credential canaries and zero application network/child effects | [score-only tests](../../pkg/scoreonly/run_test.go): `TestBundledRankingMatchesExistingEngine`, `TestExcludedCommandsAndOverrides`, `TestManifestAndTop`; `bash scripts/audit-score-only.sh`; Linux `scripts/score_only_smoke.py --trace` | [#302](https://github.com/WD-Mitchell/which-model/pull/302). Three native OS jobs plus Linux network-namespace syscall trace; not a claim that the OS loader reads nothing. |
| E2 | Protected policy authority; malformed/missing required policy, ownership/ACL, symlink and config/env override refusals before effects | [native boundaries](../../internal/company/boundaries_test.go), [policy tests](../../internal/company/policy_test.go), [protected files](../../internal/company/files_test.go); native `company-policy` CI fixtures | [#305](https://github.com/WD-Mitchell/which-model/pull/305). Standard-user boundary within the approved binary; no control over older/unmanaged binaries. |
| E3 | Native store success/error handling, no company owned plaintext fallback, explicit migration/readback/cleanup and redirected-source refusal | [native store tests](../../internal/securestore), [company fallback test](../../internal/usage/credential/company_test.go), [migration tests](../../internal/usage/credential/migrate_test.go); `secure-credentials` matrix | [#306](https://github.com/WD-Mitchell/which-model/pull/306). Locked/unavailable/permission errors include deterministic seams; native test stores use synthetic values. No live provider grant or fleet policy validation. |
| E4 | Identity minimization, physical expiry, no clock renewal, zero retention, purge, redirected path refusal and concurrent record writes | [retention tests](../../internal/privacy/retention_test.go), [maintenance tests](../../internal/privacy/maintenance_test.go), [hook privacy](../../internal/hooks/privacy_test.go); `company-privacy` matrix | [#307](https://github.com/WD-Mitchell/which-model/pull/307). Owned files only; no proof of backup/media erasure or provider/harness cleanup. |
| E5 | Protected native image/input identity, fixed args, custom-shell denial, inherited secret rejection, changed/redirected image refusal and owned integration cleanup | [plan tests](../../internal/approvedexec/plan_test.go), [native execution/integration tests](../../internal/service/harness_company_native_test.go), [foreign-hook refusal](../../internal/hooks/company_removal_test.go); `company-execution` matrix | [#308](https://github.com/WD-Mitchell/which-model/pull/308). Real synthetic child proves argv/env boundary, not containment of an agent's descendants, dynamic libraries or tools. |
| E6 | Missing/partial/stale/auth/provider evidence and pre/post audit failures remain advisory; scores unchanged, route-specific freshness, correlated intent/start/copy phases | [service advisory tests](../../internal/service/advisory_test.go), [hook matrix](../../internal/hooks/advisory_test.go), [CLI labels](../../pkg/whichmodel/pick_advisory_test.go), [UI fixture/screenshots](../../output/playwright/company-286/README.md); `go test ./internal/advisory ./internal/hooks ./internal/service ./pkg/whichmodel` | [#313](https://github.com/WD-Mitchell/which-model/pull/313), [UI/source CI at ba3aed6](https://github.com/WD-Mitchell/which-model/actions/runs/34541527876). Original visual fixture revision is disclosed there. No mandatory launch block or tamper-resistant log claim. |
| E7 | Default delegation denial; image/config approval before credential read and execution; cache-only no delegation; bounded typed credential/env/output; wrong-provider/source/error/timeout matrix | [preflight/fetch tests](../../internal/usage/fetch/company_codexbar_test.go), [adapter matrix](../../internal/usage/provider/codexbar/company_test.go), [native invocation](../../internal/usage/provider/codexbar/company_native_test.go); `company-codexbar` matrix | [#314](https://github.com/WD-Mitchell/which-model/pull/314). Synthetic three-OS native executable; no actual upstream Windows package established. |
| E8 | Source/workflow/ref-bound signature verification, SBOM linkage, optional-package/fallback refusal, staged non-executable download and receipt validation | [release verifier tests](../../npm/which-model/verify-release.test.js), [installer tests](../../npm/which-model/install.test.js); `node --test npm/which-model/*.test.js`; native `release-verifier` matrix | [#301](https://github.com/WD-Mitchell/which-model/pull/301). 21 local installer/verifier tests passed at baseline. npm postinstall warns and preserves install exit behavior; an unverifiable fallback is not runnable. |
| E9 | Ten candidate binaries and SBOMs, signed scan reports, bundled hashes, altered-source/ref/manifest refusals and independently reproduced restricted binary | Exact candidate and commands below; [machine-readable inventory](evidence/company-291.json) | Verification-only [run 34544198645](https://github.com/WD-Mitchell/which-model/actions/runs/34544198645). Release and npm publish jobs skipped. No published-release provenance or live installation claim. |
| E10 | Windows path hygiene, explicit local identity/owners, pre-release metadata/support and outstanding approval | `python3 scripts/check_tracked_paths.py`; native `windows-cli`; `actionlint -oneline .github/workflows/npm-release.yml`; reviewed [identity](company-identity.md) and [readiness](../releases/readiness.md) | [#298](https://github.com/WD-Mitchell/which-model/pull/298), [#299](https://github.com/WD-Mitchell/which-model/pull/299), [#315](https://github.com/WD-Mitchell/which-model/pull/315). Human readiness/assessment review remains pending; private reporting was enabled when read on 2026-09-11. |

Historical npm verification in E8 also checked all six published `2.5.5` packages
against source `90abb8d07c8b21930350f01d57fe14b6f755b8d5` with
`node npm/scripts/verify-provenance.js`, including a wrong-source refusal. The
[PR #301 record](https://github.com/WD-Mitchell/which-model/pull/301) identifies
that observation. It does not validate an npm publication of the new company
candidate; that publication has not occurred.

These tests are adversarial fixtures, not a full penetration test. No complete
end-to-end prompt injection/exfiltration evaluation, independent assessor report,
live company-account test or fleet rollout is credited. A known broader legacy
`go test -tags nousage ./pkg/whichmodel` compile limitation references usage-only
hook-test symbols; supported release `audit-nousage.sh` and the distinct restricted
package pass. It is not presented as a green all-package/all-tag test suite.

## Exact candidate evidence

- Version: `2.5.6-company291.1` (candidate label only; no version tag created).
- Source: `b0bc8dacd20d95adc50c141433d1d74888222641`.
- Ref: `refs/heads/codex/company-289-release-readiness`.
- Workflow: `.github/workflows/npm-release.yml`, run `34544198645`, `verify_only=true`.
- Artifact: `release-assets`, ID `10178489175`; hosted archive SHA-256
  `0a3c8c6b443a7db37225f9c64f2bada9173522f2d6921b2c97560adce40af277`.
- Hosted expiration: `2026-12-09T23:54:31Z`. The company/release owner must archive
  the original bytes, bundle and approved trust roots; this JSON inventory is not
  a replacement signature or permanent artifact host.
- Build/scanner: Go `1.26.8`, govulncheck `1.8.0`; CycloneDX `1.6` per-binary SBOMs.
  All ten signed binary scan reports said `No vulnerabilities found.` at build
  time. This is scanner/database evidence, not proof of absence of vulnerabilities.
- Independent verification used trusted GitHub CLI `2.97.0`, the source checkout's
  verifier and a separately exported trust root obtained before this candidate.
  All ten binary/SBOM pairs, manifest, checksums, capabilities and ten scan reports
  verified against the expected source/ref/workflow. Wrong source SHA, wrong ref
  and an altered signed manifest were refused.
- After verification, the macOS arm64 restricted binary's capabilities matched
  the signed manifest byte-for-byte; repeated ranking output was identical. A
  separate local build with the same source/toolchain/flags reproduced that
  binary's SHA-256: `b6868e8b036c80432a92aea6d30c05c7b3ee67b26eb1fb8eb020ea049daff374`.
  The other nine targets were built, inventoried and signature-verified; independent
  byte reproduction and native execution of every target are not claimed.

Embedded catalog SHA-256:
`55f3809d54368a563c6e7bfec1b06485e9614cf5329e2d760662b4991c738959`.
Built-in profile serialization SHA-256:
`e7709300375b04ead9dcc726e22ac4ecc277a5b5204f0e2bd1bb82f4ca03bd69`.
These match the requester's repository snapshot choice. Release-owner data-rights
approval remains open.

The [JSON record](evidence/company-291.json) contains each binary/SBOM digest and
hashes of the evidence files. It records observed results, not self-authenticating
trust. Signed provenance must be verified with separately approved tooling/roots.

## Reproduce the checks

Run from the reviewed source checkout with trusted Node and GitHub CLI. The shell
variables below identify local directories containing downloaded evidence and an
independently obtained trust root; do not use a root supplied by an untrusted mirror.

```sh
gh run download 34544198645 --name release-assets --dir "$candidate_dir"
node npm/which-model/verify-release.js "$candidate_dir" 2.5.6-company291.1 \
  b0bc8dacd20d95adc50c141433d1d74888222641 \
  refs/heads/codex/company-289-release-readiness "$trusted_root"
```

Expected summary: `Verified 10 artifacts and SBOMs` with the candidate version,
full source commit and ref. Verify scan reports separately with the same identity
(the main release verifier checks binaries/SBOMs; it does not read scan conclusions):

```sh
for report in "$candidate_dir"/*.govulncheck.txt; do
  gh attestation verify "$report" --bundle "$candidate_dir/provenance.jsonl" \
    --repo WD-Mitchell/which-model --hostname github.com \
    --signer-workflow WD-Mitchell/which-model/.github/workflows/npm-release.yml \
    --source-digest b0bc8dacd20d95adc50c141433d1d74888222641 \
    --source-ref refs/heads/codex/company-289-release-readiness \
    --predicate-type https://slsa.dev/provenance/v1 \
    --cert-oidc-issuer https://token.actions.githubusercontent.com \
    --deny-self-hosted-runners --custom-trusted-root "$trusted_root" || exit 1
done
```

Read the verified reports and retain findings; signature success does not mean a
scan found nothing. For independent restricted macOS arm64 reproduction:

```sh
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 GOTOOLCHAIN=go1.26.8 \
  go build -trimpath -buildvcs=false -tags nousage \
  -ldflags '-s -w -X github.com/WD-Mitchell/which-model/pkg/scoreonly.Version=2.5.6-company291.1 -X github.com/WD-Mitchell/which-model/pkg/scoreonly.Commit=b0bc8dacd20d95adc50c141433d1d74888222641' \
  -o "$rebuilt_binary" ./cmd/which-model-score-only
cmp "$rebuilt_binary" "$candidate_dir/which-model-score-only-darwin-arm64"
```

Grant execution permission only after verification. The CI-only protected-policy
fixtures refuse to enroll a normal development machine; use isolated CI/test
endpoints and synthetic credentials for native ownership/store tests.

## Earlier milestone checkpoint

R1's restricted candidate `2.5.6-company290.1` was built at
`7301bfbb19220ad26dcc52d2daa23bcbd0afbb13` in
[run 34521085309](https://github.com/WD-Mitchell/which-model/actions/runs/34521085309).
It established the restricted input/artifact boundary before managed consumers.
The R2/R3 PRs then supplied native controls; E9 refreshes binary evidence at the
complete implementation baseline. Older evidence is retained as history, not
substituted for the final candidate. Final release publication, npm provenance for
that publication, upgrade/rollback deployment validation and company acceptance
remain release/company responsibilities.
