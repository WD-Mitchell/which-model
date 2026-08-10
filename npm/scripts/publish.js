#!/usr/bin/env node
"use strict";

// Publish all npm packages for a release.
//
// Usage: node npm/scripts/publish.js [--dry-run]
//
// Requires NPM_TOKEN in the environment. Publishes the five platform packages
// first, then the launcher, so an interrupted run never leaves the launcher
// pinned to missing optional dependencies.

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const DRY_RUN = process.argv.includes("--dry-run");
const npmRoot = path.join(__dirname, "..");

function main() {
  if (!DRY_RUN && !process.env.NPM_TOKEN) {
    console.error("NPM_TOKEN is required to publish");
    process.exit(2);
  }

  const launcherManifest = read(path.join(npmRoot, "which-model", "package.json"));
  const version = launcherManifest.version;
  const dirs = Object.keys(launcherManifest.optionalDependencies).map(
    (name) => name.replace("@wdm-uk/", "wdm-uk-")
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
  console.log(`${DRY_RUN ? "[dry-run] would publish" : "publishing"} ${name}@${version} from ${dir}`);
  const args = ["publish", "--access", "public"];
  if (DRY_RUN) args.push("--dry-run");
  const res = spawnSync("npm", args, { cwd: dir, stdio: "inherit", env: process.env });
  if (res.status !== 0) {
    console.error(`npm publish failed for ${name} (exit ${res.status})`);
    process.exit(res.status || 1);
  }
}

function read(p) {
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

main();
