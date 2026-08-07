---
kind: feature-tasks
feature: F15-provider-claude
version: "1.0"
task_count: 7
---

# F15 — provider-claude: Tasks

## Task graph

```mermaid
graph LR
    T1[T1 skeleton + Descriptor] --> T2[T2 credential loader]
    T1 --> T3[T3 normalization]
    T2 --> T4[T4 fetch port]
    T3 --> T4
    T4 --> T5[T5 error mapping]
    T4 --> T6[T6 canary]
    T3 --> T7[T7 golden fixtures + parity]
    T4 --> T7
```

## Task F15-T1: Create the package skeleton and register the Claude descriptor

**Depends on:** none
**Files:**
- create `internal/usage/provider/claude/claude.go`
- create `internal/usage/provider/claude/claude_test.go`

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §2.1, §2.11`, `specs/features/F15-provider-claude/CONTRACTS.md §2`, `specs/global/CONTRACTS.md §1.3`, `docs/plan/annex-a-provider-matrix.md §3.2, §5`

**Instructions:**
1. Write the test file first. It imports the package under test as `claude` and asserts `usage.Lookup("claude")` (registry from F11, `internal/usage/registry.go`) succeeds after import (the package's `init()` registers).
2. Implement `claude.go`: package doc comment stating it is the port of `usage-allowance-checks/lib/claude.mjs`; the constant `UsageURL = "https://api.anthropic.com/api/oauth/usage"` (verbatim from `claude.mjs:15`); `const UserAgent = "claude-code/2.1.0"`; an `init()` that calls `usage.Register` with the Descriptor literal copied exactly from `specs/features/F15-provider-claude/CONTRACTS.md §2` — do not change any field value.
3. Use the canonical types from F11: `usage.Descriptor`, `usage.AuthSource`, `usage.KeychainSpec`, `usage.WindowSpec`, `usage.KindSubscription`, `usage.UnitPercent`, `usage.UnitUSD`. Do not define or extend them.
4. The `Fetch` field references `Fetch` from task F15-T4; until then declare `var Fetch usage.FetchFunc = func(context.Context, usage.Credential, *http.Client) (usage.Snapshot, error) { return usage.Snapshot{}, nil }` so the package compiles. Replace it in F15-T4.
5. Do not add any other source file.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `usage.Lookup("claude")` | ok, descriptor found |
| 2 | descriptor `ID` | `"claude"` |
| 3 | descriptor `DisplayName` | `"Claude"` |
| 4 | descriptor `Kind`, `Tier` | `usage.KindSubscription`, `1` |
| 5 | descriptor `Timeout`, `CacheTTL` | `15 * time.Second`, `60 * time.Second` |
| 6 | descriptor `Windows` IDs in order | `["5h","weekly","sonnet_7d","opus_7d","oauth_apps_7d","routines_7d","extra_usage"]` |
| 7 | window `5h` spec | `Optional == false`, `Unit == usage.UnitPercent` |
| 8 | windows `weekly`..`extra_usage` | all `Optional == true`; `extra_usage` `Unit == usage.UnitUSD` |
| 9 | `Auth` source kinds in order | `[AuthEnvVar, AuthKeychainGeneric, AuthFile, AuthFile, AuthFile, AuthFile, AuthFile, AuthFile]` |
| 10 | first `AuthFile` entry | `FilePaths == ["~/.claude/.credentials.json", "~/.claude/credentials.json"]`, `JSONPath == "claudeAiOauth.accessToken"` |
| 11 | env entry | `EnvVar == "WHICH_MODEL_CLAUDE_OAUTH_TOKEN"`; keychain entry `Service == "Claude Code-credentials"` |
| 12 | `UsageURL` constant | `"https://api.anthropic.com/api/oauth/usage"` |

**Acceptance criteria:**
- [ ] `go build ./internal/usage/provider/claude/...` succeeds
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] no file outside the Files list modified
- [ ] descriptor literal matches CONTRACTS §2 field-for-field (no invented windows or auth sources)

`go test ./internal/usage/provider/claude/...`

## Task F15-T2: Port the Claude credential-file loader

**Depends on:** F15-T1
**Files:**
- create `internal/usage/provider/claude/claude_credential.go`
- create `internal/usage/provider/claude/claude_credential_test.go`

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §2.3`, `specs/features/F15-provider-claude/CONTRACTS.md §1, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §2.1 (`loadClaudeCredential`, claude.mjs:17-31)

**Instructions:**
1. Write the test file first. Tests use `t.TempDir()` for real files (never the real home dir) and `security.ValidateOpaqueToken` semantics. Use the canary literal `"canary-secret-token-123"` for tokens in fixtures.
2. Implement `LoadFileCredential(dotPath, plainPath string, now time.Time) (FileCredential, error)` as the port of `loadClaudeCredential`'s file leg (`claude.mjs:17-31`):
   - Probe `dotPath` first, then `plainPath`; first file that yields a token wins.
   - Read bounded: `security.ReadBoundedFile(path, security.MaxCredentialBytes)`; ENOENT/not-found → try the next path, and if none found return `(FileCredential{}, nil)` (caller falls back to the chain credential); other read errors → `Error{Code: "credential_file", Message: "Claude credentials were not found; sign in with Claude Code first."}`.
   - Decode JSON object; non-object/unparseable → `Error{Code: "credential_json", Message: "The credential file is not valid JSON."}` (mirrors `readCredentialJson`, `core.mjs:62-70`).
   - `oauth := value.claudeAiOauth ?? value.oauth ?? value`; token `oauth.accessToken ?? oauth.access_token`; missing or failing `security.ValidateOpaqueToken` → `Error{Code: "unsafe_credential", Message: "The Claude access token is missing or unsafe."}`.
   - Expiry: `expiresAt := oauth.expiresAt ?? oauth.expires_at`; if present: number, or `time.Parse` on the string; number `> 10_000_000_000` is milliseconds, else seconds; unparseable or `<= now` → `Error{Code: "expired_credential", Message: "The Claude access token is expired."}` (the `.mjs` treats both unparseable and past as expired — `claude.mjs:24-28`; the `.mjs`'s `Date.parse` failure is the Go unparseable case).
   - `BroadPermissions = security.HasBroadPermissions(mode)` from the winning file's mode.
3. Messages are the fixed strings from CONTRACTS §6; never interpolate file contents.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | dot file `{"claudeAiOauth":{"accessToken":"canary-secret-token-123","expiresAt":<now+60s ms>}}` | token = canary, no error |
| 2 | same but `"access_token"` + `"expires_at"` snake keys | token = canary, no error |
| 3 | plain file only, flat `{"accessToken":"canary-secret-token-123"}` | token = canary, no error |
| 4 | `{"oauth":{"accessToken":"canary-secret-token-123"}}` | token = canary |
| 5 | neither file exists | `(FileCredential{}, nil)` |
| 6 | file `{bad` | `credential_json` |
| 7 | file `{}` | `unsafe_credential` |
| 8 | `expiresAt: 1` with `now: 10_000` | `expired_credential` |
| 9 | `expiresAt: now-1000` (ms) | `expired_credential` |
| 10 | file mode `0o644` | `BroadPermissions == true`; mode `0o600` → `false` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] every error message for cases 6-9 is the fixed string from CONTRACTS §6 and contains no canary
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/claude/...`

## Task F15-T3: Port the Claude usage normalizer (including the synthetic 5h rule)

**Depends on:** F15-T1
**Files:**
- create `internal/usage/provider/claude/claude_normalize.go`
- create `internal/usage/provider/claude/claude_normalize_test.go`

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §2.7, §2.8, §2.9, D5-D9`, `specs/features/F15-provider-claude/CONTRACTS.md §4, §5`, `docs/plan/research/usage-allowance-checks-spec.md` §2.1 (`normalizeClaudeUsage`, claude.mjs:33-56)

**Instructions:**
1. Write the test file first, table-driven with `json.RawMessage` inputs inline in the table.
2. Implement `NormalizeUsage(raw []byte) ([]usage.Window, error)`:
   - Decode `raw` as a JSON object; decode failure → `Error{Code: "response_json", Message: "The provider returned unsupported JSON."}` (non-object top level included).
   - Probe the fixed keys in order: `five_hour`, `seven_day`, `seven_day_sonnet`, `seven_day_opus`, `seven_day_oauth_apps`; then the routines try-keys in order `seven_day_routines`, `seven_day_claude_routines`, `claude_routines`, `routines`, `routine`, `seven_day_cowork`, `cowork` — the first key present with a non-null object wins for the routines window.
   - Per window: skip when the value is absent or not an object; `usedPercent = finitePercent(utilization ?? used_percent)` (number or numeric string, finite, 0..100) — skip the window when undefined. `resetsAt = resetTime(resets_at ?? reset_at)` (ISO string or epoch number; number `> 10_000_000_000` is ms else seconds; unparseable → nil).
   - Window construction per CONTRACTS §5 (ID/label/window minutes/model scope). `WindowMinutes` is a pointer to the constant: 300 for `5h`, 10080 for the six weekly windows. `UsageKnown: true` for real windows. `Synthetic: false`.
   - Synthetic rule (SPEC D5): `five_hour` present-but-null or present-but-not-an-object → append `Window{ID:"5h", Label:"five hour", Unit:usage.UnitPercent, Synthetic:true, UsageKnown:false}` with no `UsedPercent`/`Used`/`Limit`/`Remaining`. Absent `five_hour` → nothing synthetic.
   - `extraUsage` (SPEC §2.9): object with at least one valid field among `usedCredits` (finite ≥ 0), `monthlyLimit` (finite ≥ 0), `utilization` (0..100) → window ID `extra_usage`, Label `Extra usage`, `Unit: usage.UnitUSD`, `Used`/`Limit`/`UsedPercent` set only for the valid fields, `UsageKnown: true`. `isEnabled`/`currency` ignored.
   - `limits` (SPEC §2.9): array of objects; per entry with valid `percent` (0..100): ID `"limit:" + slug(kind + "_" + group)`, Label `group` if non-empty else `kind`, `UsedPercent = percent`, `ResetsAt = resetTime(resetsAt)`, `ModelScope: []string{scope.model.id}` when present, `UsageKnown: true`. `slug` = strings.ToLower with every run of non-alphanumeric bytes collapsed to `-` and trimmed at both ends. `isActive` ignored.
   - Zero windows and no synthetic → `Error{Code: "unsupported_response", Message: "Claude returned an unsupported usage shape."}`.
   - `finitePercent`/`finiteNonNegative`/`resetTime` are private package helpers (ports of `core.mjs:202-224`); add them in this file.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `{"five_hour":{"utilization":25,"resets_at":"2030-01-01T00:00:00Z"}}` (copy from `usage-allowance-checks/tests/usage-allowance.test.mjs` case 1) | 1 window: ID `5h`, Label `five hour`, Unit percent, UsedPercent 25, WindowMinutes 300, ResetsAt `2030-01-01T00:00:00Z`, UsageKnown true |
| 2 | `{"five_hour":{"used_percent":25},"seven_day":{"utilization":41}}` | 2 windows: `5h` (used 25), `weekly` (used 41, WindowMinutes 10080) |
| 3 | `{"five_hour":{"utilization":150}}` | utilization out of range → no windows → `unsupported_response` |
| 4 | `{"five_hour":null,"seven_day":{"used_percent":41,"resets_at":"2030-01-01T00:00:00Z"}}` | 2 windows: `5h` Synthetic true, UsageKnown false, no percent fields; `weekly` used 41 |
| 5 | `{"five_hour":null}` | 1 window: synthetic `5h` only, no error |
| 6 | `{"garbage":1}` | `unsupported_response` (no synthetic — `five_hour` absent) |
| 7 | `{"five_hour":{"utilization":25},"extraUsage":{"isEnabled":true,"monthlyLimit":40,"usedCredits":7.5,"utilization":18.75,"currency":"USD"}}` | `5h` + `extra_usage`: Unit usd, Used 7.5, Limit 40, UsedPercent 18.75 |
| 8 | `{"five_hour":{"utilization":25},"limits":[{"kind":"weekly","group":"sonnet","percent":41,"resetsAt":"2026-08-07T18:00:00Z","scope":{"model":{"id":"claude-sonnet-4-5","display_name":"Claude Sonnet 4.5"}},"isActive":true}]}` | `5h` + `limit:weekly_sonnet`: UsedPercent 41, ModelScope `["claude-sonnet-4-5"]`, ResetsAt `2026-08-07T18:00:00Z` |
| 9 | `{"five_hour":{"utilization":25,"resets_at":"not-a-date"}}` | `5h` used 25, ResetsAt nil |
| 10 | `{"five_hour":{"utilization":"25"}}` (string) | `5h` used 25 |
| 11 | `not json` | `response_json` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] normalized windows for case 1 equal the values `normalizeClaudeUsage` from `usage-allowance-checks/lib/claude.mjs` produces for the same fixture (used 25, label `five hour`, reset `2030-01-01T00:00:00Z`)
- [ ] null `five_hour` never yields a window with UsedPercent 0 (SPEC D5)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/claude/...`

## Task F15-T4: Port the Claude fetch (request helper + checkClaudeUsage)

**Depends on:** F15-T2, F15-T3
**Files:**
- create `internal/usage/provider/claude/claude_fetch.go`
- create `internal/usage/provider/claude/claude_fetch_test.go`
- edit `internal/usage/provider/claude/claude.go` (replace the placeholder `Fetch` with the real one)

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §2.2-§2.6, §2.10, D2-D4`, `specs/features/F15-provider-claude/CONTRACTS.md §3, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §2.1 (`checkClaudeUsage`, claude.mjs:58-91), §1 (`requestJson`, `statusError`)

**Instructions:**
1. Write the test file first. Fake the network with a stub `http.RoundTripper` struct whose `RoundTrip` returns canned `*http.Response` bodies (record the request for assertions). Capture stderr for the warning test via `log.SetOutput` (restore after).
2. Implement the private helper `requestJSON(ctx context.Context, client *http.Client, url string, allowed []string, headers map[string]string) (int, json.RawMessage, error)`:
   - `security.ValidateExactHTTPS(url, allowed)`; failure → `Error{Code: "endpoint_refused", Message: "The provider endpoint was refused."}`.
   - Build the request with the header map; copy the client (`c2 := *client; c2.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }`) and issue via `c2.Do`.
   - Response status in [300, 400) → `redirect_refused` (`The provider attempted an unsafe redirect.`).
   - `security.ReadResponseBounded(resp, security.MaxResponseBytes)`; over-budget → `response_too_large` (`The provider response exceeded the safe size limit.`).
   - `ctx.Err() == context.DeadlineExceeded` → `timeout` (`The provider request timed out.`); other transport errors → `network` (`The provider request failed.`).
   - Non-2xx → return `(status, nil, nil)` (caller maps); 2xx: empty body or non-object JSON → `response_json` (`The provider returned unsupported JSON.`).
3. Implement `Fetch(ctx, cred, client)` (replacing the placeholder in `claude.go`):
   - Empty `cred.Token` → `Snapshot{Provider:"claude", Failure:&usage.Failure{Code:"login_required", Message:"Claude credentials were not found; sign in with Claude Code first."}, FetchedAt: time.Now().UTC(), Source: usage.SourceOAuth, Confidence:"live"}`.
   - `cred.Source == usage.AuthFile`: call `LoadFileCredential(dotPath, plainPath, time.Now())` with paths `filepath.Join(home, ".claude/.credentials.json")` then `filepath.Join(home, ".claude/credentials.json")` (`os.UserHomeDir()`); on hard error return it as `Snapshot.Failure`; on success with a token: expired → Failure `expired_credential`; else use that token and, when `BroadPermissions`, write the verbatim warning line to stderr via `log` and return the Snapshot with `Failure` unset. When the load returns no token, proceed with `cred.Token`.
   - Request `GET UsageURL`, allow-list `[]string{UsageURL}`, headers exactly per CONTRACTS §3 with `Authorization: Bearer <token>`.
   - Status 200 → `NormalizeUsage(body)`; failure → Failure with that error's code/message. Non-200 → `mapStatus("Claude", status)` per the table in CONTRACTS §6 (401/403 → `unauthorized` `Claude rejected the credential.`; 429 → `rate_limited` `Claude rate-limited the usage request.`; other → `provider_status` `Claude usage is unavailable (HTTP <status>).`).
   - Success → `Snapshot{Provider:"claude", Windows: windows, FetchedAt: now, Source: usage.SourceOAuth, Confidence:"live"}`.
4. Every Failure message is the fixed CONTRACTS §6 string; never include response bodies or tokens.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | cred token canary, stub 200 with `{"five_hour":{"utilization":25,"resets_at":"2030-01-01T00:00:00Z"}}` (fixture: copy from `usage-allowance.test.mjs` case 1) | Snapshot windows: 1 window `5h` used 25; request URL == `UsageURL`; headers contain `Accept: application/json`, `Authorization: Bearer canary-secret-token-123`, `anthropic-beta: oauth-2025-04-20`, `Content-Type: application/json`, `User-Agent: claude-code/2.1.0`; exactly 5 headers |
| 2 | same, stub 401 with body containing canary | Failure code `unauthorized`, message contains no canary |
| 3 | same, stub 403 | `unauthorized` |
| 4 | same, stub 429 | `rate_limited` |
| 5 | same, stub 500 | `provider_status` |
| 6 | same, stub 302 | `redirect_refused` |
| 7 | same, stub 200 with `Content-Length: 300` and a 300-byte body | `response_too_large` (bounded via `security.ReadResponseBounded`/`MaxResponseBytes`) |
| 8 | same, stub `{bad` | `response_json` |
| 9 | stub transport returns error containing canary | `network`, message has no canary |
| 10 | empty `cred.Token` | Failure `credential_file` with verbatim message `Claude credentials were not found; sign in with Claude Code first.`; no HTTP request issued |
| 11 | `cred.Source == usage.AuthFile`, temp dot file mode `0o644` with valid token + future expiry | warning line `Warning: Claude credential permissions are broader than 0600; review them before continuing.` written to captured stderr, fetch proceeds |
| 12 | `cred.Source == usage.AuthFile`, temp file with `expiresAt: <now-1000>` | Failure `expired_credential` before any HTTP request |

**Acceptance criteria:**
- [ ] `go build ./internal/usage/provider/claude/...` succeeds
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] request URL/headers match CONTRACTS §3 exactly (no extra headers, no redirects followed)
- [ ] no file outside the Files list modified (the `claude.go` edit is the declared placeholder replacement)

