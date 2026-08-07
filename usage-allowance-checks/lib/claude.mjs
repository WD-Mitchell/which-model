import path from 'node:path';
import os from 'node:os';
import fs from 'node:fs/promises';
import {
  UsageError,
  assertOpaque,
  finitePercent,
  formatUsageReport,
  hasBroadPermissions,
  readCredentialJson,
  requestJson,
  resetText,
  statusError,
} from './core.mjs';

export const CLAUDE_USAGE_URL = 'https://api.anthropic.com/api/oauth/usage';

export async function loadClaudeCredential({
  filePath = path.join(os.homedir(), '.claude', 'credentials.json'),
  fsApi = fs,
  now = Date.now(),
} = {}) {
  const { value, mode } = await readCredentialJson(filePath, {
    fsApi,
    missingMessage: 'Claude credentials were not found; sign in with Claude Code first.',
  });
  const oauth = value.claudeAiOauth ?? value.oauth ?? value;
  const token = assertOpaque(oauth.accessToken ?? oauth.access_token, 'Claude access token');
  const expiresAt = oauth.expiresAt ?? oauth.expires_at;
  if (expiresAt !== undefined) {
    const expiry = typeof expiresAt === 'number' ? expiresAt : Date.parse(expiresAt);
    const milliseconds = expiry > 10_000_000_000 ? expiry : expiry * 1000;
    if (!Number.isFinite(milliseconds) || milliseconds <= now) {
      throw new UsageError('expired_credential', 'The Claude access token is expired.');
    }
  }
  return { token, broadPermissions: hasBroadPermissions(mode) };
}

export function normalizeClaudeUsage(value) {
  const known = [
    ['five_hour', 'five hour'],
    ['seven_day', 'seven day'],
    ['seven_day_sonnet', 'seven day Sonnet'],
    ['seven_day_opus', 'seven day Opus'],
    ['seven_day_oauth_apps', 'seven day OAuth apps'],
  ];
  const windows = [];
  for (const [key, label] of known) {
    const source = value[key];
    if (!source || typeof source !== 'object') continue;
    const usedPercent = finitePercent(source.utilization ?? source.used_percent);
    if (usedPercent === undefined) continue;
    windows.push({
      label,
      usedPercent,
      remainingPercent: 100 - usedPercent,
      resetAt: resetText(source.resets_at ?? source.reset_at),
    });
  }
  if (!windows.length) {
    throw new UsageError('unsupported_response', 'Claude returned an unsupported usage shape.');
  }
  return windows;
}

export async function checkClaudeUsage({
  fetchImpl = globalThis.fetch,
  credentialOptions,
  timeoutMs,
  warn = (message) => process.stderr.write(`${message}\n`),
} = {}) {
  const { token, broadPermissions } = await loadClaudeCredential(credentialOptions);
  if (broadPermissions) {
    warn('Warning: Claude credential permissions are broader than 0600; review them before continuing.');
  }
  const result = await requestJson({
    url: CLAUDE_USAGE_URL,
    allowedUrls: [CLAUDE_USAGE_URL],
    fetchImpl,
    timeoutMs,
    headers: {
      accept: 'application/json',
      authorization: `Bearer ${token}`,
      'anthropic-beta': 'oauth-2025-04-20',
    },
  });
  if (result.status !== 200) throw statusError('Claude', result.status);
  return formatUsageReport('Claude', normalizeClaudeUsage(result.value));
}
