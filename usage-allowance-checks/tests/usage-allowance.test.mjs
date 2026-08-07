import assert from 'node:assert/strict';
import test from 'node:test';
import {
  UsageError,
  requestJson,
  runSafely,
} from '../lib/core.mjs';
import {
  CLAUDE_USAGE_URL,
  checkClaudeUsage,
  loadClaudeCredential,
} from '../lib/claude.mjs';
import {
  CODEX_USAGE_URL,
  checkCodexUsage,
  parseCodexConfig,
} from '../lib/codex.mjs';
import {
  COPILOT_CLIENT_ID,
  COPILOT_USAGE_URL,
  GITHUB_DEVICE_CODE_URL,
  GITHUB_DEVICE_TOKEN_URL,
  GITHUB_USER_URL,
  checkCopilotUsage,
  discoverCopilotToken,
  pollDeviceFlow,
  startDeviceFlow,
} from '../lib/copilot.mjs';

const CANARY_TOKEN = 'canary-secret-token-123';
const CANARY_DEVICE = 'canary-device-code-456';

function json(value, status = 200, headers = {}) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'content-type': 'application/json', ...headers },
  });
}

function memoryFs(files) {
  return {
    async stat(filePath) {
      if (!(filePath in files)) throw Object.assign(new Error('missing'), { code: 'ENOENT' });
      const text = files[filePath].text;
      return { size: Buffer.byteLength(text), mode: files[filePath].mode ?? 0o600, isFile: () => true };
    },
    async readFile(filePath) {
      if (!(filePath in files)) throw Object.assign(new Error('missing'), { code: 'ENOENT' });
      return files[filePath].text;
    },
  };
}

test('Claude reads the expected credential, warns on broad mode, and normalizes usage', async () => {
  const warnings = [];
  const output = await checkClaudeUsage({
    credentialOptions: {
      filePath: 'synthetic-claude.json',
      fsApi: memoryFs({
        'synthetic-claude.json': {
          mode: 0o644,
          text: JSON.stringify({ claudeAiOauth: { accessToken: CANARY_TOKEN, expiresAt: Date.now() + 60_000 } }),
        },
      }),
    },
    warn: (message) => warnings.push(message),
    fetchImpl: async (url, options) => {
      assert.equal(url.href, CLAUDE_USAGE_URL);
      assert.equal(options.redirect, 'manual');
      assert.equal(options.headers['anthropic-beta'], 'oauth-2025-04-20');
      return json({ five_hour: { utilization: 25, resets_at: '2030-01-01T00:00:00Z' } });
    },
  });
  assert.match(output, /75% available/u);
  assert.equal(warnings.length, 1);
  assert.doesNotMatch(`${output}\n${warnings.join('\n')}`, /canary-secret/u);
});

test('credential parsing rejects malformed JSON, missing token, and expired token safely', async () => {
  for (const text of ['{bad', '{}', JSON.stringify({ accessToken: CANARY_TOKEN, expiresAt: 1 })]) {
    await assert.rejects(
      loadClaudeCredential({ filePath: 'credential', fsApi: memoryFs({ credential: { text } }), now: 10_000 }),
      UsageError,
    );
  }
});

test('HTTP helper refuses redirects, oversized bodies, network errors, and non-canonical hosts', async () => {
  await assert.rejects(
    requestJson({ url: CLAUDE_USAGE_URL, allowedUrls: [CLAUDE_USAGE_URL], fetchImpl: async () => new Response('', { status: 302 }) }),
    (error) => error.code === 'redirect_refused',
  );
  await assert.rejects(
    requestJson({ url: CLAUDE_USAGE_URL, allowedUrls: [CLAUDE_USAGE_URL], fetchImpl: async () => json({ value: 'x'.repeat(100) }), maxBytes: 16 }),
    (error) => error.code === 'response_too_large',
  );
  await assert.rejects(
    requestJson({ url: CLAUDE_USAGE_URL, allowedUrls: [CLAUDE_USAGE_URL], fetchImpl: async () => { throw new Error(CANARY_TOKEN); } }),
    (error) => error.code === 'network' && !error.message.includes(CANARY_TOKEN),
  );
  await assert.rejects(
    requestJson({ url: CLAUDE_USAGE_URL, allowedUrls: [CLAUDE_USAGE_URL], fetchImpl: async () => new Response('{bad') }),
    (error) => error.code === 'response_json',
  );
  await assert.rejects(
    requestJson({
      url: CLAUDE_USAGE_URL,
      allowedUrls: [CLAUDE_USAGE_URL],
      timeoutMs: 1,
      fetchImpl: async (_url, options) => new Promise((_resolve, reject) => {
        options.signal.addEventListener('abort', () => reject(new Error(CANARY_TOKEN)), { once: true });
      }),
    }),
    (error) => error.code === 'timeout' && !error.message.includes(CANARY_TOKEN),
  );
  await assert.rejects(
    requestJson({ url: 'https://api.guthub.com/copilot_internal/user', allowedUrls: [COPILOT_USAGE_URL], fetchImpl: async () => json({}) }),
    (error) => error.code === 'endpoint_refused',
  );
});

