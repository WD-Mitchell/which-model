"use strict";

// Postinstall fallback for @wdm-uk/which-model.
//
// Normal installs get the binary from the platform-specific optional
// dependency (e.g. @wdm-uk/darwin-arm64). When that package could not be
// installed — a registry mirror without optional dependencies, an offline
// cache that omits it, or a platform filter — this script downloads the
// matching binary from the GitHub releases of WD-Mitchell/which-model.
//
// The fallback is best-effort: any failure prints a warning and exits 0 so
// installation itself never breaks; bin.js reports a clear error at runtime
// if no binary ends up available.

const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const crypto = require("crypto");

const REPO = "WD-Mitchell/which-model";
const VERSION = require("./package.json").version;

const NODE_OS = process.platform === "win32" ? "windows" : process.platform;
const PLATFORM = { darwin: "darwin", linux: "linux", windows: "windows" }[NODE_OS];
const ARCH = { arm64: "arm64", x64: "x64" }[process.arch];

function main() {
  const name = `@wdm-uk/${NODE_OS}-${process.arch}`;
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

  download(`${base}/checksums.txt`)
    .then((checksums) => download(`${base}/${asset}`).then((body) => ({ checksums, body })))
    .then(({ checksums, body }) => {
      const want = parseChecksum(checksums, asset);
      if (want === null) {
        throw new Error(`no checksum entry for ${asset} in checksums.txt`);
      }
      const got = crypto.createHash("sha256").update(body).digest("hex");
      if (got !== want) {
        throw new Error(`checksum mismatch for ${asset}: got ${got}`);
      }
      const dest = path.join(__dirname, binName);
      fs.writeFileSync(dest, body, { mode: 0o755 });
      console.warn(`which-model postinstall: installed ${asset} from GitHub release ${tag} (fallback).`);
    })
    .catch((err) => {
      console.warn(
        `which-model postinstall: fallback download failed (${err.message}); the platform package was not installed and no release binary was fetched.`
      );
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

function download(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 5) return reject(new Error("too many redirects"));
    https
      .get(url, { headers: { "user-agent": `which-model-npm-install/${VERSION}` } }, (res) => {
        if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location) {
          return download(new URL(res.headers.location, url).toString(), redirects + 1).then(resolve, reject);
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

main();
