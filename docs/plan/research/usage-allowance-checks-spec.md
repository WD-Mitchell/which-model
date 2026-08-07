# usage-allowance-checks — Full Behavioural + Security Spec

All paths relative to `/Users/will/Projects/Software/WDM-Model-Picker/usage-allowance-checks/`.

---
## 1. `lib/core.mjs` (293 lines) — function-by-function

### Constants (core.mjs:1-6)
```js
export const DEFAULT_TIMEOUT_MS = 10_000;
export const MAX_CREDENTIAL_BYTES = 1_048_576;   // 1 MiB
export const MAX_RESPONSE_BYTES = 262_144;        // 256 KiB
```

### `class UsageError extends Error` (core.mjs:8-14)
`constructor(code, message)`: sets `this.name = 'UsageError'`, `this.code = code`. All thrown domain errors use this; codes observed across the codebase: `unsafe_credential`, `credential_file`, `credential_json`, `endpoint_refused`, `untrusted_origin`, `response_too_large`, `timeout`, `network`, `response_json`, `redirect_refused`, `unauthorized`, `rate_limited`, `provider_status`, `expired_credential`, `unsupported_response`, `fallback_unavailable`, `arguments`, `identity_validation`, `identity_response`, `device_response`, `access_denied`, `device_expired`, `login_required`.

### `assertOpaque(value, label='credential')` (core.mjs:16-25)
Signature: `(string, string) -> string | throws`.
Rejects unless: `typeof value === 'string'`, `8 <= length <= 16384`, and **no** whitespace/control chars matched by `/[\s\u0000-\u001f\u007f]/u`. On failure: `throw new UsageError('unsafe_credential', 'The ${label} is missing or unsafe.')`. Returns `value` unchanged on success.

### `assertIdentifier(value, label)` (core.mjs:27-35)
Same shape check but length bound `1 <= length <= 512` (no minimum of 8). Same regex, same error code `unsafe_credential`, message `The ${label} is missing or unsafe.`.

### `readBoundedFile(filePath, { fsApi, maxBytes=MAX_CREDENTIAL_BYTES, missingMessage })` (core.mjs:37-60)
Async. Steps:
1. `stat = await fsApi.stat(filePath)`.
2. If `stat.isFile` exists and returns false → `UsageError('credential_file', missingMessage)`.
3. If `!Number.isSafeInteger(stat.size) || stat.size < 1 || stat.size > maxBytes` → `UsageError('credential_file', 'The credential file has an invalid size.')`.
4. `text = await fsApi.readFile(filePath, 'utf8')`.
5. Recheck actual UTF-8 byte length > maxBytes → same invalid-size error (defends against stat/readFile size mismatch).
6. Returns `{ text, mode: stat.mode }`.
Catch-all: any `UsageError` rethrown as-is; `ENOENT` → `UsageError('credential_file', missingMessage)`; anything else → `UsageError('credential_file', 'The credential file could not be read safely.')` (swallows underlying error detail — no leakage).

### `readCredentialJson(filePath, options)` (core.mjs:62-70)
Wraps `readBoundedFile`, then `JSON.parse(text)`. Requires parsed value to be a non-null, non-array object, else throws (caught) → `UsageError('credential_json', 'The credential file is not valid JSON.')`. Returns `{ value, mode }`.

### `hasBroadPermissions(mode)` (core.mjs:72-74)
`Number.isInteger(mode) && (mode & (S_IRWXG | S_IRWXO)) !== 0` — true if group or other has any rwx bits (i.e., broader than `0600`/`0700`).

### `validateExactHttpsUrl(rawUrl, allowedUrls)` (core.mjs:76-91)
Parses `rawUrl` with `new URL()`; parse failure → `UsageError('endpoint_refused', 'The provider endpoint is not a valid URL.')`.
Rejects (→ `UsageError('endpoint_refused', 'The provider endpoint was refused.')`) if: `protocol !== 'https:'`, OR `username`/`password` present, OR `hash` present, OR `!allowedUrls.includes(url.href)` (exact allow-list membership, not prefix/origin match). Returns the parsed `URL`.

### `validateTrustedBaseUrl(rawUrl, trustedOrigin)` (core.mjs:93-116)
Parses both `rawUrl` (→`base`) and `trustedOrigin` (→`trusted`); either parse failure → `UsageError('untrusted_origin', 'The configured Codex fallback origin was not explicitly trusted.')`.
Rejects with same `untrusted_origin` error if ANY of: `base.protocol !== 'https:'`; `base.username`/`password` set; `base.search` or `base.hash` set; `trusted.protocol !== 'https:'`; `trusted.username`/`password` set; `trusted.search`/`hash` set; `trusted.pathname !== '/'` (i.e. the trust argument MUST be a bare origin, no path); `base.origin !== trusted.origin`.
Then builds `target = new URL('api/codex/usage', base.origin + pathname_with_trailing_slash)` — appends fixed suffix path `api/codex/usage` under the base's own path. If `target.origin !== base.origin` (defensive, should be unreachable) → `UsageError('endpoint_refused', 'The configured Codex fallback endpoint was refused.')`. Returns target `URL`.

