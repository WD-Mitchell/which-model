#!/usr/bin/env node
"use strict";

// Publish all npm packages for a release.
//
// Usage: node npm/scripts/publish.js [--dry-run]
//
// Authentication: npm Trusted Publishing (OIDC) inside GitHub Actions — no
// long-lived token. Locally, `npm login` or an .npmrc authToken works (this
// is the bootstrap path for the first publish of a brand-new package). As a
// last resort, set NPM_TOKEN and the workflow writes an .npmrc auth line
// before invoking this script. Publishes the five platform packages first,
// then the launcher, so an interrupted run never leaves the launcher pinned
// to missing optional dependencies.

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const DRY_RUN = process.argv.includes("--dry-run");
const npmRoot = path.join(__dirname, "..");

function main() {
  if (!DRY_RUN && !process.env.NPM_TOKEN && !process.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN && !npmAuthenticated()) {
    console.error(
      "publish requires npm Trusted Publishing (GitHub Actions with id-token: write), NPM_TOKEN, or `npm login`"
    );
    process.exit(2);
  }

  const launcherManifest = read(path.join(npmRoot, "which-model", "package.json"));
  const version = launcherManifest.version;
  const dirs = Object.keys(launcherManifest.optionalDependencies).map(
    (name) => name.replace("@wdm-uk/", "")
  );

  for (const dir of dirs) {
    const dirPath = path.join(npmRoot, dir);
    const manifest = read(path.join(dirPath, "package.json"));
    if (manifest.version !== version) {
      console.error(`${dir}: version ${manifest.version} != launcher ${version}`);
      process.exit(1);
    }
    const bin = manifest.files[0];
    const binPath = path.join(dirPath, bin);
    if (!fs.existsSync(binPath)) {
      console.error(`${dir}: binary ${bin} missing — run the release build first`);
      process.exit(1);
    }
    // Executable bit must survive npm pack.
    fs.chmodSync(binPath, 0o755);
  }

  const order = [...dirs, "which-model"];
  for (const dir of order) {
    publish(path.join(npmRoot, dir), version);
  }
  console.log(DRY_RUN ? "dry-run complete" : `published ${order.length} packages at v${version}`);
}

function publish(dir, version) {
  const name = read(path.join(dir, "package.json")).name;
  if (!DRY_RUN && versionExists(name, version)) {
    console.log(`skipping ${name}@${version} (already published)`);
    return;
  }
  console.log(`${DRY_RUN ? "[dry-run] would publish" : "publishing"} ${name}@${version} from ${dir}`);
  const args = ["publish", "--access", "public"];
  // Prerelease versions (e.g. 1.1.0-beta.0) tag as `beta` so a beta release
  // never moves the `latest` dist-tag and is installed via
  // `pnpm add -g @wdm-uk/which-model@beta`.
  if (/-/.test(version)) args.push("--tag", "beta");
  if (DRY_RUN) args.push("--dry-run");
  const res = spawnSync("npm", args, { cwd: dir, stdio: "inherit", env: process.env });
  if (res.status !== 0) {
    console.error(`npm publish failed for ${name} (exit ${res.status})`);
    process.exit(res.status || 1);
  }
}

// versionExists reports whether name@version is already on the registry, so
// an interrupted multi-package release can be re-run safely.
function versionExists(name, version) {
  const res = spawnSync("npm", ["view", `${name}@${version}`, "version"], {
    encoding: "utf8",
    env: process.env,
  });
  return res.status === 0 && res.stdout.includes(version);
}

function read(p) {
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

// npmAuthenticated reports whether the current environment can publish.
// `npm whoami` covers ~/.npmrc sessions (login) and authTokens; it does NOT
// see OIDC (trusted publishing authenticates at publish time), so the caller
// checks ACTIONS_ID_TOKEN_REQUEST_TOKEN separately.
function npmAuthenticated() {
  return spawnSync("npm", ["whoami"], { stdio: "ignore", env: process.env }).status === 0;
}

main();