`go test ./internal/usage/provider/claude/...`

## Task F15-T5: Add the status-to-Failure mapping table tests

**Depends on:** F15-T4
**Files:**
- create `internal/usage/provider/claude/claude_status_test.go`

**Spec references:** `specs/features/F15-provider-claude/CONTRACTS.md §6`, `docs/plan/research/usage-allowance-checks-spec.md` §1 (`statusError`, core.mjs:192-200), `docs/plan/annex-a-provider-matrix.md` §7

**Instructions:**
1. Write a table-driven test over the exported mapping surface: every HTTP status in 100..599 exercises `mapStatus` (the private function from F15-T4 — test it via the package's test file, which is in the same package) and asserts the `(code, message)` pair.
2. The table MUST cover: 401, 403 → `unauthorized`; 429 → `rate_limited`; 200 → no error; 3xx statuses never reach `mapStatus` (they are `redirect_refused` at the request layer — assert via a stub transport returning each of 301/302/307/308 and expecting `redirect_refused`); a sample of other statuses (400, 404, 500, 502, 503) → `provider_status` with the exact message `Claude usage is unavailable (HTTP <status>).`.
3. Add one sanitization case: a 401 response whose body is the canary literal; assert the returned message never contains the canary.
4. Assert every code in the result is one of the global `Failure.Code` values listed in `specs/global/CONTRACTS.md §1.6` (hard-code the expected set in the test).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | status 401 | `unauthorized`, `Claude rejected the credential.` |
| 2 | status 403 | `unauthorized` |
| 3 | status 429 | `rate_limited`, `Claude rate-limited the usage request.` |
| 4 | status 400, 404, 500, 502, 503 | `provider_status`, `Claude usage is unavailable (HTTP <status>).` |
| 5 | status 301, 302, 307, 308 via stub transport | `redirect_refused` at request layer |
| 6 | status 401, body = canary | message free of canary |
| 7 | every emitted code | member of the global §1.6 Failure.Code set |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/claude/...`

## Task F15-T6: Canary-test every credential-touching path

**Depends on:** F15-T4
**Files:**
- create `internal/usage/provider/claude/claude_canary_test.go`

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §3`, `specs/global/SPEC.md §6 item 5`, `docs/plan/research/usage-allowance-checks-spec.md` §9

**Instructions:**
1. Use `security.WithCanary(canary, fn)` (pinned F05 helper) around each scenario: it fails the test if the canary appears in any returned error text.
2. Canary values: token `"canary-secret-token-123"`; response-body marker `"canary-body-marker-42"`.
3. Scenarios: (a) canary token in the credential file, stub 401; (b) canary token in the credential file, stub network error; (c) canary marker in a 200 response body (assert the marker is absent from the normalized windows and from any error); (d) canary token + expired expiry; (e) canary in the redirect Location header of a 302.
4. Also assert the stderr capture contains no canary in the broad-permission warning scenario from F15-T4 case 11.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | canary token, 401 body `{"message":"canary-secret-token-123"}` | error text has no canary |
| 2 | canary token, transport error `errors.New("boom canary-secret-token-123")` | `network`, no canary |
| 3 | 200 body containing `canary-body-marker-42` in `five_hour` | normalized windows and errors contain no marker |
| 4 | canary token, expired | `expired_credential`, no canary |
| 5 | 302 with `Location: https://canary-secret-token-123.example/` | `redirect_refused`, no canary |
| 6 | broad-permission warning path with canary token | captured stderr has no canary |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/claude/...`

## Task F15-T7: Golden fixtures and Node-script output parity

**Depends on:** F15-T3, F15-T4
**Files:**
- create `internal/usage/provider/claude/testdata/usage/claude/oauth_basic.json`
- create `internal/usage/provider/claude/testdata/usage/claude/oauth_synthetic_5h.json`
- create `internal/usage/provider/claude/testdata/usage/claude/oauth_unsupported.json`
- create `internal/usage/provider/claude/testdata/usage/claude/oauth_extra_usage.json`
- create `internal/usage/provider/claude/testdata/usage/claude/oauth_limits.json`
- create `internal/usage/provider/claude/claude_fixture_test.go`

**Spec references:** `specs/features/F15-provider-claude/SPEC.md §2.7-§2.9, D5`, `docs/plan/annex-a-provider-matrix.md` §8 (golden-file policy), `docs/plan/research/usage-allowance-checks-spec.md` §8

**Instructions:**
1. Create the fixture files with EXACTLY this content (each marked with a comment in the test as to provenance):
   - `oauth_basic.json`: `{"five_hour": {"utilization": 25, "resets_at": "2030-01-01T00:00:00Z"}}` — copy from `usage-allowance-checks/tests/usage-allowance.test.mjs` case 1.
   - `oauth_synthetic_5h.json`: `{"five_hour": null, "seven_day": {"used_percent": 41, "resets_at": "2030-01-01T00:00:00Z"}}` — constructed from SPEC D5 (no `.mjs` case exists).
   - `oauth_unsupported.json`: `{"garbage": 1}` — constructed (fail-closed shape).
   - `oauth_extra_usage.json`: `{"five_hour": {"utilization": 25}, "extraUsage": {"isEnabled": true, "monthlyLimit": 40, "usedCredits": 7.5, "utilization": 18.75, "currency": "USD"}}` — constructed from annex-a §3.2/survey:136-143 field names.
   - `oauth_limits.json`: `{"five_hour": {"utilization": 25}, "limits": [{"kind": "weekly", "group": "sonnet", "percent": 41, "resetsAt": "2026-08-07T18:00:00Z", "scope": {"model": {"id": "claude-sonnet-4-5", "display_name": "Claude Sonnet 4.5"}}, "isActive": true}]}` — constructed from annex-a §3.2/survey:136-143.
2. Write a table-driven test: for each fixture, `os.ReadFile` + `NormalizeUsage`, asserting the exact expected windows (IDs, labels, units, pointer values, WindowMinutes, ResetsAt, Synthetic, UsageKnown).
3. Expected values: `oauth_basic` → `[{5h, five hour, percent, used 25, minutes 300, resets 2030-01-01T00:00:00Z, known}]`; `oauth_synthetic_5h` → `[{5h synthetic, unknown, no percents}, {weekly, seven day, used 41, minutes 10080, resets 2030-01-01T00:00:00Z, known}]`; `oauth_unsupported` → `unsupported_response`; `oauth_extra_usage` → `[{5h used 25}, {extra_usage, Extra usage, usd, used 7.5, limit 40, used% 18.75, known}]`; `oauth_limits` → `[{5h used 25}, {limit:weekly_sonnet, sonnet, used 41, scope [claude-sonnet-4-5], resets 2026-08-07T18:00:00Z, known}]`.
4. Parity note (comment in the test): for `oauth_basic.json`, `normalizeClaudeUsage` in `usage-allowance-checks/lib/claude.mjs` yields `{label:"five hour", usedPercent:25, remainingPercent:75, resetAt:"2030-01-01T00:00:00Z"}` — the Go windows carry the same values (used 25; remaining derived as 75 by the F24 renderer per global CONTRACTS §1.4), so the rendered text `- five hour: 25% used; 75% available; resets 2030-01-01T00:00:00Z` is identical to the Node script's output for the same fixture.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `oauth_basic.json` | windows as in step 3, `usage_known: true` |
| 2 | `oauth_synthetic_5h.json` | synthetic `5h` + `weekly` used 41 |
| 3 | `oauth_unsupported.json` | `unsupported_response` |
| 4 | `oauth_extra_usage.json` | `5h` + `extra_usage` (usd, 7.5 used / 40 limit / 18.75%) |
| 5 | `oauth_limits.json` | `5h` + `limit:weekly_sonnet` with scope |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/claude/...` passes with the test cases above
- [ ] output matches `usage-allowance-checks` Node script for the same recorded fixture: for `oauth_basic.json` (`.mjs` case 1), normalized values are identical to `normalizeClaudeUsage`'s (`usedPercent 25`, label `five hour`, reset `2030-01-01T00:00:00Z`), and the F24 renderer derives the identical `75% available` line from `UsedPercent` that the Node `formatUsageReport` prints
- [ ] fixture field names are verbatim from the `.mjs`/survey — no invented field names
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/claude/...`