### `readResponseText(response, maxBytes)` (internal, not exported) (core.mjs:118-144)
- Checks `content-length` header; if present and `> maxBytes` → `UsageError('response_too_large', ...)` before reading body.
- If streaming body (`response.body.getReader`) available: reads chunks, accumulates `total`; if `total > maxBytes` mid-stream, cancels reader and throws `response_too_large`. Decodes with `new TextDecoder('utf-8', { fatal: true })` (strict UTF-8, throws on invalid sequences → surfaces as generic error upstream).
- Fallback: `response.text()` then re-check `Buffer.byteLength(text,'utf8') > maxBytes` → `response_too_large`.

### `requestJson({ url, allowedUrls, fetchImpl, method='GET', headers={}, body, timeoutMs=DEFAULT_TIMEOUT_MS, maxBytes=MAX_RESPONSE_BYTES })` (core.mjs:146-190)
1. `endpoint = validateExactHttpsUrl(url, allowedUrls)`.
2. `AbortController` + `setTimeout(() => controller.abort(), timeoutMs)`.
3. `fetchImpl(endpoint, { method, headers, body, redirect: 'manual', signal })` — **redirect is always `'manual'`**, never followed.
4. fetch throw → if aborted, `UsageError('timeout', 'The provider request timed out.')`; else `UsageError('network', 'The provider request failed.')` (original error message discarded — prevents secret leakage from thrown errors, verified in tests with `CANARY_TOKEN` not appearing).
5. If `300 <= status < 400` → `UsageError('redirect_refused', 'The provider attempted an unsafe redirect.')` (redirects are NEVER followed, always rejected explicitly).
6. `text = await readResponseText(...)`; wraps non-UsageError failures as `timeout` (if aborted) or `network`.
7. If status outside `[200,300)` → returns `{ status, value: undefined }` (no JSON parse attempted for error statuses — caller must call `statusError`).
8. Empty text → `UsageError('response_json', 'The provider returned an empty response.')`.
9. `JSON.parse(text)`; must be non-null, non-array object, else caught → `UsageError('response_json', 'The provider returned unsupported JSON.')`.
10. Returns `{ status, value }`.

### `statusError(provider, status)` (core.mjs:192-200)
- `401 | 403` → `UsageError('unauthorized', '${provider} rejected the credential.')`
- `429` → `UsageError('rate_limited', '${provider} rate-limited the usage request.')`
- else → `UsageError('provider_status', '${provider} usage is unavailable (HTTP ${status}).')`

### `finitePercent(value)` (core.mjs:202-205)
Coerces string→Number only if it's a non-empty trimmed string, else passes through. Returns the number iff `Number.isFinite(number) && 0 <= number <= 100`, else `undefined`.

### `finiteNonNegative(value)` (core.mjs:207-210)
Same coercion; valid iff `Number.isFinite(number) && number >= 0`, else `undefined`.

### `resetText(value)` (core.mjs:212-224)
- Numeric epoch: if `typeof value === 'number' && isFinite && value > 0`: treats `value > 10_000_000_000` as milliseconds already, else multiplies by 1000 (seconds→ms heuristic, 10-billion threshold ≈ year 2286 in seconds / 1973 in ms). Builds `Date`; returns ISO string or `undefined` if invalid.
- String: only if `length <= 128`; `Date.parse`; returns ISO string or `undefined`.
- Else `undefined`.

### `displayNumber(value)` (internal) (core.mjs:226-228)
`Number.isInteger(value) ? String(value) : value.toFixed(1).replace(/\.0$/, '')` — integers print bare, else 1 decimal place, trailing `.0` trimmed (so `25` → `"25"`, `24.5` → `"24.5"`, `24.0` computed differently as isInteger catches it first).

### `formatUsageReport(provider, windows)` (core.mjs:230-247) — VERBATIM OUTPUT FORMAT
```js
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
```
Exact line format: header line `"${provider} usage allowance"`, then one line per window: `"- ${label}: ${detail1}; ${detail2}; ..."`. Order of detail fields (when present): `unlimited` (exclusive) OR [`usedPercent`, `remainingPercent`, `remaining`, `entitlement`] in that fixed order, then `resetAt` always last. Example verified by test: Claude 25% used → line contains `75% available` (i.e. `remainingPercent` computed as `100 - usedPercent` by caller, not by this function).

### `runSafely(action, { stdout=process.stdout, stderr=process.stderr }={})` (core.mjs:249-263) — CLI ENTRY WRAPPER
```js
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
```
Success → exit code `0`, report on stdout with trailing `\n`, nothing on stderr (other than provider `warn()` calls made by the action itself, e.g. Claude broad-permission warning, which go to stderr separately during execution — see claude.mjs).
Failure → exit code `1`. Known `UsageError` → stderr `Usage check failed [<code>]: <message>\n`. Unknown/non-UsageError error → **exact fixed string** `Usage check failed [unexpected]: The check stopped safely.\n` — original error message is NEVER included (verified by test asserting `CANARY_TOKEN` absent).

---
## 2. Per-provider flows

### 2.1 Claude (`lib/claude.mjs`, 91 lines)
- `CLAUDE_USAGE_URL = 'https://api.anthropic.com/api/oauth/usage'` (claude.mjs:15)
- `loadClaudeCredential({ filePath = ~/.claude/credentials.json, fsApi = fs, now = Date.now() })` (claude.mjs:17-31):
  - Reads JSON via `readCredentialJson`, missing-file message: `'Claude credentials were not found; sign in with Claude Code first.'`.
  - `oauth = value.claudeAiOauth ?? value.oauth ?? value` (nested-or-flat shape tolerance).
  - `token = assertOpaque(oauth.accessToken ?? oauth.access_token, 'Claude access token')`.
  - `expiresAt = oauth.expiresAt ?? oauth.expires_at`; if present, parsed as number or `Date.parse`; seconds→ms heuristic (`>10_000_000_000` treated as already-ms); if `!isFinite(ms) || ms <= now` → `UsageError('expired_credential', 'The Claude access token is expired.')`.
  - Returns `{ token, broadPermissions: hasBroadPermissions(mode) }`.
