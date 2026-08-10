#!/usr/bin/env node
"use strict";

// Platform launcher for the which-model Go binary.
//
// The actual binary is shipped in a platform-specific package selected via
// optionalDependencies (esbuild / turbo style). Each platform package contains
// the executable at its root as `which-model` (or `which-model.exe` on Windows).
// This wrapper locates it and execs it, forwarding stdio, signals, and the
// exit code, so the installed command behaves exactly like the native binary.

const fs = require("fs");
const path = require("path");
const { spawnSync, spawn } = require("child_process");

const WINDOWS = process.platform === "win32";
const BINARY_NAME = WINDOWS ? "which-model.exe" : "which-model";

function platformPackage() {
  // Node's process.platform is "win32" on Windows, but the package names use
  // the conventional "windows" spelling (e.g. @wdm-uk/which-model-windows-x64).
  const os = WINDOWS ? "windows" : process.platform;
  return `@wdm-uk/which-model-${os}-${process.arch}`;
}

function resolveBinaryFromPackage(name) {
  // Locate the platform package's directory by requiring its package.json
  // relative to this file. This works in node_modules layouts produced by
  // npm, pnpm, yarn, and bun (all resolve sibling optional dependencies
  // upward from the requiring module).
  try {
    const pkgJson = require.resolve(`${name}/package.json`, {
      paths: [__dirname, path.dirname(__dirname)],
    });
    const root = path.dirname(pkgJson);
    const bin = path.join(root, BINARY_NAME);
    if (fs.existsSync(bin)) return bin;
  } catch (_) {
    // fall through to next strategy
  }
  return null;
}

function resolveBinary() {
  const name = platformPackage();

  // 1. Platform package installed as an optional dependency.
  const fromPkg = resolveBinaryFromPackage(name);
  if (fromPkg) return fromPkg;

  // 2. Fallback installed by the postinstall script into our own node_modules
  //    directory or the package's bin directory (used when the optional dep
  //    could not be installed and install.js downloaded a GitHub release asset).
  const local = path.join(__dirname, BINARY_NAME);
  if (fs.existsSync(local)) return local;

  return null;
}

function fail(message) {
  console.error(`which-model: ${message}`);
  console.error(
    `Supported targets: darwin-arm64, darwin-x64, linux-arm64, linux-x64, windows-x64.`
  );
  console.error(
    `Reinstall the package, or report the issue at https://github.com/WD-Mitchell/which-model/issues`
  );
  process.exit(2);
}

function main() {
  const bin = resolveBinary();
  if (!bin) {
    fail(
      `could not find the which-model binary for platform "${platformPackage()}".`
    );
  }

  try {
    fs.accessSync(bin, fs.constants.X_OK);
  } catch (_) {
    fail(`binary at ${bin} is not executable.`);
  }

  const args = process.argv.slice(2);

  // spawnSync on the same stdio gives us a faithful pass-through and returns
  // the child's exit status. Signals are not forwarded to the child with
  // spawnSync, so on non-Windows we prefer a spawn + signal forwarding loop
  // and fall back to spawnSync otherwise.
  if (WINDOWS) {
    const res = spawnSync(bin, args, { stdio: "inherit" });
    process.exit(res.status === null ? 1 : res.status);
  }

  const child = spawn(bin, args, { stdio: "inherit" });

  const forward = (sig) => () => {
    try {
      child.kill(sig);
    } catch (_) {
      // child already exited
    }
  };
  const sigint = forward("SIGINT");
  const sigterm = forward("SIGTERM");
  process.on("SIGINT", sigint);
  process.on("SIGTERM", sigterm);

  child.on("error", (err) => {
    console.error(`which-model: failed to start binary: ${err.message}`);
    process.exit(2);
  });

  child.on("close", (code, signal) => {
    process.off("SIGINT", sigint);
    process.off("SIGTERM", sigterm);
    if (code === null) {
      // terminated by signal; mirror the convention used by shells
      process.exit(signal === "SIGINT" || signal === "SIGTERM" ? 130 : 1);
    }
    process.exit(code);
  });
}

main();
