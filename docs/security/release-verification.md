# Verifying CLI release provenance

Issue #288 adds GitHub/Sigstore verification to standalone release artifacts and
npm fallback installation. It preserves the existing npm trusted-publishing path.
These requirements apply to releases built with this change; they do not add
missing attestations retrospectively to older immutable releases.

## Trust policy

The verifier accepts SLSA v1 provenance only for `WD-Mitchell/which-model`, signed
by `.github/workflows/npm-release.yml` using GitHub Actions OIDC on a GitHub-hosted
runner. It requires the expected full source commit and source ref. The artifact
digest must match both the signed subject and the release manifest. Production
publication runs at the version tag; manual branch runs first verify a candidate,
then create the tag and hand publication to the tag run.

Use a trusted GitHub CLI installation (2.97.0 or later). GitHub CLI performs
signature, certificate, identity and transparency verification using Sigstore
trust material. Obtain the verifier and any offline roots through an approved
channel, not from the release being evaluated. A checksum from the same release
is an extra corruption check; it cannot establish the publisher's identity.

This trust model still relies on GitHub's identity service, the approved workflow,
repository access/branch controls, the build environment and Sigstore trust roots.
It cannot prove that authorised source is safe or that an authorised maintainer
has not changed the workflow maliciously. Company acceptance and any additional
signing or approval process remain separate. Sigstore provenance does not supply
macOS notarisation or Windows Authenticode signing.

## Release contents

| Artifact | Purpose |
|---|---|
| Five `which-model-<platform>` binaries | CLI for macOS arm64/x64, Linux arm64/x64 and Windows x64 |
| `<binary>.cdx.json` | CycloneDX 1.6 SBOM, including binary SHA-256, linked Go module versions and Go runtime |
| `release-manifest.json` | Schema/version/source ref/full commit and each binary/SBOM digest |
| `checksums.txt` | Existing binary SHA-256 list |
| `provenance.jsonl` | Signed bundle covering binaries, inventories, manifest, checksums and scan reports |
| `<binary>.govulncheck.txt` | Pinned scanner's binary vulnerability assessment |
| `verification.txt` | Build job's independent verification result; verify artifacts yourself rather than trusting this text |

The repository-owned SBOM generator reads each binary's embedded Go build metadata
with `go version -m -json`. Its source revision is the generator pin. It rejects
unversioned local replacements, records versioned replacements, and does not guess
licences or direct/transitive graph edges. It inventories the released Go binary,
not its host OS, external harnesses or provider applications. Python's standard
library is sufficient for generation. The release pins `govulncheck` to v1.8.0,
`actions/attest` to commit `1e69f48acb82d1966a394da916b4c1698aa569d6` (v4.2.2),
publishing npm to 12.0.2, and the release test/build toolchain to Go 1.26.8.
The scanner requires Go 1.26 or later; the product language minimum remains in
go.mod. Node 24 runs in hosted Actions; the actual Go runtime version is included
in each SBOM.

Vulnerability findings or scanner errors fail the release build. This check is
separate from the non-blocking runtime quota/audit decision. Release verification
also runs after artifact transfer and before npm packaging. An npm rerun uses the
actual immutable GitHub release assets, not newly rebuilt bytes.

## Direct downloads and mirrors

Acquire the expected version, complete source commit and source ref through your
approved release review. Download all binaries, their SBOMs and
`<binary>.govulncheck.txt` reports, manifest, checksum list and bundle into one
directory. Use the verification helper from a trusted source
checkout containing this change. It verifies the manifest before consuming its
artifact metadata, checks every listed binary/SBOM and its signed scan report,
and confirms each SBOM names and hashes the matching binary. A missing or altered
report fails this whole-release check before artifacts can be republished.

With `release_dir`, `release_version` and `release_commit` set to the approved
values, run in Bash on macOS/Linux:

```sh
node npm/which-model/verify-release.js "$release_dir" "$release_version" "$release_commit" "refs/tags/v$release_version"
```

The same helper works in PowerShell on Windows after setting `$releaseDir`,
`$releaseVersion` and `$releaseCommit`:

```powershell
node npm/which-model/verify-release.js $releaseDir $releaseVersion $releaseCommit "refs/tags/v$releaseVersion"
if ($LASTEXITCODE -ne 0) { throw "Release verification failed" }
```

