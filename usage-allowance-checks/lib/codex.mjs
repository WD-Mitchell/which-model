import path from 'node:path';
import os from 'node:os';
import fs from 'node:fs/promises';
import {
  UsageError,
  assertIdentifier,
  assertOpaque,
  finiteNonNegative,
  finitePercent,
  formatUsageReport,
  readBoundedFile,
  readCredentialJson,
  requestJson,
  resetText,
  statusError,
  validateTrustedBaseUrl,
} from './core.mjs';

export const CODEX_USAGE_URL = 'https://chatgpt.com/backend-api/wham/usage';
const FALLBACK_STATUSES = new Set([404, 405, 410, 501]);

export function parseCodexConfig(text) {
  let section = '';
  let activeProvider;
  let rootBaseUrl;
  const providerUrls = new Map();
  for (const rawLine of text.split(/\r?\n/u)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const sectionMatch = line.match(/^\[model_providers\.([A-Za-z0-9_-]+)\]$/u);
    if (sectionMatch) {
      section = sectionMatch[1];
      continue;
    }
    if (line.startsWith('[')) {
      section = '__other__';
      continue;
    }
    const assignment = line.match(/^(model_provider|base_url)\s*=\s*(["'])(.*?)\2\s*(?:#.*)?$/u);
    if (!assignment) continue;
    const [, key, , value] = assignment;
    if (!section && key === 'model_provider') activeProvider = value;
    if (!section && key === 'base_url') rootBaseUrl = value;
    if (section && section !== '__other__' && key === 'base_url') providerUrls.set(section, value);
  }
  return (activeProvider && providerUrls.get(activeProvider)) || rootBaseUrl;
}

async function optionalConfiguredBaseUrl(configPath, fsApi) {
  try {
    const { text } = await readBoundedFile(configPath, { fsApi, missingMessage: 'Codex config was not found.' });
    return parseCodexConfig(text);
  } catch (error) {
    if (error instanceof UsageError && error.code === 'credential_file') return undefined;
    throw error;
  }
}

export async function loadCodexCredential({
  authPath = path.join(os.homedir(), '.codex', 'auth.json'),
  configPath = path.join(os.homedir(), '.codex', 'config.toml'),
  fsApi = fs,
} = {}) {
  const { value } = await readCredentialJson(authPath, {
    fsApi,
    missingMessage: 'Codex credentials were not found; sign in with Codex first.',
  });
  const tokens = value.tokens ?? value.auth ?? value;
  const token = assertOpaque(tokens.access_token ?? tokens.accessToken, 'Codex access token');
  const accountId = assertIdentifier(
    tokens.account_id ?? tokens.accountId ?? value.account_id ?? value.chatgpt_account_id,
    'ChatGPT account identifier',
  );
  const configuredBaseUrl = value.base_url
    ?? value.baseUrl
    ?? value.openai_base_url
    ?? await optionalConfiguredBaseUrl(configPath, fsApi);
  return { token, accountId, configuredBaseUrl };
}

export function normalizeCodexUsage(value) {
  const rateLimit = value.rate_limit ?? value.rateLimit ?? value;
  const known = [
    ['primary_window', 'primary window'],
    ['secondary_window', 'secondary window'],
  ];
  const windows = [];
  for (const [key, label] of known) {
    const camelKey = key.replace(/_([a-z])/gu, (_, letter) => letter.toUpperCase());
    const source = rateLimit[key] ?? rateLimit[camelKey];
    if (!source || typeof source !== 'object') continue;
    const usedPercent = finitePercent(source.used_percent ?? source.usedPercent);
    if (usedPercent === undefined) continue;
    windows.push({
      label,
      usedPercent,
      remainingPercent: 100 - usedPercent,
      resetAt: resetText(source.reset_at ?? source.resetAt),
    });
  }
  const credits = value.credits;
  const balance = credits && finiteNonNegative(credits.balance);
  if (balance !== undefined) windows.push({ label: 'credits', remaining: balance });
  if (!windows.length) {
    throw new UsageError('unsupported_response', 'Codex returned an unsupported usage shape.');
  }
  return windows;
}

async function fetchCodex(url, allowedUrls, credential, fetchImpl, timeoutMs) {
  return requestJson({
    url,
    allowedUrls,
    fetchImpl,
    timeoutMs,
    headers: {
      accept: 'application/json',
      authorization: `Bearer ${credential.token}`,
      'chatgpt-account-id': credential.accountId,
    },
  });
}

export async function checkCodexUsage({
  fetchImpl = globalThis.fetch,
  credentialOptions,
  trustedOrigin,
  timeoutMs,
} = {}) {
  const credential = await loadCodexCredential(credentialOptions);
  const primary = await fetchCodex(CODEX_USAGE_URL, [CODEX_USAGE_URL], credential, fetchImpl, timeoutMs);
  if (primary.status === 200) {
    return formatUsageReport('Codex', normalizeCodexUsage(primary.value));
  }
  if (!FALLBACK_STATUSES.has(primary.status)) throw statusError('Codex', primary.status);
  if (!credential.configuredBaseUrl) {
    throw new UsageError('fallback_unavailable', 'Codex did not advertise a configured fallback endpoint.');
  }
  const fallback = validateTrustedBaseUrl(credential.configuredBaseUrl, trustedOrigin);
  const result = await fetchCodex(fallback.href, [fallback.href], credential, fetchImpl, timeoutMs);
  if (result.status !== 200) throw statusError('Codex fallback', result.status);
  return formatUsageReport('Codex', normalizeCodexUsage(result.value));
}