test('provider status errors are fixed and sanitized for 401, 403, and 429', async () => {
  for (const status of [401, 403, 429]) {
    await assert.rejects(
      checkClaudeUsage({
        credentialOptions: { filePath: 'credential', fsApi: memoryFs({ credential: { text: JSON.stringify({ accessToken: CANARY_TOKEN }) } }) },
        fetchImpl: async () => new Response(CANARY_TOKEN, { status }),
      }),
      (error) => !error.message.includes(CANARY_TOKEN),
    );
  }
});

test('Codex config parsing selects the active provider without dumping config', () => {
  assert.equal(parseCodexConfig(`model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://trusted.example/v1"`), 'https://trusted.example/v1');
});

test('Codex uses primary first and permits fallback only for an exact trusted HTTPS origin', async () => {
  const auth = JSON.stringify({
    tokens: { access_token: CANARY_TOKEN, account_id: 'acct-synthetic' },
    base_url: 'https://trusted.example/v1',
  });
  const calls = [];
  const fetchImpl = async (url, options) => {
    calls.push({ url: url.href, headers: options.headers });
    if (url.href === CODEX_USAGE_URL) return json({ error: 'unsupported' }, 404);
    return json({ rate_limit: { primary_window: { used_percent: 20, reset_at: 1_900_000_000 } } });
  };
  const options = { credentialOptions: { authPath: 'auth', configPath: 'config', fsApi: memoryFs({ auth: { text: auth } }) }, fetchImpl };
  await assert.rejects(checkCodexUsage(options), (error) => error.code === 'untrusted_origin');
  const output = await checkCodexUsage({ ...options, trustedOrigin: 'https://trusted.example' });
  assert.match(output, /80% available/u);
  assert.deepEqual(calls.slice(-2).map((call) => call.url), [CODEX_USAGE_URL, 'https://trusted.example/v1/api/codex/usage']);
  assert.ok(calls.every((call) => call.headers.authorization === `Bearer ${CANARY_TOKEN}`));
  assert.doesNotMatch(output, /canary|acct-synthetic/u);
});

test('Codex does not fallback for auth, rate-limit, or arbitrary configured origins', async () => {
  const credentialOptions = {
    authPath: 'auth',
    configPath: 'config',
    fsApi: memoryFs({ auth: { text: JSON.stringify({ tokens: { access_token: CANARY_TOKEN, account_id: 'acct' }, base_url: 'http://unsafe.example' }) } }),
  };
  for (const status of [401, 429]) {
    let calls = 0;
    await assert.rejects(
      checkCodexUsage({ credentialOptions, trustedOrigin: 'https://unsafe.example', fetchImpl: async () => { calls += 1; return json({}, status); } }),
      UsageError,
    );
    assert.equal(calls, 1);
  }
  await assert.rejects(
    checkCodexUsage({ credentialOptions, trustedOrigin: 'https://unsafe.example', fetchImpl: async () => json({}, 404) }),
    (error) => error.code === 'untrusted_origin',
  );
});

test('Copilot token discovery checks only named global/system Git and gh sources in order', async () => {
  const calls = [];
  const discovered = await discoverCopilotToken({
    runCommand: async (command, args) => {
      calls.push([command, ...args]);
      if (args.includes('--global')) return `${CANARY_TOKEN}\nsecond-line\n`;
      return command === 'gh' ? `${CANARY_TOKEN}\n` : undefined;
    },
    validateToken: async () => 'synthetic-user',
  });
  assert.equal(discovered.token, CANARY_TOKEN);
  assert.equal(discovered.validation, 'synthetic-user');
  assert.deepEqual(calls, [
    ['git', 'config', '--global', '--get', 'github.copilot.oauthToken'],
    ['git', 'config', '--system', '--get', 'github.copilot.oauthToken'],
    ['gh', 'auth', 'token', '--hostname', 'github.com'],
  ]);
  assert.ok(calls.flat().every((part) => part !== '--local' && part !== CANARY_TOKEN));
});

