---
kind: feature-tasks
feature: F17-provider-copilot
version: "1.0"
task_count: 8
---

# F17 — provider-copilot: Tasks

## Task graph

```mermaid
graph LR
    T1[T1 skeleton + Descriptor] --> T2[T2 identity gate]
    T1 --> T3[T3 normalization]
    T1 --> T4[T4 device flow]
    T2 --> T5[T5 usage fetch]
    T3 --> T5
    T2 --> T6[T6 orchestration Fetch]
    T5 --> T6
    T5 --> T7[T7 canary]
    T6 --> T7
    T3 --> T8[T8 golden fixtures + parity]
    T5 --> T8
```

## Task F17-T1: Create the package skeleton and register the Copilot descriptor

**Depends on:** none
**Files:**
- create `internal/usage/provider/copilot/copilot.go`
- create `internal/usage/provider/copilot/copilot_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.1, §2.11, D1, D13-D14`, `specs/features/F17-provider-copilot/CONTRACTS.md §2`, `specs/global/CONTRACTS.md §1.3`, `docs/plan/annex-a-provider-matrix.md §3.3, §5`

**Instructions:**
1. Write the test file first: `usage.Lookup("copilot")` must succeed after import.
2. Implement `copilot.go`: package doc (port of `usage-allowance-checks/lib/copilot.mjs`); the five URL/client-id constants and `APIVersion` copied exactly from `specs/features/F17-provider-copilot/CONTRACTS.md §1` (verbatim from `copilot.mjs:9-15`); `const IdentityUserAgent = "which-model/0.4.0"`; `init()` registering the Descriptor literal copied exactly from CONTRACTS §2.
3. Use canonical types from F11 (`usage.Descriptor`, `usage.AuthSource`, `usage.ShellSpec`, `usage.OAuthSpec`, `usage.WindowSpec`, `usage.KindSubscription`, `usage.UnitRequests`); do not define or extend them. If F12's `ShellSpec`/`OAuthSpec` field names differ from the CONTRACTS §2 literal, use F12's names — the VALUES (command, args, timeout 3s, max bytes 32 KiB, client id, URLs, scope) are binding.
4. Until F17-T5/T6, `var Fetch usage.FetchFunc = func(context.Context, usage.Credential, *http.Client) (usage.Snapshot, error) { return usage.Snapshot{}, nil }` and `var ValidateIdentity = func(context.Context, string, *http.Client) (string, error) { return "", nil }` placeholders; replace them in F17-T2 and F17-T6.
5. Do not add other source files in this task.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `usage.Lookup("copilot")` | ok |
| 2 | descriptor `ID`, `DisplayName` | `"copilot"`, `"GitHub Copilot"` |
| 3 | descriptor `Kind`, `Tier` | `usage.KindSubscription`, `1` |
| 4 | descriptor `Timeout`, `CacheTTL` | `15 * time.Second`, `60 * time.Second` |
| 5 | `Windows` IDs/labels in order | `[premium/premium interactions, chat/chat, completions/completions]`, all Optional, all UnitRequests |
| 6 | `Auth` kinds in order | `[AuthEnvVar, AuthShell, AuthShell, AuthShell, AuthOAuthDeviceFlow]` |
| 7 | first shell entry | `Command == "git"`, `Args == ["config","--global","--get","github.copilot.oauthToken"]`, `Timeout == 3s`, `MaxOutputBytes == 32768` |
| 8 | second/third shell entries | `git config --system ...`; `gh auth token --hostname github.com` |
| 9 | every `Auth` entry has `Validate != nil` | as stated (identity gate on all candidates) |
| 10 | device-flow entry | `ClientID == "Iv1.b507a08c87ecfe98"`, `DeviceCodeURL == "https://github.com/login/device/code"`, `TokenURL == "https://github.com/login/oauth/access_token"`, `Scope == "read:user"` |
| 11 | env entry | `EnvVar == "COPILOT_API_TOKEN"` |
| 12 | constants | `CopilotUsageURL == "https://api.github.com/copilot_internal/user"`, `GitHubUserURL == "https://api.github.com/user"`, `APIVersion == "2025-04-01"` |

