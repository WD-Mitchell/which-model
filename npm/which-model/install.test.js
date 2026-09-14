"use strict";
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const { validateManifest } = require('./verify-release');
const source = fs.readFileSync(path.join(__dirname, 'install.js'), 'utf8');
const binary = Buffer.from([0, 255, 128, 10, 13, 42]);
const digest = createHash('sha256').update(binary).digest('hex');
const asset = 'which-model-linux-x64';
const good = `${digest}  ${asset}\n`;

// Exercise the actual entry point with synthetic network, filesystem and verifier boundaries.
async function install(checksum = good, options = {}) {
  const { optional=false, skip=false, status=200, verificationError=false, interrupt=false,
    wrongVersion=false, missingPolicy=false, redirect, missingBundle=false, tamper=false } = options;
  const requests=[], writes=[], warnings=[], events=[], renames=[], removed=[];
  const stage='/package/.which-model-install-test';
  const policy={schema:1,version:wrongVersion?'9.9.9':'1.2.3',source_digest:'a'.repeat(40),source_ref:'refs/tags/v1.2.3',
    artifacts:[{name:asset,sha256:digest,sbom:asset+'.cdx.json',sbom_sha256:digest}]};
  const fakeRequire = name => {
    if (name==='./package.json') return {version:'1.2.3'};
    if (name==='./release-policy.json') { if(missingPolicy) throw new Error('missing');return policy; }
    if (name==='./verify-release') return {validateManifest, verifyArtifact: value => {
      events.push('verify');
      assert.equal(value.sourceDigest,policy.source_digest);assert.equal(value.sourceRef,policy.source_ref);
      assert.equal(writes[0][2].mode,0o600);
      if(verificationError) throw new Error('private-canary-detail');
    }};
    if (name==='fs') return {
      existsSync:()=>optional, mkdtempSync:()=>stage,
      writeFileSync:(...args)=>{writes.push(args);events.push('write');},
      chmodSync:()=>events.push('chmod'),
      renameSync:(...args)=>{if(interrupt) throw new Error('interrupted');renames.push(args);events.push('rename');},
      rmSync:target=>removed.push(target),
    };
    if (name==='https') return {get(url, _settings, callback) {
      requests.push(url);
      const request=new EventEmitter();request.setTimeout=()=>{};request.destroy=()=>{};
      queueMicrotask(()=>{
        const response=new EventEmitter();
        const isBundle=url.endsWith('provenance.jsonl');
        Object.assign(response,{statusCode:redirect?302:(isBundle&&missingBundle?404:status),headers:redirect?{location:redirect}:{},resume(){},destroy(){}});
        callback(response);
        response.emit('data',url.endsWith('checksums.txt')?Buffer.from(checksum):isBundle?Buffer.from('synthetic-bundle'):tamper?Buffer.from('changed'):binary);
        response.emit('end');
      });return request;
    }};
    return require(name);
  };
  fakeRequire.resolve=()=>{if(!optional)throw new Error('missing optional');return '/optional/package.json';};
  vm.runInNewContext(source,{require:fakeRequire,__dirname:'/package',Buffer,URL,
    process:{platform:'linux',arch:'x64',env:{WHICH_MODEL_SKIP_DOWNLOAD:skip?'1':'0'}},
    console:{warn:message=>warnings.push(message)}});
  await new Promise(setImmediate);
  return {requests,writes,warnings,events,renames,removed};
}
for (const [name,checksum] of [['LF',good],['CRLF and binary marker',`${digest} *${asset}\r\n`]]) {
  test(`verified fallback preserves bytes with ${name} checksums`,async()=>{
    const r=await install(checksum);
    assert.equal(r.requests.length,3);assert.equal(r.renames.length,2,r.warnings.join('\n'));
    assert.equal(r.renames[1][1],path.join('/package','which-model'));assert.deepEqual(r.writes[0][1],binary);
    assert.ok(r.events.indexOf('verify')<r.events.indexOf('chmod'));
    assert.ok(r.events.indexOf('chmod')<r.events.indexOf('rename'));
    assert.equal(r.removed.length,1);assert.match(r.warnings[0],/installed .*verified/);
  });
}
for (const [name,checksum,options] of [
  ['missing checksum',`${digest}  other-file\n`,{}],['mismatched checksum',`${'0'.repeat(64)}  ${asset}\n`,{}],
  ['malformed checksum','invalid',{}],['HTTP failure',good,{status:404}],['missing bundle',good,{missingBundle:true}],
  ['changed binary',good,{tamper:true}],['wrong version',good,{wrongVersion:true}],['missing release policy',good,{missingPolicy:true}],
  ['untrusted redirect',good,{redirect:'https://attacker.example/binary'}],
  ['malformed redirect',good,{redirect:'https://['}],
  ['verifier refusal',good,{verificationError:true}],['interrupted install',good,{interrupt:true}],
]) {
  test(`no runnable fallback on ${name}`,async()=>{
    const r=await install(checksum,options);
    assert.equal(r.renames.length,0);assert.match(r.warnings[0],/verified fallback installation failed/);
    assert.ok(!r.warnings.join('\n').includes('private-canary-detail'));
    if(r.writes.length)assert.equal(r.removed.length,1);
    if(options.verificationError)assert.ok(!r.events.includes('chmod'));
    if(options.redirect)assert.equal(r.requests.length,1);
  });
}
for(const [name,options] of [['optional dependency present',{optional:true}],['download disabled',{skip:true}]]) {
  test(`no network or verifier calls with ${name}`,async()=>{
    const r=await install('',options);assert.equal(r.requests.length,0);assert.equal(r.writes.length,0);assert.ok(!r.events.includes('verify'));
  });
}
