"use strict";

// Postinstall fallback for @wdm-uk/which-model.
//
// Normal installs get the binary from the platform-specific optional
// dependency (e.g. @wdm-uk/which-model-darwin-arm64). When that package could not be
// installed — a registry mirror without optional dependencies, an offline
// cache that omits it, or a platform filter — this script downloads the
// matching binary from the GitHub releases of WD-Mitchell/which-model.
//
// The fallback is best-effort: any failure prints a warning and exits 0 so
// installation itself never breaks; bin.js reports a clear error at runtime
// if no binary ends up available.

const fs = require("fs");
const path = require("path");
const https = require("https");
const crypto = require("crypto");
const { validateManifest, verifyArtifact } = require("./verify-release");

const REPO = "WD-Mitchell/which-model";
const VERSION = require("./package.json").version;

const NODE_OS = process.platform === "win32" ? "windows" : process.platform;
const PLATFORM = { darwin: "darwin", linux: "linux", windows: "windows" }[NODE_OS];
const ARCH = { arm64: "arm64", x64: "x64" }[process.arch];
function main() {
  const name = `@wdm-uk/which-model-${NODE_OS}-${process.arch}`;
  const binName = process.platform === "win32" ? "which-model.exe" : "which-model";

  // Already satisfied by the optional dependency?
  try {
    const pkgJson = require.resolve(`${name}/package.json`, {
      paths: [__dirname, path.dirname(__dirname)],
    });
    if (fs.existsSync(path.join(path.dirname(pkgJson), binName))) {
      return; // nothing to do
    }
  } catch (_) {
    // fall through to the download fallback
  }

  if (!PLATFORM || !ARCH) {
    console.warn(
      `which-model postinstall: unsupported platform ${process.platform}/${process.arch}; skipping fallback download.`
    );
    return;
  }
  if (process.env.WHICH_MODEL_SKIP_DOWNLOAD === "1") {
    console.warn("which-model postinstall: WHICH_MODEL_SKIP_DOWNLOAD=1, skipping fallback download.");
    return;
  }

  const asset = `which-model-${PLATFORM}-${ARCH}${process.platform === "win32" ? ".exe" : ""}`;
  const tag = `v${VERSION}`;
  const base = `https://github.com/${REPO}/releases/download/${tag}`;

  let stage;
  Promise.resolve()
    .then(async () => {
      const policy = validateManifest(require("./release-policy.json"), VERSION);
      const expected = policy.artifacts.find((item) => item.name === asset);
      if (!expected) throw new Error("platform is absent from the verified release policy");
      const checksums = await download(`${base}/checksums.txt`, 0, 1024 * 1024);
      const want = parseChecksum(checksums.toString("utf8"), asset);
      if (want !== expected.sha256) throw new Error(`no matching checksum entry for ${asset}`);
      const body = await download(`${base}/${asset}`, 0, 128 * 1024 * 1024);
      const got = crypto.createHash("sha256").update(body).digest("hex");
      if (got !== want) throw new Error(`checksum mismatch for ${asset}`);
      const bundle = await download(`${base}/provenance.jsonl`, 0, 16 * 1024 * 1024);
      // A sibling staging directory keeps unverified bytes out of the launcher
      // path and allows atomic final renames on the destination filesystem.
      stage = fs.mkdtempSync(path.join(__dirname, ".which-model-install-"));
      const candidate = path.join(stage, "candidate.download");
      const evidence = path.join(stage, "provenance.jsonl");
      fs.writeFileSync(candidate, body, { mode: 0o600, flag: "wx" });
      fs.writeFileSync(evidence, bundle, { mode: 0o600, flag: "wx" });
      verifyArtifact({ artifact: candidate, bundle: evidence, sha256: want,
        sourceDigest: policy.source_digest, sourceRef: policy.source_ref });
      const receipt = path.join(stage, "receipt.json");
      fs.writeFileSync(receipt, JSON.stringify({ version: VERSION, sha256: want,
        source_digest: policy.source_digest, source_ref: policy.source_ref }), { mode: 0o600, flag: "wx" });
      fs.chmodSync(candidate, 0o755);
      // The receipt may precede the binary: the launcher also checks its digest.
      fs.renameSync(receipt, path.join(__dirname, ".which-model-verification.json"));
      fs.renameSync(candidate, path.join(__dirname, binName));
      console.warn(`which-model postinstall: installed ${asset} from verified GitHub release ${tag} (fallback).`);
    })
    .catch(() => {
      console.warn(
        "which-model postinstall: verified fallback installation failed; no new release binary was installed. Install the matching platform package or use a trusted GitHub CLI 2.97.0+ with valid release evidence and network access."
      );
    })
    .finally(() => {
      if (stage) {
        try { fs.rmSync(stage, { recursive: true, force: true }); }
        catch (_) { console.warn("which-model postinstall: could not remove private staging data; review .which-model-install-* directories."); }
      }
    });
}

function parseChecksum(text, filename) {
  for (const line of text.split(/\r?\n/)) {
    // sha256sum format: "<hex>  <filename>"
    const m = line.match(/^([0-9a-f]{64})\s+(?:\*)?(\S+)$/);
    if (m && m[2] === filename) return m[1];
  }
  return null;
}

function download(url, redirects = 0, maxBytes = 128 * 1024 * 1024) {
  return new Promise((resolve, reject) => {
    if (redirects > 5) return reject(new Error("too many redirects"));
    const parsed = new URL(url);
    if (parsed.protocol !== "https:" || parsed.username || parsed.password || parsed.hash ||
        (parsed.port && parsed.port !== "443") ||
        !["github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"].includes(parsed.hostname)) {
      return reject(new Error("unapproved release download destination"));
    }
    const request = https
      .get(url, { headers: { "user-agent": `which-model-npm-install/${VERSION}` } }, (res) => {
        if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location) {
          res.resume();
          try {
            return download(new URL(res.headers.location, url).toString(), redirects + 1, maxBytes).then(resolve, reject);
          } catch (_) { return reject(new Error("invalid release redirect")); }
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        const chunks = [];
        let size = 0;
        res.on("data", (c) => {
          size += c.length;
          if (size > maxBytes) { res.destroy(); reject(new Error("release download is too large")); }
          else chunks.push(c);
        });
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
        res.on("aborted", () => reject(new Error("release download interrupted")));
      })
      .on("error", reject);
    request.setTimeout(30000, () => { request.destroy(); reject(new Error("release download timed out")); });
  });
}

main();