test('Copilot skips malformed and unauthorized candidates before using a later valid source', async () => {
  const deniedToken = 'denied-candidate-token-123';
  const sourceCalls = [];
  const httpCalls = [];
  const output = await checkCopilotUsage({
    runCommand: async (command, args) => {
      sourceCalls.push([command, ...args]);
      if (args.includes('--global')) return `${CANARY_TOKEN}\nsecond-line\n`;
      if (args.includes('--system')) return `${deniedToken}\n`;
      return `${CANARY_TOKEN}\n`;
    },
    fetchImpl: async (url, options) => {
      httpCalls.push(url.href);
      if (url.href === GITHUB_USER_URL && options.headers.authorization.includes(deniedToken)) {
        return json({ message: CANARY_TOKEN }, 401);
      }
      if (url.href === GITHUB_USER_URL) return json({ login: 'synthetic-user' });
      return json({ quota_snapshots: { chat: { remaining: 10, entitlement: 20 } } });
    },
  });
  assert.equal(sourceCalls.length, 3);
  assert.deepEqual(httpCalls, [GITHUB_USER_URL, GITHUB_USER_URL, COPILOT_USAGE_URL]);
  assert.doesNotMatch(output, /denied-candidate|canary-secret|synthetic-user/u);
});

test('Copilot device flow validates display fields and handles pending plus slow_down', async () => {
  const start = await startDeviceFlow({
    fetchImpl: async (url, options) => {
      assert.equal(url.href, GITHUB_DEVICE_CODE_URL);
      assert.match(options.body, new RegExp(`client_id=${COPILOT_CLIENT_ID}`, 'u'));
      assert.match(options.body, /scope=read%3Auser/u);
      return json({ device_code: CANARY_DEVICE, user_code: 'ABCD-1234', verification_uri: 'https://github.com/login/device', expires_in: 60, interval: 1 });
    },
  });
  const replies = [
    { error: 'authorization_pending' },
    { error: 'slow_down' },
    { access_token: CANARY_TOKEN },
  ];
  let now = 0;
  const waits = [];
  const token = await pollDeviceFlow(start, {
    fetchImpl: async (url, options) => {
      assert.equal(url.href, GITHUB_DEVICE_TOKEN_URL);
      assert.match(options.body, /grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code/u);
      return json(replies.shift());
    },
    now: () => now,
    sleep: async (milliseconds) => { waits.push(milliseconds); now += milliseconds; },
  });
  assert.equal(token, CANARY_TOKEN);
  assert.deepEqual(waits, [1000, 1000, 6000]);
});

test('Copilot device flow handles denied, expired, and cancelled outcomes safely', async () => {
  for (const [providerError, code] of [['access_denied', 'access_denied'], ['expired_token', 'device_expired']]) {
    let now = 0;
    await assert.rejects(
      pollDeviceFlow({ deviceCode: CANARY_DEVICE, expiresIn: 10, interval: 1 }, {
        fetchImpl: async () => json({ error: providerError }),
        now: () => now,
        sleep: async (milliseconds) => { now += milliseconds; },
      }),
      (error) => error.code === code && !error.message.includes(CANARY_DEVICE),
    );
  }
});

test('Copilot device polling never requests at or after the local deadline', async () => {
  let now = 0;
  let requests = 0;
  const waits = [];
  await assert.rejects(
    pollDeviceFlow({ deviceCode: CANARY_DEVICE, expiresIn: 5, interval: 10 }, {
      fetchImpl: async () => { requests += 1; return json({ access_token: CANARY_TOKEN }); },
      now: () => now,
      sleep: async (milliseconds) => { waits.push(milliseconds); now += milliseconds; },
    }),
    (error) => error.code === 'device_expired',
  );
  assert.deepEqual(waits, [5000]);
  assert.equal(requests, 0);
});

test('Copilot device polling applies repeated slow_down increments within the deadline', async () => {
  const replies = [{ error: 'slow_down' }, { error: 'slow_down' }, { access_token: CANARY_TOKEN }];
  const waits = [];
  let now = 0;
  const token = await pollDeviceFlow({ deviceCode: CANARY_DEVICE, expiresIn: 30, interval: 1 }, {
    fetchImpl: async () => json(replies.shift()),
    now: () => now,
    sleep: async (milliseconds) => { waits.push(milliseconds); now += milliseconds; },
  });
  assert.equal(token, CANARY_TOKEN);
  assert.deepEqual(waits, [1000, 6000, 11000]);
});

