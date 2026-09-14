"use strict";
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const verifier = require('../which-model/verify-release');
const launcher = require('../which-model/package.json');
const source = fs.readFileSync(path.join(__dirname, 'verify-provenance.js'), 'utf8');
const version = '1.2.3';
const commit = 'a'.repeat(40);
const names = [launcher.name, ...Object.keys(launcher.optionalDependencies)];

// npm auditing supplies a verified signature and subject. The separate gh
// boundary must still accept the certificate identity before evidence is saved.
function inspect({ certificateError, packFilename, wrongPredicate = false, legacyPack = false } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'which-model-npm-policy-test-'));
  const output = path.join(dir, 'evidence.json');
  const calls = [], messages = [];
  const processStub = { argv: ['node', 'verify-provenance.js', version, commit, output], exitCode: 0 };
  const predicate = { buildDefinition: {
    externalParameters: { workflow: { repository: 'https://github.com/WD-Mitchell/which-model',
      path: '.github/workflows/npm-release.yml', ref: `refs/tags/v${version}` } },
    resolvedDependencies: [{ uri: `git+https://github.com/WD-Mitchell/which-model@refs/tags/v${version}`,
      digest: { gitCommit: wrongPredicate ? 'b'.repeat(40) : commit } }],
  } };
  const bundle = { dsseEnvelope: { payload: Buffer.from(JSON.stringify({ predicate })).toString('base64') } };
  const report = { invalid: [], missing: [], verified: names.map(name => ({ name, version,
    attestationBundles: [{ predicateType: 'https://slsa.dev/provenance/v1', bundle }] })) };
  const run = (command, args, settings) => {
    calls.push({ command, args, settings });
    if (command === 'gh') {
      assert.ok(fs.readFileSync(args[2]).length);
      assert.deepEqual(JSON.parse(fs.readFileSync(args[args.indexOf('--bundle') + 1], 'utf8')), bundle);
      if (certificateError) throw new Error(certificateError);
      return Buffer.from('verified');
    }
    assert.equal(command, 'npm');
    if (args[0] === 'install') return Buffer.from('');
    if (args[0] === 'audit') return Buffer.from(JSON.stringify(report));
    assert.equal(args[0], 'pack');
    assert.ok(args.includes('--ignore-scripts'));
    const name = names.find(name => args.includes(`${name}@${version}`));
    assert.ok(name, 'pack the exact audited version');
    const filename = packFilename || `${name.replace('@', '').replace('/', '-')}-${version}.tgz`;
    if (!packFilename) fs.writeFileSync(path.join(settings.cwd, filename), `tarball for ${name}`);
    const entry = { name, version, filename };
    return Buffer.from(JSON.stringify(legacyPack ? [entry] : { [name]: entry }));
  };
  try {
    vm.runInNewContext(source, { Buffer, process: processStub,
      console: { log: text => messages.push(text), error: text => messages.push(text) },
      require(name) {
        if (name === '../which-model/package.json') return launcher;
        if (name === '../which-model/verify-release') return { ...verifier,
          verifyArtifact: options => verifier.verifyArtifact({ ...options, run }) };
        if (name === 'node:child_process') return { execFileSync: run };
        return require(name);
      },
    });
    return { status: processStub.exitCode, calls, messages, evidence: fs.existsSync(output),
      scratchRemoved: calls.filter(c => c.command === 'npm').every(c => !fs.existsSync(c.settings.cwd)) };
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
}

test('all six exact npm tarballs require certificate verification before success evidence', () => {
  const result = inspect();
  assert.equal(result.status, 0, result.messages.join('\n'));
  assert.equal(result.evidence, true);
  const calls = result.calls.filter(c => c.command === 'gh');
  assert.equal(calls.length, names.length);
  for (const { args } of calls) {
    for (const [flag, value] of [['--digest-alg', 'sha512'], ['--repo', 'WD-Mitchell/which-model'],
      ['--signer-workflow', 'WD-Mitchell/which-model/.github/workflows/npm-release.yml'],
      ['--source-digest', commit], ['--source-ref', `refs/tags/v${version}`],
      ['--cert-oidc-issuer', 'https://token.actions.githubusercontent.com']]) {
      assert.equal(args[args.indexOf(flag) + 1], value);
    }
    assert.ok(args.includes('--deny-self-hosted-runners'));
  }
  assert.equal(result.scratchRemoved, true);
});

for (const identity of ['repository', 'workflow', 'source commit', 'source ref', 'issuer', 'runner']) {
  test(`matching npm predicate cannot override a rejected certificate ${identity}`, () => {
    const result = inspect({ certificateError: `wrong certificate ${identity}` });
    assert.equal(result.status, 1);
    assert.equal(result.evidence, false);
    assert.equal(result.scratchRemoved, true);
  });
}

test('npm predicate mismatches remain rejected', () => {
  const result = inspect({ wrongPredicate: true });
  assert.equal(result.status, 1);
  assert.equal(result.evidence, false);
});

test('older npm array pack reports also preserve certificate verification', () => {
  const result = inspect({ legacyPack: true });
  assert.equal(result.status, 0, result.messages.join('\n'));
  assert.equal(result.calls.filter(c => c.command === 'gh').length, names.length);
});

test('npm pack cannot select a tarball outside the verification directory', () => {
  for (const packFilename of ['../outside.tgz', '/outside.tgz', '..\\outside.tgz']) {
    const result = inspect({ packFilename });
    assert.equal(result.status, 1);
    assert.equal(result.evidence, false);
  }
});