**Acceptance criteria:**
- [ ] `go build ./internal/usage/provider/copilot/...` succeeds
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] descriptor literal matches CONTRACTS §2 field-for-field; the three git/gh source args never contain `--local` (`.mjs` test 8 parity)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`

## Task F17-T2: Port the GitHub identity gate

**Depends on:** F17-T1
**Files:**
- create `internal/usage/provider/copilot/copilot_identity.go`
- create `internal/usage/provider/copilot/copilot_identity_test.go`
- edit `internal/usage/provider/copilot/copilot.go` (replace the `ValidateIdentity` placeholder)

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.5, D4, D6`, `specs/features/F17-provider-copilot/CONTRACTS.md §3, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §4 (`verifyGithubIdentity`, copilot.mjs:99-110)

**Instructions:**
1. Write the test file first with a stub `http.RoundTripper` (record requests; canned responses).
2. Implement the private `requestJSON` helper exactly as F15-T4 step 2 (same codes/messages; provider-agnostic).
3. Implement `ValidateIdentity(ctx, token, client)` (replacing the placeholder): `GET GitHubUserURL`, allow-list `[GitHubUserURL]`, headers exactly `{Accept: application/vnd.github+json, Authorization: Bearer <token>, User-Agent: IdentityUserAgent}` — exactly three headers, no editor/api-version keys. Non-200 → `mapStatus("GitHub identity", status)` per CONTRACTS §6. Decode object; `login` must be a string matching `^[A-Za-z0-9-]{1,39}$` (compile the regex once as a package var); failure → `Error{Code: "unsupported_response", Message: "GitHub returned an unsupported identity response."}`. Return the login.
4. The token appears only in the `Authorization` header and never in error text; the login never appears in error text.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | stub 200 `{"login":"synthetic-user","id":42}` | `"synthetic-user"`, no error; request URL == `GitHubUserURL`; exactly 3 headers: `Accept: application/vnd.github+json`, `Authorization: Bearer canary-secret-token-123`, `User-Agent: which-model/0.4.0` |
| 2 | stub 401 body `{"message":"canary-secret-token-123"}` | `unauthorized`, message `GitHub identity rejected the credential.`, no canary |
| 3 | stub 403 | `unauthorized` |
| 4 | stub 429 | `rate_limited`, `GitHub identity rate-limited the usage request.` |
| 5 | stub 500 | `provider_status`, `GitHub identity usage is unavailable (HTTP 500).` |
| 6 | stub 200 `{"login":"bad login!"}` | `unsupported_response`, `GitHub returned an unsupported identity response.` |
| 7 | stub 200 `{"id":42}` (no login) | `unsupported_response` |
| 8 | stub 200 `{"login":"a"}` | `"a"` (1-char login valid) |
| 9 | stub 200 `{"login":123}` (non-string) | `unsupported_response` |
| 10 | stub 302 | `redirect_refused` |
| 11 | stub transport error containing canary | `network`, no canary |
| 12 | stub 200 `{bad` | `response_json` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] header set for case 1 matches `githubIdentityHeaders` from `copilot.mjs` (3 headers, sorted-key-tested) except the User-Agent value per SPEC D4
- [ ] no file outside the Files list modified (the `copilot.go` edit is the declared placeholder replacement)

`go test ./internal/usage/provider/copilot/...`

## Task F17-T3: Port the Copilot usage normalizer

**Depends on:** F17-T1
**Files:**
- create `internal/usage/provider/copilot/copilot_normalize.go`
- create `internal/usage/provider/copilot/copilot_normalize_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.8, D7-D8, D13`, `specs/features/F17-provider-copilot/CONTRACTS.md §4, §5`, `docs/plan/research/usage-allowance-checks-spec.md` §4 (normalizeCopilotUsage, copilot.mjs:197-223)

**Instructions:**
1. Write the test file first, table-driven.
2. Implement `NormalizeUsage(raw []byte) ([]usage.Window, error)`:
   - Decode object; failure → `response_json` (`The provider returned unsupported JSON.`).
   - `snapshots := value.quota_snapshots`; absent, non-object, or array → `unsupported_response` (`GitHub Copilot returned an unsupported usage shape.`).
   - Iterate keys in the fixed order `chat`, `completions`, `premium_interactions` (the `.mjs` order — not map order). Per window: `source := snapshots[key]`; skip when absent/non-object. `unlimited := source.unlimited == true`; `remaining = finiteNonNegative(source.remaining)`; `entitlement = finiteNonNegative(source.entitlement)`; `remainingPercent = finitePercent(source.percent_remaining)`; skip the window unless `unlimited || remaining != nil || remainingPercent != nil` (entitlement alone is not enough — `.mjs` parity).
   - Window: ID = key (`premium_interactions` → `premium`), Label = key with `_` → ` ` (e.g. `premium interactions`), Unit requests; `Unlimited = unlimited` (only true when the source said so); `Remaining = remaining`; `Limit = entitlement`; `UsedPercent = 100 - remainingPercent` when present (SPEC D7); `ResetsAt = resetTime(source.reset_at ?? value.quota_reset_date)` (ISO string, date-only strings parse at UTC midnight, epoch number with `> 10_000_000_000` = ms; unparseable → nil); `UsageKnown: true`.
   - Zero windows → `unsupported_response` (same message).
3. `finitePercent`/`finiteNonNegative`/`resetTime` are private helpers (ports of `core.mjs:202-224`).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `{"quota_snapshots":{"chat":{"entitlement":300,"remaining":225,"percent_remaining":75}},"quota_reset_date":"2030-01-01"}` (copy from `usage-allowance.test.mjs` case 15) | 1 window: ID `chat`, Label `chat`, Unit requests, Remaining 225, Limit 300, UsedPercent 25, ResetsAt `2030-01-01T00:00:00Z`, UsageKnown true |
| 2 | `{"quota_snapshots":{"chat":{"remaining":10,"entitlement":20}}}` (copy from `usage-allowance.test.mjs` case 9) | 1 window: Remaining 10, Limit 20, UsedPercent nil, ResetsAt nil |
| 3 | `{"quota_snapshots":{"chat":{"remaining":10},"completions":{"remaining":5},"premium_interactions":{"remaining":1,"entitlement":2}}}` | 3 windows in order `chat`, `completions`, `premium`; premium Label `premium interactions` |
| 4 | `{"quota_snapshots":{"chat":{"unlimited":true}}}` | 1 window: Unlimited true, no Remaining/Limit/UsedPercent |
| 5 | `{"quota_snapshots":{"chat":{"entitlement":300}}}` | entitlement alone → no windows → `unsupported_response` |
| 6 | `{"quota_snapshots":{"chat":{"percent_remaining":150}}}` | percent out of range → no windows → `unsupported_response` |
| 7 | `{"quota_snapshots":[]}` | `unsupported_response` (array) |
| 8 | `{"garbage":1}` | `unsupported_response` (absent) |
| 9 | `{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"2030-01-01T00:00:00Z"},"completions":{"remaining":5,"reset_at":"2030-01-02T00:00:00Z"}}}` | per-window reset_at beats `quota_reset_date` (windows carry their own resets) |
| 10 | `{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"not-a-date"}}}` | window present, ResetsAt nil |
| 11 | `not json` | `response_json` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] case 1 values match `normalizeCopilotUsage` from `copilot.mjs` for the same fixture (remaining 225, entitlement 300, percent_remaining 75 → UsedPercent 25 per SPEC D7, reset `2030-01-01T00:00:00Z`)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`

