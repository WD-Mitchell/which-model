#!/usr/bin/env node
"use strict";
// Inspect the exact six published packages without executing install scripts or binaries.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { verifyArtifact } = require('../which-model/verify-release');
const [version, commit, output] = process.argv.slice(2);
let scratch;
try {
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$/.test(version || '') ||
      !/^[0-9a-f]{40}$/.test(commit || '') || !output) throw new Error('invalid verification arguments');
  const launcher = require('../which-model/package.json');
  const names = [launcher.name, ...Object.keys(launcher.optionalDependencies)];
  scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'which-model-npm-evidence-'));
  fs.writeFileSync(path.join(scratch, 'package.json'), JSON.stringify({name:'release-evidence',version:'1.0.0',private:true,
    dependencies:Object.fromEntries(names.map(name=>[name,version]))}));
  const settings = {cwd:scratch,stdio:'pipe',timeout:300000,maxBuffer:16*1024*1024,shell:false};
  // --force permits inspecting every platform package on this Linux release runner.
  // --ignore-scripts prevents the downloaded packages from running any code.
  execFileSync('npm',['install','--ignore-scripts','--force','--no-audit','--no-fund','--registry=https://registry.npmjs.org'],settings);
  const raw = execFileSync('npm',['audit','signatures','--json','--include-attestations','--registry=https://registry.npmjs.org'],settings);
  const report = JSON.parse(raw);
  if (report.invalid?.length || report.missing?.length || report.verified?.length !== names.length) throw new Error('incomplete registry verification');
  for (const name of names) {
    const pkg = report.verified.find(p=>p.name===name && p.version===version);
    const signed = pkg?.attestationBundles?.find(b=>b.predicateType==='https://slsa.dev/provenance/v1');
    if (!signed) throw new Error('a package has no verified SLSA provenance');
    const statement = JSON.parse(Buffer.from(signed.bundle.dsseEnvelope.payload,'base64').toString('utf8'));
    const build = statement.predicate?.buildDefinition;
    const workflow = build?.externalParameters?.workflow;
    if (workflow?.repository!=='https://github.com/WD-Mitchell/which-model' ||
        workflow?.path!=='.github/workflows/npm-release.yml' || workflow?.ref!==`refs/tags/v${version}` ||
        !build?.resolvedDependencies?.some(d=>d.uri===`git+https://github.com/WD-Mitchell/which-model@refs/tags/v${version}` && d.digest?.gitCommit===commit)) {
      throw new Error('verified package provenance has an unexpected source or workflow');
    }
    // npm audit verifies signatures and subjects, but does not enforce our
    // certificate identity policy. Verify the original registry tarball with
    // GitHub CLI so workflow-controlled predicate claims cannot select trust.
    const packReport = JSON.parse(execFileSync('npm', ['pack', `${name}@${version}`, '--json',
      '--ignore-scripts', '--pack-destination', scratch, '--registry=https://registry.npmjs.org'], settings));
    // npm 12 keys JSON results by package name; earlier npm returns an array.
    const packed = Array.isArray(packReport) ? packReport :
      packReport && typeof packReport === 'object' ? Object.values(packReport) : [];
    if (packed.length !== 1 || packed[0].name !== name || packed[0].version !== version ||
        typeof packed[0].filename !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._-]*\.tgz$/.test(packed[0].filename)) {
      throw new Error('npm pack did not return the expected package tarball');
    }
    const bundlePath = path.join(scratch, `${packed[0].filename}.sigstore.json`);
    fs.writeFileSync(bundlePath, JSON.stringify(signed.bundle), { mode: 0o600, flag: 'wx' });
    verifyArtifact({ artifact: path.join(scratch, packed[0].filename), bundle: bundlePath,
      digestAlgorithm: 'sha512', sourceDigest: commit, sourceRef: `refs/tags/v${version}` });
  }
  fs.writeFileSync(output, JSON.stringify({...report,version,source_digest:commit,checked_at:new Date().toISOString(),
    certificate_identities_verified:names},null,2)+'\n');
  console.log(`Verified registry signatures, provenance and source identity for all ${names.length} npm packages at ${version}.`);
} catch (_) {
  console.error('npm provenance validation failed; check package availability, signatures, attestations, expected source identity and a trusted GitHub CLI 2.97.0+.');
  process.exitCode=1;
} finally {
  if (scratch) fs.rmSync(scratch,{recursive:true,force:true});
}
