#!/usr/bin/env node
import { pathToFileURL } from 'node:url';
import { checkCopilotUsage } from '../lib/copilot.mjs';
import { runSafely, UsageError } from '../lib/core.mjs';

function parseArgs(args) {
  const allowed = new Set(['--login', '--show-identity']);
  if (args.some((arg) => !allowed.has(arg))) {
    throw new UsageError('arguments', 'GitHub Copilot usage accepts only --login and --show-identity.');
  }
  return { login: args.includes('--login'), showIdentity: args.includes('--show-identity') };
}

export async function main(args = process.argv.slice(2), io) {
  return runSafely(() => checkCopilotUsage({
    ...parseArgs(args),
    writeLogin: (message) => (io?.stdout ?? process.stdout).write(`${message}\n`),
  }), io);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await main();
}
