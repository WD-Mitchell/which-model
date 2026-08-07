#!/usr/bin/env node
import { pathToFileURL } from 'node:url';
import { checkCodexUsage } from '../lib/codex.mjs';
import { runSafely, UsageError } from '../lib/core.mjs';

function parseArgs(args) {
  if (!args.length) return {};
  if (args.length === 2 && args[0] === '--trust-configured-origin') return { trustedOrigin: args[1] };
  throw new UsageError('arguments', 'Use --trust-configured-origin <https-origin> only when you trust the configured Codex provider.');
}

export async function main(args = process.argv.slice(2), io) {
  return runSafely(() => checkCodexUsage(parseArgs(args)), io);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await main();
}