## Task F17-T4: Port the GitHub device flow (start + poll)

**Depends on:** F17-T1
**Files:**
- create `internal/usage/provider/copilot/copilot_device.go`
- create `internal/usage/provider/copilot/copilot_device_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.9, D9-D11`, `specs/features/F17-provider-copilot/CONTRACTS.md §1, §3, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §4 (device flow, copilot.mjs:122-195)

**Instructions:**
1. Write the test file first. For polling, inject `PollOptions{Now, Sleep}` with a fake clock exactly like the `.mjs` tests (`now` starts 0; `sleep` advances it and records the wait).
2. Implement `StartDeviceFlow(ctx, client)`:
   - POST `GitHubDeviceCodeURL`, allow-list `[GitHubDeviceCodeURL]`, headers `{Accept: application/json, Content-Type: application/x-www-form-urlencoded}`, body `client_id=Iv1.b507a08c87ecfe98&scope=read:user` (form-encoded; `url.Values.Encode()`).
   - Non-200 → `mapStatus("GitHub device login", status)`.
   - Validate (each failure → `unsupported_response` `GitHub returned an unsupported device-login response.`): `device_code` passes `security.ValidateOpaqueToken`; `user_code` matches `^[A-Z0-9-]{4,32}$`; `verification_uri` parses as URL with `href == "https://github.com/login/device"` EXACTLY (scheme+host+path; trailing slash or extra path → reject); `expires_in` (number or numeric string) finite 1..1800; `interval` (default 5 when absent) finite 1..30.
   - `DeviceFlow{DeviceCode, UserCode, VerificationURI: "https://github.com/login/device", ExpiresIn, Interval}`.
3. Implement `PollDeviceFlow(ctx, client, flow, opts)` (nil opts → `time.Now`/`time.Sleep`):
   - `deadline := now() + time.Duration(flow.ExpiresIn)*time.Second`.
   - Loop: `remaining := deadline.Sub(now())`; `remaining <= 0` → break (never request at/after the deadline); sleep `min(interval*time.Second, remaining)`; `now() >= deadline` → break.
   - POST `GitHubDeviceTokenURL`, allow-list `[GitHubDeviceTokenURL]`, same headers; body `client_id`, `device_code`, `grant_type=urn:ietf:params:oauth:grant-type:device_code`.
   - Non-200 → `mapStatus("GitHub device login", status)`. `access_token` present (non-empty string) → `security.ValidateOpaqueToken` → return it.
   - `error` switch: `authorization_pending` → continue; `slow_down` → `interval += 5` (no upper bound); `access_denied` → `access_denied` `GitHub device login was denied or cancelled.`; `expired_token` → `device_expired` `GitHub device login expired.`; other → `unsupported_response` device-login message.
   - Loop exit → `device_expired` `GitHub device login expired.`
4. Every message is the fixed CONTRACTS §6 string; device codes never appear in messages.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | start: stub 200 `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}` | `DeviceFlow{canary-device-code-456, ABCD-1234, https://github.com/login/device, 60, 1}`; request method POST, body contains `client_id=Iv1.b507a08c87ecfe98` and `scope=read%3Auser` |
| 2 | start: `user_code:"ab"` (too short) or `user_code:"bad code!"` (invalid chars) | `unsupported_response`, device-login message |
| 3 | start: `verification_uri:"https://github.com/login/device/"` (trailing slash) or `"https://evil.example/"` | `unsupported_response` |
| 4 | start: `expires_in:0` / `expires_in:1801` / `interval:0` / `interval:31` | `unsupported_response` (each) |
| 5 | start: stub 401 | `unauthorized`, `GitHub device login rejected the credential.` |
| 6 | poll: replies `[authorization_pending, slow_down, {access_token: canary-secret-token-123}]`, `ExpiresIn: 60, Interval: 1`, fake clock | token = canary; waits `[1000, 1000, 6000]` (`.mjs` test 10 parity) |
| 7 | poll: reply `{error: access_denied}` | `access_denied`, message has no device code |
| 8 | poll: reply `{error: expired_token}` | `device_expired` |
| 9 | poll: `ExpiresIn: 5, Interval: 10`, reply would be a token | `device_expired`; waits `[5000]`; requests == 0 (never polls at/after deadline; `.mjs` test 12 parity) |
| 10 | poll: replies `[slow_down, slow_down, {access_token: canary-secret-token-123}]`, `ExpiresIn: 30, Interval: 1` | waits `[1000, 6000, 11000]` (`.mjs` test 13 parity) |
| 11 | poll: reply `{error: "unknown_error"}` | `unsupported_response` |
| 12 | poll: replies `[authorization_pending, authorization_pending, authorization_pending]`, `ExpiresIn: 3, Interval: 1` | `device_expired` (deadline reached) |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] cases 6/9/10 reproduce the `.mjs` wait sequences and call counts exactly (`usage-allowance.test.mjs` cases 10/12/13)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`

## Task F17-T5: Port the Copilot usage fetch (six headers + status mapping)

**Depends on:** F17-T2, F17-T3
**Files:**
- create `internal/usage/provider/copilot/copilot_fetch.go`
- create `internal/usage/provider/copilot/copilot_fetch_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.6, §2.7, D5`, `specs/features/F17-provider-copilot/CONTRACTS.md §3, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §4 (`fetchCopilotUsage`, copilot.mjs:111-120)