test('Copilot starts explicit device login only after every discovered candidate is unusable', async () => {
  const sourceTokens = ['denied-global-token-123', 'denied-system-token-123', 'denied-gh-token-123'];
  const urls = [];
  const loginMessages = [];
  let sourceIndex = 0;
  let now = 0;
  const output = await checkCopilotUsage({
    login: true,
    runCommand: async () => `${sourceTokens[sourceIndex++]}\n`,
    writeLogin: (message) => loginMessages.push(message),
    clock: {
      now: () => now,
      sleep: async (milliseconds) => { now += milliseconds; },
    },
    fetchImpl: async (url) => {
      urls.push(url.href);
      if (url.href === GITHUB_USER_URL && urls.filter((entry) => entry === GITHUB_USER_URL).length <= 3) {
        return json({ message: CANARY_TOKEN }, 401);
      }
      if (url.href === GITHUB_DEVICE_CODE_URL) {
        return json({ device_code: CANARY_DEVICE, user_code: 'ABCD-1234', verification_uri: 'https://github.com/login/device', expires_in: 30, interval: 1 });
      }
      if (url.href === GITHUB_DEVICE_TOKEN_URL) return json({ access_token: CANARY_TOKEN });
      if (url.href === GITHUB_USER_URL) return json({ login: 'synthetic-user' });
      return json({ quota_snapshots: { chat: { remaining: 10, entitlement: 20 } } });
    },
  });
  assert.deepEqual(urls.slice(0, 4), [GITHUB_USER_URL, GITHUB_USER_URL, GITHUB_USER_URL, GITHUB_DEVICE_CODE_URL]);
  assert.equal(loginMessages.length, 1);
  assert.doesNotMatch(`${output}\n${loginMessages.join('\n')}`, /denied-global|denied-system|denied-gh|canary-secret/u);
});

test('Copilot validates identity before usage and sends the required usage headers', async () => {
  const calls = [];
  const output = await checkCopilotUsage({
    runCommand: async (command) => command === 'git' ? CANARY_TOKEN : undefined,
    fetchImpl: async (url, options) => {
      calls.push({ url: url.href, headers: options.headers });
      if (url.href === GITHUB_USER_URL) return json({ login: 'synthetic-user', id: CANARY_TOKEN });
      return json({ quota_snapshots: { chat: { entitlement: 300, remaining: 225, percent_remaining: 75 } }, quota_reset_date: '2030-01-01' });
    },
  });
  assert.deepEqual(calls.map((call) => call.url), [GITHUB_USER_URL, COPILOT_USAGE_URL]);
  assert.deepEqual(Object.keys(calls[0].headers).sort(), ['accept', 'authorization', 'user-agent']);
  assert.equal(calls[0].headers.accept, 'application/vnd.github+json');
  assert.equal(calls[0].headers.authorization, `Bearer ${CANARY_TOKEN}`);
  assert.equal(calls[0].headers['user-agent'], 'CENtreeUsageAllowance/1.0');
  assert.equal(calls[0].headers['editor-version'], undefined);
  assert.equal(calls[0].headers['editor-plugin-version'], undefined);
  assert.equal(calls[0].headers['x-github-api-version'], undefined);
  assert.deepEqual(Object.keys(calls[1].headers).sort(), [
    'accept',
    'authorization',
    'editor-plugin-version',
    'editor-version',
    'user-agent',
    'x-github-api-version',
  ]);
  assert.equal(calls[1].headers.authorization, `Bearer ${CANARY_TOKEN}`);
  assert.equal(calls[1].headers['editor-version'], 'vscode/1.96.2');
  assert.equal(calls[1].headers['editor-plugin-version'], 'copilot-chat/0.26.7');
  assert.equal(calls[1].headers['user-agent'], 'GitHubCopilotChat/0.26.7');
  assert.equal(calls[1].headers['x-github-api-version'], '2025-04-01');
  assert.match(output, /GitHub identity verified/u);
  assert.doesNotMatch(output, /synthetic-user|canary/u);
});

test('identity failure stops before the Copilot private endpoint', async () => {
  let calls = 0;
  let sourceCalls = 0;
  await assert.rejects(
    checkCopilotUsage({
      runCommand: async () => sourceCalls++ === 0 ? CANARY_TOKEN : undefined,
      fetchImpl: async () => { calls += 1; return json({ message: CANARY_TOKEN }, 401); },
    }),
    (error) => error.code === 'login_required' && !error.message.includes(CANARY_TOKEN),
  );
  assert.equal(calls, 1);
});

test('safe runner never emits secrets from unexpected errors', async () => {
  let stdout = '';
  let stderr = '';
  const exitCode = await runSafely(async () => { throw new Error(CANARY_TOKEN); }, {
    stdout: { write: (text) => { stdout += text; } },
    stderr: { write: (text) => { stderr += text; } },
  });
  assert.equal(exitCode, 1);
  assert.doesNotMatch(`${stdout}${stderr}`, /canary-secret/u);
});
