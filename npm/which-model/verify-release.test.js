"use strict";
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { verifyArtifact, validateManifest } = require('./verify-release');
const bytes = Buffer.from([0, 255, 1]);
const sha = crypto.createHash('sha256').update(bytes).digest('hex');
const source = 'a'.repeat(40);
const ref = 'refs/tags/v1.2.3';
const manifest = () => ({ schema: 1, version: '1.2.3', source_digest: source, source_ref: ref,
  artifacts: [{name:'which-model-linux-x64',sha256:sha,sbom:'which-model-linux-x64.cdx.json',sbom_sha256:sha}] });
test('manifest binds version, source, unique safe artifact names and SBOMs', () => {
  assert.doesNotThrow(() => validateManifest(manifest(), '1.2.3', source, ref));
  for (const mutate of [m=>m.version='1.2.4',m=>m.source_digest='b'.repeat(40),m=>m.source_ref='refs/tags/v9.0.0',
    m=>m.artifacts[0].name='../binary',m=>m.artifacts[0].sbom='/tmp/sbom',m=>m.artifacts[0].sha256='invalid',
    m=>m.artifacts.push(m.artifacts[0]),m=>m.artifacts=[],m=>m.schema=2]) {
    const m=manifest();mutate(m);assert.throws(()=>validateManifest(m,'1.2.3',source,ref));
  }
});
test('restricted artifacts require a safe, digest-pinned capability manifest', () => {
  const m=manifest();
  m.artifacts[0].name='which-model-score-only-linux-x64';
  m.artifacts[0].sbom=m.artifacts[0].name+'.cdx.json';
  assert.throws(()=>validateManifest(m,'1.2.3',source,ref),/capability manifest/);
  m.score_only={capabilities:'which-model-score-only-capabilities.json',sha256:sha};
  assert.doesNotThrow(()=>validateManifest(m,'1.2.3',source,ref));
  for (const invalid of [{capabilities:'../manifest.json',sha256:sha},{capabilities:m.score_only.capabilities,sha256:'wrong'},null]) {
    assert.throws(()=>validateManifest({...m,score_only:invalid},'1.2.3',source,ref),/capability manifest/);
  }
});
test('verifier pins cryptographic identity and checks bytes before calling gh', () => {
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'which-model-verifier-'));
  try {
    const artifact=path.join(dir,'binary'), bundle=path.join(dir,'provenance.jsonl');
    fs.writeFileSync(artifact,bytes);fs.writeFileSync(bundle,'synthetic bundle');
    const calls=[];
    const options={ artifact,bundle,sha256:sha,sourceDigest:source,sourceRef:ref,run:(...args)=>calls.push(args) };
    verifyArtifact(options);
    const [command,args,settings]=calls[0];
    assert.equal(command,'gh');
    for (const [flag,value] of [['--repo','WD-Mitchell/which-model'],['--signer-workflow','WD-Mitchell/which-model/.github/workflows/npm-release.yml'],['--source-digest',source],['--source-ref',ref],['--predicate-type','https://slsa.dev/provenance/v1'],['--cert-oidc-issuer','https://token.actions.githubusercontent.com']]) assert.equal(args[args.indexOf(flag)+1],value);
    assert.ok(args.includes('--deny-self-hosted-runners'));
    assert.equal(settings.shell,false); assert.ok(settings.timeout>0);
    fs.writeFileSync(artifact,'changed');assert.throws(()=>verifyArtifact(options));assert.equal(calls.length,1);
  } finally { fs.rmSync(dir,{recursive:true,force:true}); }
});
test('missing evidence and verifier refusal never become verification success', () => {
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'which-model-verifier-'));
  try {
    const artifact=path.join(dir,'binary'),bundle=path.join(dir,'bundle');fs.writeFileSync(artifact,bytes);
    const options={artifact,bundle,sha256:sha,sourceDigest:source,sourceRef:ref,run:()=>{throw new Error('synthetic private detail');}};
    assert.throws(()=>verifyArtifact(options),/bundle/);
    fs.writeFileSync(bundle,'test');
    for (const reason of ['wrong repository','wrong workflow','wrong source','invalid signature','missing gh','timeout']) {
      assert.throws(()=>verifyArtifact({...options,run:()=>{throw new Error(reason);}}),/provenance verification failed/);
    }
  } finally { fs.rmSync(dir,{recursive:true,force:true}); }
});

test('release verification requires every signed vulnerability report', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'which-model-scan-evidence-'));
  try {
    const m = manifest();
    const artifact = m.artifacts[0];
    const report = path.join(dir, `${artifact.name}.govulncheck.txt`);
    const sbom = JSON.stringify({ bomFormat: 'CycloneDX', specVersion: '1.6', metadata: {
      component: { name: artifact.name, version: m.version, hashes: [{ alg: 'SHA-256', content: sha }] },
    } });
    artifact.sbom_sha256 = crypto.createHash('sha256').update(sbom).digest('hex');
    fs.writeFileSync(path.join(dir, 'release-manifest.json'), JSON.stringify(m));
    fs.writeFileSync(path.join(dir, artifact.name), bytes);
    fs.writeFileSync(path.join(dir, artifact.sbom), sbom);
    fs.writeFileSync(path.join(dir, 'checksums.txt'), `${sha}  ${artifact.name}\n`);
    fs.writeFileSync(path.join(dir, 'provenance.jsonl'), 'synthetic signed subjects');
    fs.writeFileSync(report, 'signed scan result');
    // Model a cryptographic verifier accepting only the original signed bytes.
    const signed = new Map(fs.readdirSync(dir).map(name => [path.join(dir, name), fs.readFileSync(path.join(dir, name))]));
    const checked = [];
    const run = (_command, args) => {
      const file = args[2];
      checked.push(file);
      assert.deepEqual(fs.readFileSync(file), signed.get(file));
    };
    const loaded = { exports: {} };
    require('node:vm').runInNewContext(fs.readFileSync(path.join(__dirname, 'verify-release.js'), 'utf8'), {
      module: loaded, require: name => name === 'node:child_process' ? { execFileSync: run } : require(name),
    });
    const { verifyRelease } = loaded.exports;
    verifyRelease(dir, m.version, source, ref);
    assert.ok(checked.includes(report), 'the published scan report must be verified');
    fs.writeFileSync(report, 'changed after signing');
    assert.throws(() => verifyRelease(dir, m.version, source, ref), /verification failed/);
    fs.unlinkSync(report);
    assert.throws(() => verifyRelease(dir, m.version, source, ref), /verification failed/);
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('fallback launch requires a matching verification receipt and unchanged bytes', () => {
  const { hasVerifiedFallback } = require('./verify-release');
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'which-model-receipt-'));
  try {
    const file=path.join(dir,'binary'); fs.writeFileSync(file,bytes);
    const receipt={version:'1.2.3',source_digest:source,source_ref:ref,sha256:sha};
    assert.equal(hasVerifiedFallback(file,'which-model-linux-x64',manifest(),receipt,'1.2.3'),true);
    for(const invalid of [null,{}, {...receipt,version:'1.2.2'}, {...receipt,source_digest:'b'.repeat(40)}, {...receipt,sha256:'0'.repeat(64)}]) {
      assert.equal(hasVerifiedFallback(file,'which-model-linux-x64',manifest(),invalid,'1.2.3'),false);
    }
    fs.writeFileSync(file,'tampered');
    assert.equal(hasVerifiedFallback(file,'which-model-linux-x64',manifest(),receipt,'1.2.3'),false);
  } finally {fs.rmSync(dir,{recursive:true,force:true});}
});
