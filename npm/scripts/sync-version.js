#!/usr/bin/env node
"use strict";

// Stamp VERSION into every npm package manifest.
//
// Usage: node npm/scripts/sync-version.js <version>
//
// Called from the release workflow so the launcher and all five platform
// packages publish with a single identical version. Keeps package.json files
// in git at a stable placeholder otherwise.

const fs = require("fs");
const path = require("path");

const semverRe = /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/;

function main() {
  const version = process.argv[2];
  if (!version || !semverRe.test(version)) {
    console.error(`usage: sync-version.js <semver>`);
    process.exit(2);
  }

  const npmRoot = path.join(__dirname, "..");
  const launcher = path.join(npmRoot, "which-model", "package.json");
  const launcherManifest = read(launcher);

  const platforms = Object.keys(launcherManifest.optionalDependencies).map(
    (name) => name.replace("@wdm-uk/", "")
  );

  // Launcher: own version + pinned optionalDependencies.
  launcherManifest.version = version;
  for (const dep of Object.keys(launcherManifest.optionalDependencies)) {
    launcherManifest.optionalDependencies[dep] = version;
  }
  write(launcher, launcherManifest);
  console.log(`stamped ${launcher} -> ${version}`);

  for (const dir of platforms) {
    const manifestPath = path.join(npmRoot, dir, "package.json");
    if (!fs.existsSync(manifestPath)) {
      console.error(`missing platform package: ${manifestPath}`);
      process.exit(1);
    }
    const manifest = read(manifestPath);
    manifest.version = version;
    write(manifestPath, manifest);
    console.log(`stamped ${manifestPath} -> ${version}`);
  }
}

function read(p) {
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

function write(p, obj) {
  fs.writeFileSync(p, JSON.stringify(obj, null, 2) + "\n");
}

main();