**Instructions:**
1. Write the test file first with the stub transport (record requests).
2. Implement the private `fetchUsage(ctx, token, client) ([]usage.Window, error)`:
   - `GET CopilotUsageURL`, allow-list `[CopilotUsageURL]`, headers exactly CONTRACTS §3 (six: `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`, `Editor-Version: vscode/1.96.2`, `Editor-Plugin-Version: copilot-chat/0.26.7`, `User-Agent: GitHubCopilotChat/0.26.7`, `X-GitHub-Api-Version: 2025-04-01`).
   - Non-200 → `mapStatus("GitHub Copilot", status)`. 200 → `NormalizeUsage(body)`.
3. `mapStatus` is the package-private status mapper (same shape as F15-T5's): 401/403 → `unauthorized`; 429 → `rate_limited`; else `provider_status` — messages per CONTRACTS §6 with the provider name `GitHub Copilot`.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | stub 200 with case-15 fixture | 1 window: `chat` Remaining 225, Limit 300, UsedPercent 25; request headers sorted == `[Accept, Authorization, Editor-Plugin-Version, Editor-Version, User-Agent, X-GitHub-Api-Version]`; `Authorization: Bearer canary-secret-token-123`; `Editor-Version: vscode/1.96.2`; `Editor-Plugin-Version: copilot-chat/0.26.7`; `User-Agent: GitHubCopilotChat/0.26.7`; `X-GitHub-Api-Version: 2025-04-01`; `Accept: application/vnd.github+json` (`.mjs` test 15 parity) |
| 2 | stub 401 | `unauthorized`, `GitHub Copilot rejected the credential.` |
| 3 | stub 403 | `unauthorized` |
| 4 | stub 429 | `rate_limited`, `GitHub Copilot rate-limited the usage request.` |
| 5 | stub 500 | `provider_status`, `GitHub Copilot usage is unavailable (HTTP 500).` |
| 6 | stub 302 | `redirect_refused` |
| 7 | stub 200 `{bad` | `response_json` |
| 8 | stub 200 `{}` | `unsupported_response` |
| 9 | stub transport error containing canary | `network`, no canary |
| 10 | ctx already cancelled | `timeout` (deadline) |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] header set for case 1 matches `copilotUsageHeaders` from `copilot.mjs` (6 headers, sorted-key-tested) and the `Bearer` scheme (SPEC D5)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`

## Task F17-T6: Port the Copilot orchestration Fetch

**Depends on:** F17-T5
**Files:**
- create `internal/usage/provider/copilot/copilot_check.go`
- create `internal/usage/provider/copilot/copilot_check_test.go`
- edit `internal/usage/provider/copilot/copilot.go` (replace the `Fetch` placeholder)

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.3, §2.4, §2.10, D3, D12`, `specs/features/F17-provider-copilot/CONTRACTS.md §6, §8`, `docs/plan/research/usage-allowance-checks-spec.md` §4 (`checkCopilotUsage`, copilot.mjs:244-285)