- `normalizeClaudeUsage(value)` (claude.mjs:33-56): iterates fixed key list with labels:
  - `five_hour` → `"five hour"`
  - `seven_day` → `"seven day"`
  - `seven_day_sonnet` → `"seven day Sonnet"`
  - `seven_day_opus` → `"seven day Opus"`
  - `seven_day_oauth_apps` → `"seven day OAuth apps"`
  For each present object-typed source: `usedPercent = finitePercent(source.utilization ?? source.used_percent)`; skip window if undefined. Window = `{ label, usedPercent, remainingPercent: 100 - usedPercent, resetAt: resetText(source.resets_at ?? source.reset_at) }`. If zero windows collected → `UsageError('unsupported_response', 'Claude returned an unsupported usage shape.')`.
- `checkClaudeUsage({ fetchImpl=fetch, credentialOptions, timeoutMs, warn })` (claude.mjs:58-91):
  - Loads credential; if `broadPermissions`, calls `warn('Warning: Claude credential permissions are broader than 0600; review them before continuing.')` (goes to stderr via script wiring).
  - `requestJson` with `allowedUrls: [CLAUDE_USAGE_URL]`, headers exactly: `{ accept: 'application/json', authorization: 'Bearer ${token}', 'anthropic-beta': 'oauth-2025-04-20' }`.
  - Non-200 status → `statusError('Claude', status)`.
  - Returns `formatUsageReport('Claude', normalizeClaudeUsage(value))`.

### 2.2 Codex (`lib/codex.mjs`, 141 lines)
- `CODEX_USAGE_URL = 'https://chatgpt.com/backend-api/wham/usage'` (codex.mjs:16)
- `FALLBACK_STATUSES = new Set([404, 405, 410, 501])` (codex.mjs:17) — only these primary-endpoint statuses trigger fallback consideration; anything else (incl. 401/403/429) throws immediately via `statusError`.
- `parseCodexConfig(text)` (codex.mjs:19-40) — line-oriented TOML-ish parser, NOT a real TOML parser:
  - Splits on `/\r?\n/u`; trims each line; skips blank/`#`-comment lines.
  - Section header regex: `/^\[model_providers\.([A-Za-z0-9_-]+)\]$/u` sets `section` to captured provider id.
  - Any other `[...]` line sets `section = '__other__'` (ignored zone).
  - Assignment regex: `/^(model_provider|base_url)\s*=\s*(["'])(.*?)\2\s*(?:#.*)?$/u` — only recognizes keys `model_provider` and `base_url`, quoted (single or double) values, optional trailing comment.
  - At root (`!section`): `model_provider = "x"` sets `activeProvider`; `base_url = "x"` sets `rootBaseUrl`.
  - Inside a named `[model_providers.<id>]` section: `base_url = "x"` recorded into `providerUrls.get(id)`.
  - Returns `(activeProvider && providerUrls.get(activeProvider)) || rootBaseUrl` — i.e. prefers the base_url of the section matching the root-level `model_provider`, else falls back to root `base_url`.
  - Test-verified: `parseCodexConfig('model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://trusted.example/v1"')` → `'https://trusted.example/v1'`.
- `optionalConfiguredBaseUrl(configPath, fsApi)` (codex.mjs:42-49): reads config file; if missing (`UsageError` code `credential_file`) returns `undefined` silently (config.toml is optional); other UsageErrors rethrown.
- `loadCodexCredential({ authPath=~/.codex/auth.json, configPath=~/.codex/config.toml, fsApi=fs })` (codex.mjs:51-66):
  - Missing-file message: `'Codex credentials were not found; sign in with Codex first.'`.
  - `tokens = value.tokens ?? value.auth ?? value`.
  - `token = assertOpaque(tokens.access_token ?? tokens.accessToken, 'Codex access token')`.
  - `accountId = assertIdentifier(tokens.account_id ?? tokens.accountId ?? value.account_id ?? value.chatgpt_account_id, 'ChatGPT account identifier')`.
  - `configuredBaseUrl = value.base_url ?? value.baseUrl ?? value.openai_base_url ?? await optionalConfiguredBaseUrl(configPath, fsApi)` — auth.json fields take priority over config.toml parse.
  - Returns `{ token, accountId, configuredBaseUrl }`.
- `normalizeCodexUsage(value)` (codex.mjs:68-93):
  - `rateLimit = value.rate_limit ?? value.rateLimit ?? value`.
  - Fixed windows: `primary_window` → `"primary window"`, `secondary_window` → `"secondary window"` (also checks camelCase key variant via regex conversion).
  - `usedPercent = finitePercent(source.used_percent ?? source.usedPercent)`; window = `{ label, usedPercent, remainingPercent: 100-usedPercent, resetAt: resetText(source.reset_at ?? source.resetAt) }`.
  - Additionally: `credits = value.credits`; `balance = finiteNonNegative(credits.balance)`; if defined, pushes `{ label: 'credits', remaining: balance }` (absolute-count window, no percent fields).
  - Zero windows → `UsageError('unsupported_response', 'Codex returned an unsupported usage shape.')`.
