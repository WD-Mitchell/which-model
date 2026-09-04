"use strict";

const assert = require("node:assert/strict");
const { createHash } = require("node:crypto");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "install.js"), "utf8");
const binary = Buffer.from([0, 255, 128, 10, 13, 42]);
const digest = createHash("sha256").update(binary).digest("hex");
const asset = "which-model-linux-x64";

// Exercise the actual postinstall entry point without network or disk writes.
async function install(checksum, { optional = false, skip = false, status = 200 } = {}) {
  const requests = [], writes = [], warnings = [];
  const fakeRequire = (name) => {
    if (name === "./package.json") return { version: "1.2.3" };
    if (name === "fs") return {
      existsSync: () => optional,
      writeFileSync: (...args) => writes.push(args),
    };
    if (name === "https") return {
      get(url, _options, callback) {
        requests.push(url);
        const request = new EventEmitter();
        queueMicrotask(() => {
          const response = new EventEmitter();
          Object.assign(response, { statusCode: status, headers: {}, resume() {} });
          callback(response);
          response.emit("data", url.endsWith("checksums.txt") ? Buffer.from(checksum) : binary);
          response.emit("end");
        });
        return request;
      },
    };
    return require(name);
  };
  fakeRequire.resolve = () => {
    if (!optional) throw new Error("optional dependency missing");
    return "/optional/package.json";
  };
  vm.runInNewContext(source, {
    require: fakeRequire, __dirname: "/package", Buffer, URL,
    process: { platform: "linux", arch: "x64", env: { WHICH_MODEL_SKIP_DOWNLOAD: skip ? "1" : "0" } },
    console: { warn: (message) => warnings.push(message) },
  });
  await new Promise(setImmediate);
  return { requests, writes, warnings };
}

for (const [name, checksum] of [
  ["LF", `${digest}  ${asset}\n`],
  ["CRLF and binary marker", `${digest} *${asset}\r\n`],
]) {
  test(`fallback validates ${name} checksums and preserves binary bytes`, async () => {
    const result = await install(checksum);
    assert.equal(result.requests.length, 2);
    assert.equal(result.writes.length, 1, result.warnings.join("\n"));
    assert.equal(result.writes[0][0], "/package/which-model");
    assert.deepEqual(result.writes[0][1], binary);
    assert.equal(result.writes[0][2].mode, 0o755);
    assert.match(result.warnings[0], /installed .* fallback|installed .*\(fallback\)/);
  });
}

for (const [name, checksum, options] of [
  ["missing checksum", `${digest}  other-file\n`, {}],
  ["mismatched checksum", `${"0".repeat(64)}  ${asset}\n`, {}],
  ["malformed checksum", `invalid  ${asset}\n`, {}],
  ["HTTP failure", "", { status: 404 }],
]) {
  test(`fallback fails without writing on ${name}`, async () => {
    const result = await install(checksum, options);
    assert.equal(result.writes.length, 0);
    assert.match(result.warnings[0], /fallback download failed/);
  });
}

for (const [name, options] of [["optional dependency present", { optional: true }], ["download disabled", { skip: true }]]) {
  test(`fallback makes no requests with ${name}`, async () => {
    const result = await install("", options);
    assert.equal(result.requests.length, 0);
    assert.equal(result.writes.length, 0);
  });
}