Only place a binary on PATH or grant execute permission after verification passes.
For a branch candidate use its reviewed `refs/heads/...` ref instead of a tag.
The helper's successful output lists the version, source ref/commit and artifact
count; an incorrect source, altered bytes or unverifiable signature returns exit 1.

A mirror transports the same untrusted bytes and evidence; it cannot select the
expected repository, workflow, version or source identity. Offline verification
requires a trust-root export obtained independently on an approved connected host:

```sh
gh attestation trusted-root > trusted_root.jsonl
```

Transfer that approved root with the artifacts and append its path as the helper's
fifth argument. GitHub CLI uses `--bundle` and `--custom-trusted-root`, avoiding
online attestation lookup and trust-root retrieval. Maintain root updates through
the company's trusted tooling channel. If required evidence is missing, obtain it
through that channel; do not disable verification or substitute a mirror's root.

## npm installation

Ordinary installs use the matching optional platform package. npm registry
integrity and provenance apply to that package; `npm audit signatures` provides
an explicit signature/attestation check where the package manager supports it.
The release gate installs all six exact-version packages with scripts disabled,
verifies their npm signatures/attestations and checks the recorded repository,
workflow, tag and source revision using `npm/scripts/verify-provenance.js`.
It also downloads the exact registry tarballs with scripts disabled and uses
GitHub CLI 2.97.0+ to verify their SHA-512 subjects and OIDC-derived certificate
identities against the same repository, workflow, tag, commit, issuer and hosted
runner policy. Predicate values are consistency checks; they cannot substitute
for certificate identity. Success evidence lists all six packages under
`certificate_identities_verified` only after these checks pass.
Its result is uploaded as the `npm-provenance` workflow artifact.

The launcher includes a release policy stamped from the verified manifest. If an
optional platform package is unavailable, postinstall requires that policy and a
trusted GitHub CLI. It downloads bounded checksum/binary/bundle data over approved
GitHub HTTPS destinations, stages bytes as `candidate.download` outside the launcher path (mode 0600 on
POSIX; inherited package-directory ACL on Windows), never executes them, verifies the
expected checksum and signed source identity, then grants execute permission and
atomically installs the binary with a verification receipt. The launcher requires
a matching version/source receipt and current binary digest before using a local
fallback, so stale/unverified leftovers are refused. This protects installation
integrity within the package directory; a same-user actor able to replace the
launcher or its policy remains outside the local identity boundary. Any failure leaves no new runnable fallback and
prints a warning while preserving the existing successful npm-install exit code.
Install the matching platform package or correct the verifier/evidence problem;
there is no checksum-only bypass. `WHICH_MODEL_SKIP_DOWNLOAD=1` still opts out.
A verifier is not invoked when an optional package already supplies the binary.

## Evidence and verification-only runs

Baseline inspection on 2026-09-10: all six published npm packages at **2.5.5** have
verified registry signatures and SLSA attestations. `npm audit signatures --json
--include-attestations` identifies the release workflow, `refs/tags/v2.5.5`, and
source `90abb8d07c8b21930350f01d57fe14b6f755b8d5`. The then-current standalone
GitHub release contains five binaries and checksums, without these new bundles or
SBOMs. Existing npm provenance is retained rather than duplicated.

For a reviewed branch, dispatch `npm-release.yml` with its candidate `version`
and `verify_only=true`. The run tests, builds, scans, attests and verifies candidate
artifacts. It skips the jobs that create tags or publish releases/packages.
Download `release-assets`, use the run's full commit and branch ref to verify
independently, and retain the verification output. Before accepting #288, also
exercise altered bytes, wrong source commit/ref, wrong repository/workflow and
missing/malformed bundles against the candidate. Release publication remains a
separate human-reviewed action.

Primary references: [GitHub artifact attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations),
[GitHub CLI verification](https://cli.github.com/manual/gh_attestation_verify),
[CycloneDX 1.6 schema](https://github.com/CycloneDX/specification/blob/1.6/schema/bom-1.6.schema.json),
[Go build metadata](https://pkg.go.dev/debug/buildinfo), and
[npm signature auditing](https://docs.npmjs.com/cli/v11/commands/npm-audit/).
