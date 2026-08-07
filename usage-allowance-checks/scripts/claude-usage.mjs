#!/usr/bin/env node
import { pathToFileURL } from 'node:url';
import { checkClaudeUsage } from '../lib/claude.mjs';
import { runSafely, UsageError } from '../lib/core.mjs';

export async function main(args = process.argv.slice(2), io) {
  return runSafely(async () => {
    if (args.length) throw new UsageError('arguments', 'Claude usage accepts no arguments.');
    return checkClaudeUsage({ warn: (message) => (io?.stderr ?? process.stderr).write(`${message}\n`) });
  }, io);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await main();
}