- `fetchCodex(url, allowedUrls, credential, fetchImpl, timeoutMs)` (codex.mjs:95-106): headers exactly `{ accept: 'application/json', authorization: 'Bearer ${credential.token}', 'chatgpt-account-id': credential.accountId }`.
- `checkCodexUsage({ fetchImpl=fetch, credentialOptions, trustedOrigin, timeoutMs })` (codex.mjs:108-128) — **fallback flow**:
  1. Load credential.
  2. Call primary `CODEX_USAGE_URL`. If `status===200` → format and return.
  3. If status NOT in `FALLBACK_STATUSES` → `throw statusError('Codex', status)` (no fallback attempted; test confirms exactly 1 fetch call for 401/429).
  4. If `!credential.configuredBaseUrl` → `UsageError('fallback_unavailable', 'Codex did not advertise a configured fallback endpoint.')`.
  5. `fallback = validateTrustedBaseUrl(credential.configuredBaseUrl, trustedOrigin)` — throws `untrusted_origin` if `trustedOrigin` absent/mismatched/malformed (test: missing `--trust-configured-origin` on a 404 primary throws `untrusted_origin` even though `configuredBaseUrl` exists).
  6. Fetch `fallback.href` with same headers; non-200 → `statusError('Codex fallback', status)`.
  7. Return formatted report. Fallback target is always `<origin>/api/codex/usage` regardless of the configured path (per `validateTrustedBaseUrl` construction) — test verifies exact fallback URL `https://trusted.example/v1/api/codex/usage` from configured `https://trusted.example/v1`.

### 2.3 GitHub Copilot (`lib/copilot.mjs`, ~230 lines)
- Constants (copilot.mjs:12-16):
  ```js
  GITHUB_DEVICE_CODE_URL = 'https://github.com/login/device/code'
  GITHUB_DEVICE_TOKEN_URL = 'https://github.com/login/oauth/access_token'
  GITHUB_USER_URL = 'https://api.github.com/user'
  COPILOT_USAGE_URL = 'https://api.github.com/copilot_internal/user'
  COPILOT_CLIENT_ID = 'Iv1.b507a08c87ecfe98'
  ```
  `API_VERSION = '2025-04-01'` (copilot.mjs:19).
- `defaultCommandRunner(command, args)` (copilot.mjs:21-31): `execFile` via `promisify`, options `{ encoding: 'utf8', maxBuffer: 32_768, timeout: 3_000, windowsHide: true }`. Any failure (including timeout) swallowed → returns `undefined` (never throws upward).
- `tokenCandidate(output)` (copilot.mjs:33-40): strips exactly one trailing `\r\n` or `\n` (not both, not trimEnd — extra blank lines after cause `assertOpaque` rejection since embedded `\n` triggers the control-char regex), then `assertOpaque(candidate, 'GitHub access token')`.
- `discoverCopilotToken({ runCommand=defaultCommandRunner, validateToken })` (copilot.mjs:42-68) — **token discovery order** (fixed, exact):
  1. `git config --global --get github.copilot.oauthToken`
  2. `git config --system --get github.copilot.oauthToken`
  3. `gh auth token --hostname github.com`
  (NEVER `--local` or repo-scoped git config — SKILL.md explicitly calls this out and test asserts `--local` never appears in calls.)
  `validateToken` is REQUIRED — if not a function, throws `UsageError('identity_validation', 'GitHub token discovery requires identity validation.')` before any command runs.
  For each source in order: run command; skip (`continue`) if output is `undefined`/`''`; extract candidate via `tokenCandidate` — on `unsafe_credential` UsageError, skip to next source (malformed token silently tried next); on any other UsageError, rethrow.
  Then `return { token: candidate, validation: await validateToken(candidate) }` on the FIRST candidate that both parses safely AND validates; if `validateToken` throws `unauthorized` or `identity_response` UsageError, skip to next source (test: denied System token skipped, gh token used); any other error type rethrown.
  Returns `undefined` if no source yields a usable+valid token.
- `githubIdentityHeaders(token)` (copilot.mjs:75-81): `{ accept: 'application/vnd.github+json', authorization: 'Bearer ${token}', 'user-agent': 'CENtreeUsageAllowance/1.0' }` — exactly 3 headers (test asserts sorted keys `['accept','authorization','user-agent']`; NO editor-version/plugin/api-version on identity check).
- `copilotUsageHeaders(token)` (copilot.mjs:83-90): `{ accept: 'application/vnd.github+json', authorization: 'Bearer ${token}', 'editor-version': 'vscode/1.96.2', 'editor-plugin-version': 'copilot-chat/0.26.7', 'user-agent': 'GitHubCopilotChat/0.26.7', 'x-github-api-version': '2025-04-01' }` — 6 headers total, spoofing VS Code Copilot Chat client identity for the private endpoint.
- `verifyGithubIdentity(token, { fetchImpl, timeoutMs })` (copilot.mjs:92-104) — **identity verification gate**: `GET GITHUB_USER_URL` with identity headers. Non-200 → `statusError('GitHub identity', status)`. Response `login` field MUST be a string matching `/^[A-Za-z0-9-]{1,39}$/u` (GitHub username charset/length rules) else `UsageError('identity_response', 'GitHub returned an unsupported identity response.')`. Returns the login string.
- `formBody(entries)` (copilot.mjs:106-108): `new URLSearchParams(entries).toString()`.
- `startDeviceFlow({ fetchImpl, timeoutMs })` (copilot.mjs:110-132):
  - `POST GITHUB_DEVICE_CODE_URL`, headers `{ accept:'application/json', 'content-type':'application/x-www-form-urlencoded' }`, body `client_id=<COPILOT_CLIENT_ID>&scope=read%3Auser`.
  - Non-200 → `statusError('GitHub device login', status)`.
  - `deviceCode = assertOpaque(value.device_code, 'GitHub device code')`.
  - `user_code` MUST match `/^[A-Z0-9-]{4,32}$/u` else `UsageError('device_response', 'GitHub returned an unsupported device-login response.')`.
  - `verification_uri` MUST parse as URL AND `verification.href === 'https://github.com/login/device'` EXACTLY (fixed allow-listed value, not just any HTTPS URL) — else `device_response` error.
  - `expires_in` MUST be finite, `1 <= x <= 1800`; `interval` (default 5) MUST be finite, `1 <= x <= 30` — else `device_response` error.
  - Returns `{ deviceCode, userCode: value.user_code, verificationUri: verification.href, expiresIn, interval }`.
