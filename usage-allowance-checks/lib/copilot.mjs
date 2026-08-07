import { execFile as execFileCallback } from 'node:child_process';
import { promisify } from 'node:util';
import {
  UsageError,
  assertOpaque,
  finiteNonNegative,
  finitePercent,
  formatUsageReport,
  requestJson,
  resetText,
  statusError,
} from './core.mjs';

export const GITHUB_DEVICE_CODE_URL = 'https://github.com/login/device/code';
export const GITHUB_DEVICE_TOKEN_URL = 'https://github.com/login/oauth/access_token';
export const GITHUB_USER_URL = 'https://api.github.com/user';
export const COPILOT_USAGE_URL = 'https://api.github.com/copilot_internal/user';
export const COPILOT_CLIENT_ID = 'Iv1.b507a08c87ecfe98';

const execFile = promisify(execFileCallback);
const API_VERSION = '2025-04-01';

export async function defaultCommandRunner(command, args) {
  try {
    const result = await execFile(command, args, {
      encoding: 'utf8',
      maxBuffer: 32_768,
      timeout: 3_000,
      windowsHide: true,
    });
    return result.stdout;
  } catch {
    return undefined;
  }
}

function tokenCandidate(output) {
  const candidate = output.endsWith('\r\n')
    ? output.slice(0, -2)
    : output.endsWith('\n')
      ? output.slice(0, -1)
      : output;
  return assertOpaque(candidate, 'GitHub access token');
}

export async function discoverCopilotToken({
  runCommand = defaultCommandRunner,
  validateToken,
} = {}) {
  if (typeof validateToken !== 'function') {
    throw new UsageError('identity_validation', 'GitHub token discovery requires identity validation.');
  }
  const sources = [
    ['git', ['config', '--global', '--get', 'github.copilot.oauthToken']],
    ['git', ['config', '--system', '--get', 'github.copilot.oauthToken']],
    ['gh', ['auth', 'token', '--hostname', 'github.com']],
  ];
  for (const [command, args] of sources) {
    const output = await runCommand(command, args);
    if (output === undefined || output === '') continue;
    let candidate;
    try {
      candidate = tokenCandidate(output);
    } catch (error) {
      if (error instanceof UsageError && error.code === 'unsafe_credential') continue;
      throw error;
    }
    try {
      return { token: candidate, validation: await validateToken(candidate) };
    } catch (error) {
      if (error instanceof UsageError && ['unauthorized', 'identity_response'].includes(error.code)) continue;
      throw error;
    }
  }
  return undefined;
}

function githubIdentityHeaders(token) {
  return {
    accept: 'application/vnd.github+json',
    authorization: `Bearer ${token}`,
    'user-agent': 'CENtreeUsageAllowance/1.0',
  };
}

function copilotUsageHeaders(token) {
  return {
    accept: 'application/vnd.github+json',
    authorization: `Bearer ${token}`,
    'editor-version': 'vscode/1.96.2',
    'editor-plugin-version': 'copilot-chat/0.26.7',
    'user-agent': 'GitHubCopilotChat/0.26.7',
    'x-github-api-version': API_VERSION,
  };
}

export async function verifyGithubIdentity(token, { fetchImpl, timeoutMs } = {}) {
  const result = await requestJson({
    url: GITHUB_USER_URL,
    allowedUrls: [GITHUB_USER_URL],
    fetchImpl,
    timeoutMs,
    headers: githubIdentityHeaders(token),
  });
  if (result.status !== 200) throw statusError('GitHub identity', result.status);
  if (typeof result.value.login !== 'string' || !/^[A-Za-z0-9-]{1,39}$/u.test(result.value.login)) {
    throw new UsageError('identity_response', 'GitHub returned an unsupported identity response.');
  }
  return result.value.login;
}

function formBody(entries) {
  return new URLSearchParams(entries).toString();
}

export async function startDeviceFlow({ fetchImpl, timeoutMs } = {}) {
  const result = await requestJson({
    url: GITHUB_DEVICE_CODE_URL,
    allowedUrls: [GITHUB_DEVICE_CODE_URL],
    fetchImpl,
    method: 'POST',
    timeoutMs,
    headers: { accept: 'application/json', 'content-type': 'application/x-www-form-urlencoded' },
    body: formBody({ client_id: COPILOT_CLIENT_ID, scope: 'read:user' }),
  });
  if (result.status !== 200) throw statusError('GitHub device login', result.status);
  const value = result.value;
  const deviceCode = assertOpaque(value.device_code, 'GitHub device code');
  if (!/^[A-Z0-9-]{4,32}$/u.test(value.user_code ?? '')) {
    throw new UsageError('device_response', 'GitHub returned an unsupported device-login response.');
  }
  let verification;
  try {
    verification = new URL(value.verification_uri);
  } catch {
    throw new UsageError('device_response', 'GitHub returned an unsupported device-login response.');
  }
  if (verification.href !== 'https://github.com/login/device') {
    throw new UsageError('device_response', 'GitHub returned an unsupported device-login response.');
  }
  const expiresIn = Number(value.expires_in);
  const interval = Number(value.interval ?? 5);
  if (!Number.isFinite(expiresIn) || expiresIn < 1 || expiresIn > 1800 || !Number.isFinite(interval) || interval < 1 || interval > 30) {
    throw new UsageError('device_response', 'GitHub returned an unsupported device-login response.');
  }
  return { deviceCode, userCode: value.user_code, verificationUri: verification.href, expiresIn, interval };
}

