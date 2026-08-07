import { constants as fsConstants } from 'node:fs';

export const DEFAULT_TIMEOUT_MS = 10_000;
export const MAX_CREDENTIAL_BYTES = 1_048_576;
export const MAX_RESPONSE_BYTES = 262_144;

export class UsageError extends Error {
  constructor(code, message) {
    super(message);
    this.name = 'UsageError';
    this.code = code;
  }
}

export function assertOpaque(value, label = 'credential') {
  if (
    typeof value !== 'string'
    || value.length < 8
    || value.length > 16_384
    || /[\s\u0000-\u001f\u007f]/u.test(value)
  ) {
    throw new UsageError('unsafe_credential', `The ${label} is missing or unsafe.`);
  }
  return value;
}

export function assertIdentifier(value, label) {
  if (
    typeof value !== 'string'
    || value.length < 1
    || value.length > 512
    || /[\s\u0000-\u001f\u007f]/u.test(value)
  ) {
    throw new UsageError('unsafe_credential', `The ${label} is missing or unsafe.`);
  }
  return value;
}

export async function readBoundedFile(filePath, {
  fsApi,
  maxBytes = MAX_CREDENTIAL_BYTES,
  missingMessage = 'The credential file was not found.',
} = {}) {
  try {
    const stat = await fsApi.stat(filePath);
    if (!stat.isFile?.() && stat.isFile !== undefined) {
      throw new UsageError('credential_file', missingMessage);
    }
    if (!Number.isSafeInteger(stat.size) || stat.size < 1 || stat.size > maxBytes) {
      throw new UsageError('credential_file', 'The credential file has an invalid size.');
    }
    const text = await fsApi.readFile(filePath, 'utf8');
    if (Buffer.byteLength(text, 'utf8') > maxBytes) {
      throw new UsageError('credential_file', 'The credential file has an invalid size.');
    }
    return { text, mode: stat.mode };
  } catch (error) {
    if (error instanceof UsageError) throw error;
    if (error?.code === 'ENOENT') {
      throw new UsageError('credential_file', missingMessage);
    }
    throw new UsageError('credential_file', 'The credential file could not be read safely.');
  }
}

export async function readCredentialJson(filePath, options) {
  const { text, mode } = await readBoundedFile(filePath, options);
  try {
    const value = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error();
    return { value, mode };
  } catch {
    throw new UsageError('credential_json', 'The credential file is not valid JSON.');
  }
}

export function hasBroadPermissions(mode) {
  return Number.isInteger(mode) && (mode & (fsConstants.S_IRWXG | fsConstants.S_IRWXO)) !== 0;
}

export function validateExactHttpsUrl(rawUrl, allowedUrls) {
  let url;
  try {
    url = new URL(rawUrl);
  } catch {
    throw new UsageError('endpoint_refused', 'The provider endpoint is not a valid URL.');
  }
  if (
    url.protocol !== 'https:'
    || url.username
    || url.password
    || url.hash
    || !allowedUrls.includes(url.href)
  ) {
    throw new UsageError('endpoint_refused', 'The provider endpoint was refused.');
  }
  return url;
}

export function validateTrustedBaseUrl(rawUrl, trustedOrigin) {
  let base;
  let trusted;
  try {
    base = new URL(rawUrl);
    trusted = new URL(trustedOrigin);
  } catch {
    throw new UsageError('untrusted_origin', 'The configured Codex fallback origin was not explicitly trusted.');
  }
  if (
    base.protocol !== 'https:'
    || base.username
    || base.password
    || base.search
    || base.hash
    || trusted.protocol !== 'https:'
    || trusted.username
    || trusted.password
    || trusted.search
    || trusted.hash
    || trusted.pathname !== '/'
    || base.origin !== trusted.origin
  ) {
    throw new UsageError('untrusted_origin', 'The configured Codex fallback origin was not explicitly trusted.');
  }
  const pathname = base.pathname.endsWith('/') ? base.pathname : `${base.pathname}/`;
  const target = new URL('api/codex/usage', `${base.origin}${pathname}`);
  if (target.origin !== base.origin) {
    throw new UsageError('endpoint_refused', 'The configured Codex fallback endpoint was refused.');
  }
  return target;
}