- `pollDeviceFlow(flow, { fetchImpl, timeoutMs, now=()=>Date.now(), sleep=setTimeout-based })` (copilot.mjs:134-168) — **polling semantics**:
  - `deadline = now() + flow.expiresIn * 1000`.
  - `interval = flow.interval` (mutable, grows on slow_down).
  - Loop: `remaining = deadline - now()`; if `remaining <= 0` break (exit loop → falls through to final `throw device_expired`) — **critically, this check happens BEFORE sleeping, so a flow already past its deadline makes ZERO poll requests** (test: `expiresIn:5, interval:10` → sleeps `[5000]` then breaks with 0 requests, since remaining(5000) is clamped to sleep `Math.min(interval*1000, remaining)=5000`, then post-sleep `now()>=deadline` → break, 0 fetch calls).
  - `await sleep(Math.min(interval*1000, remaining))` — sleeps the smaller of the current interval or remaining time to deadline (never oversleeps past deadline).
  - After waking, re-check `now() >= deadline` → break (no request made if woke up exactly at/after deadline).
  - `POST GITHUB_DEVICE_TOKEN_URL` with `client_id`, `device_code`, `grant_type=urn:ietf:params:oauth:grant-type:device_code`.
  - Non-200 → `statusError('GitHub device login', status)`.
  - `access_token` present → validate via `assertOpaque` and return it (loop exits successfully).
  - Else switch on `value.error`:
    - `'authorization_pending'` → no-op, loop continues.
    - `'slow_down'` → `interval += 5` (GitHub spec: add 5s to polling interval).
    - `'access_denied'` → `UsageError('access_denied', 'GitHub device login was denied or cancelled.')` (covers both explicit denial and user cancellation).
    - `'expired_token'` → `UsageError('device_expired', 'GitHub device login expired.')`.
    - default/unknown → `UsageError('device_response', 'GitHub returned an unsupported device-login response.')`.
  - If loop exits via deadline (not via return/throw) → final `throw new UsageError('device_expired', 'GitHub device login expired.')`.
  - Test-verified wait sequences: normal pending+slow_down → `[1000,1000,6000]` (interval 1s→1s→6s after +5 slow_down); repeated slow_down → `[1000,6000,11000]` (1→6→11, +5 each time).
- `normalizeCopilotUsage(value)` (copilot.mjs:170-193):
  - Requires `value.quota_snapshots` to be a non-null, non-array object, else `UsageError('unsupported_response', 'GitHub Copilot returned an unsupported usage shape.')`.
  - Iterates fixed key list `['chat', 'completions', 'premium_interactions']`, label = key with `_` replaced by space (e.g. `"premium interactions"`).
  - Per window: `unlimited = source.unlimited === true`; `remaining = finiteNonNegative(source.remaining)`; `entitlement = finiteNonNegative(source.entitlement)`; `remainingPercent = finitePercent(source.percent_remaining)`. Skip window unless `unlimited` OR `remaining !== undefined` OR `remainingPercent !== undefined` (entitlement alone insufficient to include a window).
  - `resetAt = resetText(source.reset_at ?? value.quota_reset_date)` — falls back to the top-level shared reset date if the per-window one is absent.
  - Zero windows → `unsupported_response` error.
- `fetchCopilotUsage(token, { fetchImpl, timeoutMs })` (copilot.mjs:195-204): `GET COPILOT_USAGE_URL` with `copilotUsageHeaders`; non-200 → `statusError('GitHub Copilot', status)`; returns `normalizeCopilotUsage(value)`.
- `checkCopilotUsage({ fetchImpl=fetch, runCommand, login=false, showIdentity=false, writeLogin=stdout.write, timeoutMs, clock })` (copilot.mjs:206-230) — **top-level orchestration**:
  1. `discoverCopilotToken({ runCommand, validateToken: (candidate) => verifyGithubIdentity(candidate, {fetchImpl, timeoutMs}) })` — identity verification IS the token validator used during discovery (every candidate is identity-checked before acceptance).
  2. If no token discovered and `!login` → `UsageError('login_required', 'No usable GitHub token was found; rerun with --login to start device login.')` (test confirms only 1 HTTP call made — identity check — before this throw; the private Copilot endpoint is NEVER reached without a valid, identity-verified token).
  3. If no token and `login===true`: `startDeviceFlow`; `writeLogin('Open ${verificationUri} and enter code ${userCode}.')` — ONLY these two validated fields are ever shown to the user, never the raw device code or token; `pollDeviceFlow(flow, {..., ...clock})`; then `verifyGithubIdentity(token, ...)` again to get `loginName`.
  4. `windows = fetchCopilotUsage(token, ...)`.
  5. `report = [formatUsageReport('GitHub Copilot', windows), 'GitHub identity verified.']`; if `showIdentity` → also push `'GitHub login: ${loginName}'`. Joined with `\n`.
  - **Identity gate ordering** (test `'identity failure stops before the Copilot private endpoint'`): identity check happens strictly before the usage endpoint is ever called; a 401 on identity aborts with `login_required` and exactly 1 fetch call total.