export async function pollDeviceFlow(flow, {
  fetchImpl,
  timeoutMs,
  now = () => Date.now(),
  sleep = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds)),
} = {}) {
  const deadline = now() + flow.expiresIn * 1000;
  let interval = flow.interval;
  while (true) {
    const remaining = deadline - now();
    if (remaining <= 0) break;
    await sleep(Math.min(interval * 1000, remaining));
    if (now() >= deadline) break;
    const result = await requestJson({
      url: GITHUB_DEVICE_TOKEN_URL,
      allowedUrls: [GITHUB_DEVICE_TOKEN_URL],
      fetchImpl,
      method: 'POST',
      timeoutMs,
      headers: { accept: 'application/json', 'content-type': 'application/x-www-form-urlencoded' },
      body: formBody({
        client_id: COPILOT_CLIENT_ID,
        device_code: flow.deviceCode,
        grant_type: 'urn:ietf:params:oauth:grant-type:device_code',
      }),
    });
    if (result.status !== 200) throw statusError('GitHub device login', result.status);
    if (result.value.access_token) return assertOpaque(result.value.access_token, 'GitHub access token');
    switch (result.value.error) {
      case 'authorization_pending':
        break;
      case 'slow_down':
        interval += 5;
        break;
      case 'access_denied':
        throw new UsageError('access_denied', 'GitHub device login was denied or cancelled.');
      case 'expired_token':
        throw new UsageError('device_expired', 'GitHub device login expired.');
      default:
        throw new UsageError('device_response', 'GitHub returned an unsupported device-login response.');
    }
  }
  throw new UsageError('device_expired', 'GitHub device login expired.');
}

export function normalizeCopilotUsage(value) {
  const snapshots = value.quota_snapshots;
  if (!snapshots || typeof snapshots !== 'object' || Array.isArray(snapshots)) {
    throw new UsageError('unsupported_response', 'GitHub Copilot returned an unsupported usage shape.');
  }
  const windows = [];
  for (const key of ['chat', 'completions', 'premium_interactions']) {
    const source = snapshots[key];
    if (!source || typeof source !== 'object') continue;
    const unlimited = source.unlimited === true;
    const remaining = finiteNonNegative(source.remaining);
    const entitlement = finiteNonNegative(source.entitlement);
    const remainingPercent = finitePercent(source.percent_remaining);
    if (!unlimited && remaining === undefined && remainingPercent === undefined) continue;
    windows.push({
      label: key.replaceAll('_', ' '),
      unlimited,
      remaining,
      entitlement,
      remainingPercent,
      resetAt: resetText(source.reset_at ?? value.quota_reset_date),
    });
  }
  if (!windows.length) {
    throw new UsageError('unsupported_response', 'GitHub Copilot returned an unsupported usage shape.');
  }
  return windows;
}

export async function fetchCopilotUsage(token, { fetchImpl, timeoutMs } = {}) {
  const result = await requestJson({
    url: COPILOT_USAGE_URL,
    allowedUrls: [COPILOT_USAGE_URL],
    fetchImpl,
    timeoutMs,
    headers: copilotUsageHeaders(token),
  });
  if (result.status !== 200) throw statusError('GitHub Copilot', result.status);
  return normalizeCopilotUsage(result.value);
}

export async function checkCopilotUsage({
  fetchImpl = globalThis.fetch,
  runCommand,
  login = false,
  showIdentity = false,
  writeLogin = (message) => process.stdout.write(`${message}\n`),
  timeoutMs,
  clock,
} = {}) {
  const discovered = await discoverCopilotToken({
    runCommand,
    validateToken: (candidate) => verifyGithubIdentity(candidate, { fetchImpl, timeoutMs }),
  });
  let token = discovered?.token;
  let loginName = discovered?.validation;
  if (!token) {
    if (!login) {
      throw new UsageError('login_required', 'No usable GitHub token was found; rerun with --login to start device login.');
    }
    const flow = await startDeviceFlow({ fetchImpl, timeoutMs });
    writeLogin(`Open ${flow.verificationUri} and enter code ${flow.userCode}.`);
    token = await pollDeviceFlow(flow, { fetchImpl, timeoutMs, ...clock });
    loginName = await verifyGithubIdentity(token, { fetchImpl, timeoutMs });
  }
  const windows = await fetchCopilotUsage(token, { fetchImpl, timeoutMs });
  const report = [formatUsageReport('GitHub Copilot', windows), 'GitHub identity verified.'];
  if (showIdentity) report.push(`GitHub login: ${loginName}`);
  return report.join('\n');
}