async function readResponseText(response, maxBytes) {
  const lengthHeader = response.headers?.get?.('content-length');
  if (lengthHeader && Number(lengthHeader) > maxBytes) {
    throw new UsageError('response_too_large', 'The provider response exceeded the safe size limit.');
  }

  if (response.body?.getReader) {
    const reader = response.body.getReader();
    const chunks = [];
    let total = 0;
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        total += value.byteLength;
        if (total > maxBytes) {
          await reader.cancel().catch(() => {});
          throw new UsageError('response_too_large', 'The provider response exceeded the safe size limit.');
        }
        chunks.push(value);
      }
    } finally {
      reader.releaseLock?.();
    }
    return new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks.map((chunk) => Buffer.from(chunk))));
  }

  const text = await response.text();
  if (Buffer.byteLength(text, 'utf8') > maxBytes) {
    throw new UsageError('response_too_large', 'The provider response exceeded the safe size limit.');
  }
  return text;
}

export async function requestJson({
  url,
  allowedUrls,
  fetchImpl,
  method = 'GET',
  headers = {},
  body,
  timeoutMs = DEFAULT_TIMEOUT_MS,
  maxBytes = MAX_RESPONSE_BYTES,
}) {
  const endpoint = validateExactHttpsUrl(url, allowedUrls);
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  let response;
  try {
    response = await fetchImpl(endpoint, {
      method,
      headers,
      body,
      redirect: 'manual',
      signal: controller.signal,
    });
  } catch {
    clearTimeout(timer);
    if (controller.signal.aborted) {
      throw new UsageError('timeout', 'The provider request timed out.');
    }
    throw new UsageError('network', 'The provider request failed.');
  }

  let text;
  try {
    if (response.status >= 300 && response.status < 400) {
      throw new UsageError('redirect_refused', 'The provider attempted an unsafe redirect.');
    }
    text = await readResponseText(response, maxBytes);
  } catch (error) {
    if (error instanceof UsageError) throw error;
    if (controller.signal.aborted) throw new UsageError('timeout', 'The provider request timed out.');
    throw new UsageError('network', 'The provider response could not be read safely.');
  } finally {
    clearTimeout(timer);
  }
  if (response.status < 200 || response.status >= 300) {
    return { status: response.status, value: undefined };
  }
  if (!text) {
    throw new UsageError('response_json', 'The provider returned an empty response.');
  }
  try {
    const value = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error();
    return { status: response.status, value };
  } catch {
    throw new UsageError('response_json', 'The provider returned unsupported JSON.');
  }
}

export function statusError(provider, status) {
  if (status === 401 || status === 403) {
    return new UsageError('unauthorized', `${provider} rejected the credential.`);
  }
  if (status === 429) {
    return new UsageError('rate_limited', `${provider} rate-limited the usage request.`);
  }
  return new UsageError('provider_status', `${provider} usage is unavailable (HTTP ${status}).`);
}

export function finitePercent(value) {
  const number = typeof value === 'string' && value.trim() ? Number(value) : value;
  return Number.isFinite(number) && number >= 0 && number <= 100 ? number : undefined;
}

export function finiteNonNegative(value) {
  const number = typeof value === 'string' && value.trim() ? Number(value) : value;
  return Number.isFinite(number) && number >= 0 ? number : undefined;
}

export function resetText(value) {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    const milliseconds = value > 10_000_000_000 ? value : value * 1000;
    const date = new Date(milliseconds);
    return Number.isNaN(date.valueOf()) ? undefined : date.toISOString();
  }
  if (typeof value === 'string' && value.length <= 128) {
    const date = new Date(value);
    return Number.isNaN(date.valueOf()) ? undefined : date.toISOString();
  }
  return undefined;
}

function displayNumber(value) {
  return Number.isInteger(value) ? String(value) : value.toFixed(1).replace(/\.0$/, '');
}

export function formatUsageReport(provider, windows) {
  const lines = [`${provider} usage allowance`];
  for (const window of windows) {
    const details = [];
    if (window.unlimited === true) {
      details.push('unlimited');
    } else {
      if (window.usedPercent !== undefined) details.push(`${displayNumber(window.usedPercent)}% used`);
      if (window.remainingPercent !== undefined) details.push(`${displayNumber(window.remainingPercent)}% available`);
      if (window.remaining !== undefined) details.push(`${displayNumber(window.remaining)} remaining`);
      if (window.entitlement !== undefined) details.push(`${displayNumber(window.entitlement)} total`);
    }
    if (window.resetAt) details.push(`resets ${window.resetAt}`);
    lines.push(`- ${window.label}: ${details.join('; ')}`);
  }
  return lines.join('\n');
}

export async function runSafely(action, { stdout = process.stdout, stderr = process.stderr } = {}) {
  try {
    const output = await action();
    stdout.write(`${output}\n`);
    return 0;
  } catch (error) {
    if (error instanceof UsageError) {
      stderr.write(`Usage check failed [${error.code}]: ${error.message}\n`);
    } else {
      stderr.write('Usage check failed [unexpected]: The check stopped safely.\n');
    }
    return 1;
  }
}