---
## 3. Codex configured-base-url fallback — summarised cross-reference
See §2.2 for `parseCodexConfig` full parsing rules and `FALLBACK_STATUSES = {404,405,410,501}`. `validateTrustedBaseUrl` full rule set is in §1 (core.mjs:93-116): requires exact origin match between the configured `base_url` and the user-supplied `--trust-configured-origin`, HTTPS-only both sides, no userinfo/query/hash on either, trusted origin arg must be bare (`pathname==='/'`) — protects against a malicious/compromised local config silently exfiltrating the Bearer token to an arbitrary attacker-controlled HTTPS host; the user must explicitly, exactly opt in per run (not persisted).

---
## 4. Copilot device flow — summary table
| Aspect | Value |
|---|---|
| Client ID | `Iv1.b507a08c87ecfe98` |
| Device code endpoint | `POST https://github.com/login/device/code`, form body `client_id`+`scope=read:user` |
| Token endpoint | `POST https://github.com/login/oauth/access_token`, form body `client_id`+`device_code`+`grant_type=urn:ietf:params:oauth:grant-type:device_code` |
| Verification URI | must equal exactly `https://github.com/login/device` |
| user_code format | `/^[A-Z0-9-]{4,32}$/u` |
| expires_in bounds | 1–1800s |
| interval bounds | 1–30s |
| slow_down behaviour | `interval += 5` each occurrence, unbounded accumulation |
| pending | silent retry |
| denied/cancelled | `access_denied` GitHub OAuth error → `UsageError('access_denied', ...)` |
| expired | `expired_token` GitHub error OR local deadline reached → `UsageError('device_expired', ...)` |
| Token discovery order | (1) `git config --global --get github.copilot.oauthToken` (2) `git config --system --get github.copilot.oauthToken` (3) `gh auth token --hostname github.com` — never `--local` |
| Identity gate | `GET https://api.github.com/user` with 3-header identity request MUST succeed and return a valid-format `login` before ANY candidate token (discovered or device-flow-issued) is accepted or used against the usage endpoint |
| `--show-identity` | Only appends `GitHub login: <login>` line to output when explicitly requested; identity is otherwise verified but never printed |

---
## 5. Normalized data model — exact window shape
Common fields across normalizers (not all present simultaneously — presence is provider/window dependent):
```ts
type UsageWindow = {
  label: string;                 // fixed provider-defined label, e.g. "five hour", "primary window", "chat"
  usedPercent?: number;          // 0-100, Claude + Codex rate-limit windows only
  remainingPercent?: number;     // 0-100; Claude/Codex = 100-usedPercent (derived); Copilot = source.percent_remaining (raw, independent field)
  remaining?: number;            // absolute count, Codex credits + Copilot quota windows
  entitlement?: number;          // absolute count, Copilot only (total allotment)
  unlimited?: boolean;           // Copilot only; true suppresses all percent/count detail rendering
  resetAt?: string;               // ISO-8601 string via resetText(), optional
};
```
Absolute-count fields used only by codex/copilot normalizers: `remaining` (Codex `credits.balance`; Copilot `quota_snapshots.<key>.remaining`), `entitlement` (Copilot `quota_snapshots.<key>.entitlement`). Claude never emits `remaining`/`entitlement`/`unlimited`.

---
## 6. CLI surface (`scripts/*.mjs`)

All three scripts: `main(args = process.argv.slice(2), io)` exported; auto-runs `process.exitCode = await main()` only when invoked as the direct entry (`import.meta.url === pathToFileURL(process.argv[1]).href`), so they're safely importable in tests. All wrap the underlying `check*Usage` in `runSafely` (core.mjs) → stdout gets the formatted report + trailing `\n` on success (exit 0), stderr gets `Usage check failed [<code>]: <message>\n` on failure (exit 1); Claude's broad-permission warning is separately written to stderr mid-run via `warn`.

### `scripts/claude-usage.mjs` (16 lines)
- Flags: **none accepted**. Any argument → `UsageError('arguments', 'Claude usage accepts no arguments.')` (exit 1).
- Wires `warn` to `(io?.stderr ?? process.stderr).write`.

### `scripts/codex-usage.mjs` (17 lines)
- Flags: `--trust-configured-origin <https-origin>` — must be EXACTLY 2 args (`args.length===2 && args[0]==='--trust-configured-origin'`), else (any other non-empty arg combination) → `UsageError('arguments', 'Use --trust-configured-origin <https-origin> only when you trust the configured Codex provider.')`.
- Zero args → `{}` (no trustedOrigin; fallback unavailable unless config exposes none needing it).