**Instructions:**
1. Write the test file first with the stub transport; scenarios use the canary token and case-15/case-9 fixtures.
2. Implement `Fetch(ctx, cred, client)` (replace placeholder), the port of `checkCopilotUsage` minus the interactive `--login` leg:
   - `cred.Token == ""` → `Snapshot{Provider:"copilot", Failure:&usage.Failure{Code:"login_required", Message:"No usable GitHub token was found; rerun with --login to start device login."}, FetchedAt: time.Now().UTC(), Source: usage.SourceOAuth, Confidence:"live"}` — no HTTP calls.
   - Else: `login, err := ValidateIdentity(ctx, cred.Token, client)`; error → Failure with the error's code/message (hard failure — never a fallback to other sources).
   - `windows, err := fetchUsage(ctx, cred.Token, client)`; error → Failure.
   - Success → `Snapshot{Provider:"copilot", Windows: windows, UsageKnown: any window satisfies UsageKnown && !Synthetic, Account: login, FetchedAt: now UTC, Source: usage.SourceOAuth, Confidence:"live"}`.
3. The invariant: the usage call MUST NOT happen unless `ValidateIdentity` succeeded in this same run.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | cred token = canary; stub USER 200 `{"login":"synthetic-user","id":canary}`; usage 200 case-15 fixture | Snapshot: `chat` window per case 15; `Account == "synthetic-user"`; request order `[GitHubUserURL, CopilotUsageURL]` (`.mjs` test 15 parity) |
| 2 | cred token = canary; stub USER 401; usage stub would return 200 | Failure `unauthorized`; `Account` unset; usage URL NEVER requested (identity failure stops before the private endpoint — `.mjs` test 16 parity) |
| 3 | cred token = canary; stub USER 200 `{"login":"bad login!"}` | Failure `unsupported_response`; usage never requested |
| 4 | empty cred | Failure `login_required`, verbatim message; ZERO HTTP calls |
| 5 | cred token; USER 200; usage 401 | Failure `unauthorized`, `GitHub Copilot rejected the credential.` |
| 6 | cred token; USER 200; usage 200 case-9 fixture | Snapshot: `chat` Remaining 10, Limit 20; `Account == "synthetic-user"` |
| 7 | cred token; USER 200 `{"login":"octocat"}`; usage 200 `{bad` | Failure `response_json` |
| 8 | cred token; USER 200; usage 200 `{}` | Failure `unsupported_response` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] call order and counts for case 1 match `.mjs` test 15 (`[USER, USAGE]`)
- [ ] no file outside the Files list modified (the `copilot.go` edit is the declared placeholder replacement)

