"use strict";

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const REPO = 'WD-Mitchell/which-model';
const WORKFLOW = `${REPO}/.github/workflows/npm-release.yml`;
const DIGEST = /^[0-9a-f]{64}$/;
const COMMIT = /^[0-9a-f]{40}$/;
const REF = /^refs\/(heads|tags)\/[A-Za-z0-9][A-Za-z0-9._/-]*$/;
const NAME = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
const VERSION = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$/;

function validateManifest(m, version, sourceDigest, sourceRef) {
  if (!m || m.schema !== 1 || !VERSION.test(m.version) || m.version !== version ||
      !COMMIT.test(m.source_digest) || !REF.test(m.source_ref) ||
      (sourceDigest !== undefined && m.source_digest !== sourceDigest) ||
      (sourceRef !== undefined && m.source_ref !== sourceRef) ||
      !Array.isArray(m.artifacts) || m.artifacts.length === 0 || m.artifacts.length > 64) {
    throw new Error('release policy does not match the expected version and source');
  }
  const names = new Set();
  for (const a of m.artifacts) {
    if (!a || !NAME.test(a.name) || !NAME.test(a.sbom) || a.sbom !== `${a.name}.cdx.json` ||
        !DIGEST.test(a.sha256) || !DIGEST.test(a.sbom_sha256) || names.has(a.name) || names.has(a.sbom)) {
      throw new Error('release policy contains invalid or duplicate artifact metadata');
    }
    names.add(a.name); names.add(a.sbom);
  }
  if (m.artifacts.some(a => a.name.startsWith('which-model-score-only-')) || m.score_only !== undefined) {
    if (m.score_only?.capabilities !== 'which-model-score-only-capabilities.json' || !DIGEST.test(m.score_only.sha256)) {
      throw new Error('restricted release requires a pinned capability manifest');
    }
  }
  return m;
}

function digest(file) {
  return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

function verifyArtifact({ artifact, bundle, sha256, sourceDigest, sourceRef, trustedRoot, digestAlgorithm = 'sha256', run = execFileSync }) {
  if (!COMMIT.test(sourceDigest) || !REF.test(sourceRef)) throw new Error('expected source identity is required');
  if (!['sha256', 'sha512'].includes(digestAlgorithm)) throw new Error('unsupported artifact digest algorithm');
  if (sha256 !== undefined && (!DIGEST.test(sha256) || digest(artifact) !== sha256)) throw new Error('artifact checksum mismatch');
  if (!fs.existsSync(bundle) || fs.statSync(bundle).size === 0) throw new Error('provenance bundle is missing');
  const args = ['attestation', 'verify', path.resolve(artifact), '--bundle', path.resolve(bundle),
    '--digest-alg', digestAlgorithm,
    '--hostname', 'github.com', '--repo', REPO, '--signer-workflow', WORKFLOW, '--source-digest', sourceDigest,
    '--source-ref', sourceRef, '--predicate-type', 'https://slsa.dev/provenance/v1',
    '--cert-oidc-issuer', 'https://token.actions.githubusercontent.com', '--deny-self-hosted-runners'];
  if (trustedRoot) args.push('--custom-trusted-root', path.resolve(trustedRoot));
  try {
    run('gh', args, { shell: false, timeout: 120000, maxBuffer: 1024 * 1024, stdio: 'pipe', windowsHide: true });
  } catch (_) {
    // Verifier diagnostics may contain local paths or account information.
    throw new Error('provenance verification failed; install a trusted GitHub CLI 2.97.0+ and check release evidence/network access');
  }
}

function hasVerifiedFallback(file, asset, policy, receipt, version) {
  try {
    validateManifest(policy, version);
    const expected = policy.artifacts.find(a => a.name === asset);
    return Boolean(expected && receipt && receipt.version === version &&
      receipt.source_digest === policy.source_digest && receipt.source_ref === policy.source_ref &&
      receipt.sha256 === expected.sha256 && digest(file) === expected.sha256);
  } catch (_) { return false; }
}

function verifyRelease(dir, version, sourceDigest, sourceRef, trustedRoot) {
  const manifestPath = path.join(dir, 'release-manifest.json');
  const bundle = path.join(dir, 'provenance.jsonl');
  const identity = { bundle, sourceDigest, sourceRef, trustedRoot };
  // Verify the manifest before treating its artifact metadata as evidence.
  verifyArtifact({ artifact: manifestPath, ...identity });
  verifyArtifact({ artifact: path.join(dir, 'checksums.txt'), ...identity });
  const manifest = validateManifest(JSON.parse(fs.readFileSync(manifestPath, 'utf8')), version, sourceDigest, sourceRef);
  let capabilities;
  if (manifest.score_only) {
    const file = path.join(dir, manifest.score_only.capabilities);
    verifyArtifact({ artifact: file, sha256: manifest.score_only.sha256, ...identity });
    capabilities = JSON.parse(fs.readFileSync(file, 'utf8'));
    if (capabilities.artifact !== 'which-model-score-only' || capabilities.version !== version ||
        capabilities.source_commit !== sourceDigest || capabilities.usage_enabled !== false ||
        capabilities.usage_disabled_reason !== 'compiled_out' || !DIGEST.test(capabilities.catalog?.sha256) ||
        !DIGEST.test(capabilities.profiles?.sha256)) {
      throw new Error('restricted capabilities do not describe the release artifact');
    }
  }
  for (const a of manifest.artifacts) {
    verifyArtifact({ artifact: path.join(dir, a.name), sha256: a.sha256, ...identity });
    verifyArtifact({ artifact: path.join(dir, a.sbom), sha256: a.sbom_sha256, ...identity });
    verifyArtifact({ artifact: path.join(dir, `${a.name}.govulncheck.txt`), ...identity });
    const sbom = JSON.parse(fs.readFileSync(path.join(dir, a.sbom), 'utf8'));
    if (sbom.bomFormat !== 'CycloneDX' || sbom.specVersion !== '1.6' ||
        sbom.metadata?.component?.name !== a.name || sbom.metadata?.component?.version !== version ||
        !sbom.metadata?.component?.hashes?.some(h => h.alg === 'SHA-256' && h.content === a.sha256)) {
      throw new Error('SBOM does not describe its release artifact');
    }
    if (a.name.startsWith('which-model-score-only-')) {
      for (const key of ['catalog', 'profiles']) {
        if (!sbom.components?.some(c => c['bom-ref'] === `bundled-${key}` &&
            c.name === capabilities[key].source_path &&
            c.hashes?.some(h => h.alg === 'SHA-256' && h.content === capabilities[key].sha256))) {
          throw new Error('restricted SBOM does not describe its bundled inputs');
        }
      }
    }
  }
  return manifest;
}

if (require.main === module) {
  const [dir, version, sourceDigest, sourceRef, trustedRoot] = process.argv.slice(2);
  try {
    if (!dir || !VERSION.test(version || '') || !COMMIT.test(sourceDigest || '') || !REF.test(sourceRef || '')) {
      throw new Error('usage: verify-release.js <directory> <version> <full-source-commit> <source-ref> [trusted-root.jsonl]');
    }
    const manifest = verifyRelease(dir, version, sourceDigest, sourceRef, trustedRoot);
    console.log(`Verified ${manifest.artifacts.length} artifacts and SBOMs for ${version} at ${sourceDigest} (${sourceRef}).`);
  } catch (error) {
    console.error(`which-model release: ${error.message}`);
    process.exitCode = 1;
  }
}
module.exports = { validateManifest, verifyArtifact, verifyRelease, hasVerifiedFallback };