### `scripts/copilot-usage.mjs` (19 lines)
- Flags: `--login`, `--show-identity` (any combination/order; both boolean, order-independent, can coexist). Any other token → `UsageError('arguments', 'GitHub Copilot usage accepts only --login and --show-identity.')`.
- Wires `writeLogin` to `(io?.stdout ?? process.stdout).write` (login prompt goes to STDOUT, not stderr — distinct from the final failure/report split).

### Exit codes (all scripts, via `runSafely`)
- `0` — success, report on stdout.
- `1` — any thrown error (UsageError or unexpected), message on stderr.

---
## 7. `agents/openai.yaml` — verbatim + purpose
This is an agent-skill interface descriptor (not OpenAI API config) — declares how this skill is surfaced/invoked by an agent harness (display name, description, default prompt suggestion). It is NOT provider configuration; it does not affect runtime behaviour of the scripts.
```yaml
interface:
  display_name: "Usage allowance checks"
  short_description: "Safely report Claude, Codex, or Copilot allowance"
  default_prompt: "Use $usage-allowance-checks to run one explicit, read-only provider usage allowance report without automatic enforcement."
```

---
## 8. Tests (`tests/usage-allowance.test.mjs`, 385 lines) — every case + invariant
Mocking strategy: `memoryFs(files)` fakes `{stat, readFile}` (no real disk I/O); `json(value,status,headers)` builds a real `Response` object (uses actual `fetch`/`Response` globals, not a full fetch mock — only `fetchImpl` function itself is injected); `runCommand` injected function replaces `execFile` for Copilot; `clock: {now, sleep}` injected into `pollDeviceFlow`/`checkCopilotUsage` to avoid real timers. All tests use `node:test` + `node:assert/strict`. A `CANARY_TOKEN`/`CANARY_DEVICE` constant is asserted absent from all outputs/errors to catch secret leakage.