`go test ./internal/usage/provider/copilot/...`

## Task F17-T7: Canary-test every secret-touching path

**Depends on:** F17-T5, F17-T6
**Files:**
- create `internal/usage/provider/copilot/copilot_canary_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §3`, `specs/global/SPEC.md §6 item 5`, `docs/plan/research/usage-allowance-checks-spec.md` §9

**Instructions:**
1. Use `security.WithCanary(canary, fn)` around each scenario.
2. Canary values: token `"canary-secret-token-123"`, device code `"canary-device-code-456"`, login `"synthetic-user"`, body marker `"canary-body-marker-42"`.
3. Scenarios: (a) canary token with 401 identity response whose body echoes the canary; (b) canary token with network error; (c) identity 200 with `{"login":"synthetic-user","id":<canary>}` then usage 200 — assert no canary and no `synthetic-user` in the Snapshot JSON beyond `Account` (the `.mjs` asserts the login does not appear in text output without `--show-identity`; the Snapshot field itself is allowed, rendering is F24's gate); (d) canary body marker inside a usage 200 body; (e) device flow with canary device code and an `access_denied` reply (message must not echo the code — `.mjs` test 11); (f) full Fetch success — assert output-relevant strings never match `/denied-global|denied-system|denied-gh|canary-secret/` (`.mjs` test 14 pattern).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | canary token, USER 401 body with canary | error free of canary |
| 2 | canary token, transport error `errors.New("boom canary-secret-token-123")` | `network`, no canary |
| 3 | success path with login `synthetic-user` | Snapshot.Account == `synthetic-user`; no window, message, or Failure carries it elsewhere |
| 4 | usage 200 body with `canary-body-marker-42` | windows/errors contain no marker |
| 5 | device flow `access_denied` with canary device code | `access_denied` message has no canary |
| 6 | full Fetch with denied-style tokens | no `denied-global`/`denied-system`/`denied-gh`/`canary-secret` in any error or Snapshot field |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`

## Task F17-T8: Golden fixtures and Node-script output parity

**Depends on:** F17-T3, F17-T5
**Files:**
- create `internal/usage/provider/copilot/testdata/usage/copilot/copilot_chat.json`
- create `internal/usage/provider/copilot/testdata/usage/copilot/copilot_minimal.json`
- create `internal/usage/provider/copilot/testdata/usage/copilot/copilot_unlimited.json`
- create `internal/usage/provider/copilot/testdata/usage/copilot/copilot_all.json`
- create `internal/usage/provider/copilot/testdata/usage/copilot/copilot_unsupported.json`
- create `internal/usage/provider/copilot/copilot_fixture_test.go`

**Spec references:** `specs/features/F17-provider-copilot/SPEC.md §2.8, D7-D8`, `docs/plan/annex-a-provider-matrix.md` §8 (golden-file policy)

**Instructions:**
1. Create the fixtures with EXACTLY this content:
   - `copilot_chat.json`: `{"quota_snapshots": {"chat": {"entitlement": 300, "remaining": 225, "percent_remaining": 75}}, "quota_reset_date": "2030-01-01"}` — copy from `usage-allowance-checks/tests/usage-allowance.test.mjs` case 15.
   - `copilot_minimal.json`: `{"quota_snapshots": {"chat": {"remaining": 10, "entitlement": 20}}}` — copy from `usage-allowance.test.mjs` case 9.
   - `copilot_unlimited.json`: `{"quota_snapshots": {"chat": {"unlimited": true}}}` — constructed from `copilot.mjs` field names.
   - `copilot_all.json`: `{"quota_snapshots": {"chat": {"remaining": 10, "entitlement": 20}, "completions": {"remaining": 5}, "premium_interactions": {"remaining": 1, "entitlement": 2}}}` — constructed (order check).
   - `copilot_unsupported.json`: `{"garbage": 1}` — constructed (fail-closed shape).
2. Table-driven test: read + `NormalizeUsage` per fixture; expected windows:
   - `copilot_chat` → `[{chat, chat, requests, remaining 225, limit 300, used% 25, resets 2030-01-01T00:00:00Z, known}]`
   - `copilot_minimal` → `[{chat, remaining 10, limit 20, no percent, no reset}]`
   - `copilot_unlimited` → `[{chat, unlimited true, nothing else}]`
   - `copilot_all` → `[chat, completions, premium]` in that order; premium Label `premium interactions`
   - `copilot_unsupported` → `unsupported_response`
3. Parity comment in the test: for `copilot_chat.json`, `normalizeCopilotUsage` in `copilot.mjs` yields `{label:"chat", unlimited:false, remaining:225, entitlement:300, remainingPercent:75, resetAt:"2030-01-01T00:00:00.000Z"}`; the Go window carries the same data under the canonical fields (remaining 225, limit 300, UsedPercent 25 per SPEC D7, reset `2030-01-01T00:00:00Z`), and the F24 renderer derives the `75% available` line from `UsedPercent`.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `copilot_chat.json` | windows as in step 2 |
| 2 | `copilot_minimal.json` | `chat` remaining 10, limit 20 |
| 3 | `copilot_unlimited.json` | `chat` unlimited |
| 4 | `copilot_all.json` | 3 windows in `.mjs` order |
| 5 | `copilot_unsupported.json` | `unsupported_response` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/copilot/...` passes with the test cases above
- [ ] output matches `usage-allowance-checks` Node script for the same recorded fixture: `copilot_chat.json` (`.mjs` case 15) and `copilot_minimal.json` (`.mjs` case 9) normalized values identical to `normalizeCopilotUsage` (remaining/entitlement/percent data), and the F17-T6 request order `[USER, USAGE]` matches `.mjs` test 15
- [ ] fixture field names verbatim from the `.mjs` — no invented field names
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/copilot/...`