1. **"Claude reads the expected credential, warns on broad mode, and normalizes usage"** — invariant: correct URL called, `redirect:'manual'`, `anthropic-beta` header set, 25% used → "75% available" in output, exactly 1 broad-permission warning emitted (mode 0644), no secret leakage.
2. **"credential parsing rejects malformed JSON, missing token, and expired token safely"** — invariant: `{bad`, `{}`, and an expired-token payload all reject with `UsageError` (not a raw JS error).
3. **"HTTP helper refuses redirects, oversized bodies, network errors, and non-canonical hosts"** — invariant: 302 → `redirect_refused`; oversized JSON with `maxBytes:16` → `response_too_large`; fetch-throw → `network` (no secret in message); malformed JSON → `response_json`; abort → `timeout` (no secret in message); URL not in `allowedUrls` → `endpoint_refused` (exact allow-list, near-miss typo domain rejected).
4. **"provider status errors are fixed and sanitized for 401, 403, and 429"** — invariant: none of these ever leak the raw response body/token into the thrown error message.
5. **"Codex config parsing selects the active provider without dumping config"** — invariant: `parseCodexConfig` returns only the resolved URL string, never the whole config.
6. **"Codex uses primary first and permits fallback only for an exact trusted HTTPS origin"** — invariant: without `trustedOrigin` a 404 primary throws `untrusted_origin` (does NOT silently fall back); with correct `trustedOrigin` it calls exactly `[CODEX_USAGE_URL, 'https://trusted.example/v1/api/codex/usage']` in order, same Bearer token on both, output has no secret/account-id leakage.
7. **"Codex does not fallback for auth, rate-limit, or arbitrary configured origins"** — invariant: 401/429 primary statuses make exactly 1 fetch call each (no fallback attempted, `FALLBACK_STATUSES` exclusivity); a 404 with an HTTP (non-HTTPS) configured base_url + matching trustedOrigin still throws `untrusted_origin` (protocol enforcement can't be bypassed by trusting an http origin).
8. **"Copilot token discovery checks only named global/system Git and gh sources in order"** — invariant: exact 3-call sequence in exact order, `--local` and raw token value never appear in the command args list (defends against accidentally shelling out the secret as a literal arg elsewhere / using local scope).
9. **"Copilot skips malformed and unauthorized candidates before using a later valid source"** — invariant: a denied (401) system-config token is skipped in favor of the gh-sourced token; 3 source calls, 3 HTTP calls in order `[USER, USER, USAGE]`; no leaked candidate/username substrings in final output.
10. **"Copilot device flow validates display fields and handles pending plus slow_down"** — invariant: device-code POST body has correct `client_id`/`scope=read:user`; token POST body has correct `grant_type`; pending→no-op, slow_down→+5s; final wait sequence `[1000,1000,6000]`.
11. **"Copilot device flow handles denied, expired, and cancelled outcomes safely"** — invariant: `access_denied`→code `access_denied`, `expired_token`→code `device_expired`; neither leaks `CANARY_DEVICE` in the message.
12. **"Copilot device polling never requests at or after the local deadline"** — invariant: `expiresIn:5, interval:10` makes ZERO HTTP requests (clamped single sleep of 5000ms then deadline reached) and still throws `device_expired` — protects the local deadline as authoritative even if declared interval exceeds it.
13. **"Copilot device polling applies repeated slow_down increments within the deadline"** — invariant: cumulative wait sequence `[1000,6000,11000]` for two consecutive slow_downs (interval 1→6→11).
14. **"Copilot starts explicit device login only after every discovered candidate is unusable"** — invariant: 3 denied discovery candidates (each triggers a distinct identity 401) exhausted BEFORE device flow starts; exactly 1 login message written; none of the denied token substrings or canary leak into final output; first 4 URLs called are `[USER,USER,USER,DEVICE_CODE]`.
15. **"Copilot validates identity before usage and sends the required usage headers"** — invariant: call order `[USER, USAGE]`; identity request headers are EXACTLY `{accept, authorization, user-agent}` (no editor-version/plugin/api-version keys present at all, not just undefined); usage request headers are EXACTLY the 6-key `copilotUsageHeaders` set with exact values; output contains "GitHub identity verified" and never the login name (since `--show-identity` not passed) nor canary.
16. **"identity failure stops before the Copilot private endpoint"** — invariant: identity 401 → `login_required` error, exactly 1 HTTP call total (private usage endpoint never reached without valid identity).
17. **"safe runner never emits secrets from unexpected errors"** — invariant: `runSafely` catching a raw `Error(CANARY_TOKEN)` returns exit code 1 and NEVER includes the canary in stdout+stderr combined (proves the generic `[unexpected]` fixed-string branch, not error.message interpolation).

---
## 9. Security invariants checklist (MUST be preserved in any rewrite)

- [ ] **Fixed allow-listed endpoints only** — every outbound HTTPS request URL must match an exact string in a hardcoded allow-list (`validateExactHttpsUrl`); no substring/prefix/origin-only matching, no user-suppliable arbitrary URLs except the one narrowly-scoped, explicitly-opted-in Codex fallback origin.
- [ ] **HTTPS-only, no userinfo, no fragment** on every validated URL (both the exact-allowlist path and the trusted-origin path); reject `username`/`password`/`hash` on URLs unconditionally.
- [ ] **Redirects are never followed** — `redirect: 'manual'` on every request; any 3xx response is a hard failure (`redirect_refused`), never silently followed.
- [ ] **Codex fallback requires an explicit, exact, per-invocation origin opt-in** (`--trust-configured-origin`) that must equal the configured base_url's origin exactly (protocol+host+port); the trust argument itself must be a bare origin (no path/query/hash); fallback is only even considered on a closed, fixed set of statuses (`404,405,410,501`) — never on auth/rate-limit statuses, which fail closed immediately.
- [ ] **Response size bounding** — both credential files (≤1 MiB) and HTTP responses (≤256 KiB) are bounded before parsing, checked via both `content-length` header (fast path) and actual streamed/read byte count (defends against a lying/missing header).
- [ ] **Strict JSON shape validation** — every parsed JSON body must be a non-null, non-array object before use; malformed/wrong-shape data becomes a generic `unsupported_response`/`response_json`/`credential_json` error, never a raw parse exception surfaced to the user.
- [ ] **Credential/token opacity checks** — all bearer tokens/identifiers pass through `assertOpaque`/`assertIdentifier`: length-bounded, single-line, no control/whitespace characters, rejecting anything that looks like it could be multi-value, injected, or a parsing artifact.
- [ ] **No secret ever appears in a thrown error message or log line** — every catch block that wraps an underlying error discards the original message and substitutes a fixed, sanitized string (`network`, `timeout`, `[unexpected]: The check stopped safely.`, etc.); verified exhaustively by the canary-token tests.
- [ ] **File-permission warning, never auto-remediation** — broad credential file permissions (group/other rwx) trigger a stderr warning only; the tool never chmod's or otherwise mutates the user's credential file.
- [ ] **GitHub token discovery is scoped to named global/system config keys and `gh` CLI only** — explicitly excludes local/repo-scoped git config (`--local`) and never dumps full git config output; each candidate is validated via a live identity check before use, and unauthorized/malformed candidates are skipped (not fatal) in favor of later sources.
- [ ] **Identity verification is a mandatory gate before any private/internal usage endpoint call** — for Copilot, `GET /user` must succeed and return a validly-formatted login before `copilot_internal/user` is ever called, for both discovered and freshly-issued (device-flow) tokens.
- [ ] **Device-flow secrets stay in-memory only** — device code and resulting access token are never persisted to disk or printed; only the GitHub-provided, format-validated `verification_uri` (pinned to the exact literal `https://github.com/login/device`) and `user_code` are shown to the user, and even the login/username is opt-in-only via `--show-identity`.
- [ ] **Device-flow polling honors GitHub's protocol precisely and fails closed on deadline** — `authorization_pending` retries, `slow_down` increases interval by +5s (unbounded, cumulative), `access_denied`/`expired_token` are terminal errors, and the local wall-clock deadline (from `expires_in`) is authoritative and checked both before and after each sleep — a poll is never issued at/after the deadline even if the server declares a longer window.
- [ ] **No background polling, persistence, or auto-invocation** — each script performs exactly one bounded, explicit, foreground provider check per invocation; no daemonization, no credential caching across runs, no automatic spawn gating (per SKILL.md's explicit scope statement).
- [ ] **CLI argument surfaces are minimal, explicit allow-lists** — unknown/extra flags are hard errors (`arguments` UsageError), not silently ignored, preventing flag-injection-style privilege creep (e.g. accidentally accepting an arbitrary origin flag on the Claude/Copilot scripts).
- [ ] **Exit-code/stream discipline** — success always writes exactly the formatted report to stdout with code 0; failure always writes exactly one sanitized line to stderr with code 1; this must be preserved for any downstream automation/orchestrator (e.g. the future ranking C