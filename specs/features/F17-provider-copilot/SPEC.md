---
kind: feature-spec
feature: F17-provider-copilot
version: "1.0"
project: which-model
---

# F17 — provider-copilot: GitHub Copilot subscription-usage adapter

## 1. Purpose

`internal/usage/provider/copilot` is the Tier-1 adapter that reports GitHub Copilot usage allowance (premium-interactions, chat, and completions quota windows) from the private endpoint `GET https://api.github.com/copilot_internal/user`, behind a mandatory identity gate. It is the Go port of `usage-allowance-checks/lib/copilot.mjs` (285 lines), normalized into the canonical `usage.Window` contract, keeping its values diffable against the Node prototype's recorded fixtures. It self-registers a `usage.Descriptor` with ID `copilot` and exports the device-flow state machine F12's declarative OAuth source must delegate to.

## 2. Behaviour

1. **Token discovery is the declarative Auth chain (authoritative, F12 resolves it).** Descriptor `Auth`, in order: `AuthEnvVar` `COPILOT_API_TOKEN` (operator override), `Shell` `git config --global --get github.copilot.oauthToken`, `Shell` `git config --system --get github.copilot.oauthToken`, `Shell` `gh auth token --hostname github.com`, then `AuthOAuthDeviceFlow`. Every entry carries `Validate: copilot.ValidateIdentity` — the mandatory identity gate runs on EVERY candidate, and F12's rule is: a candidate failing `Validate` is skipped and the chain continues (annex-a §3.3 "identity gate (mandatory, before any usage call)"; F12 CONTRACTS, per F12 author confirmation). The `.mjs` order `git --global`, `git --system`, `gh` and its skip semantics (`discoverCopilotToken`, `copilot.mjs:44-75`) are thereby expressed declaratively; the sources are exactly those three commands — never `git config --local` (`.mjs` test 8 asserts `--local` absent). The device-flow entry exists solely for the `--login` path (F25); the chain itself never runs it.

2. **CLI discovery semantics are contracted to F12.** F12's CLI resolver (the port of `defaultCommandRunner`, `copilot.mjs:26-34`) MUST: run via `exec.CommandContext` with a 3-second timeout and a 32 KiB output cap, swallow every failure to "no candidate", and strip exactly one trailing `\r\n`/`\n` from stdout before opaque-shape validation (port of `tokenCandidate`, `copilot.mjs:36-42`; multi-line output fails the shape check and skips the candidate — `.mjs` test 8). Env values must pass the same shape check or yield no candidate.

3. **Fetch receives the chain-resolved `usage.Credential`.** F14 MUST invoke `Fetch` even when the chain resolved nothing. With `cred.Token == ""`, `Fetch` fails with `Failure{Code: "login_required", Message: "No usable GitHub token was found; rerun with --login to start device login."}` — verbatim from `copilot.mjs:249-250` (the `--login` flow itself is F25's interactive path; the `.mjs`'s `writeLogin` line `Open <verification_uri> and enter code <user_code>.` is emitted by F25 from the returned `DeviceFlow`).

4. **Identity is re-verified inside Fetch.** `Credential` carries no identity name, and `Snapshot.Account` must carry the GitHub login (annex-d `--show-identity`), so `Fetch` calls `ValidateIdentity` on the chain-resolved token itself. A failure returns its code/message as `Snapshot.Failure` (`unauthorized`, `unsupported_response`, `network`, …) — never a fallback to other sources (the chain already did that; a token present-but-invalid at fetch time is a hard failure, matching `.mjs` test 16's "identity failure stops before the Copilot private endpoint"). This costs one extra `/user` call on the chain path; on the device-flow path (`--login`) the call count is identical to the `.mjs` (recorded in Decisions).

5. **Identity gate** (`verifyGithubIdentity`, `copilot.mjs:99-110`): `GET https://api.github.com/user`, allow-list `[GITHUB_USER_URL]`, exactly three headers `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`, `User-Agent: which-model/0.4.0` (annex-a §3.3; the `.mjs`'s `CENtreeUsageAllowance/1.0` brand is superseded — recorded in Decisions). Non-200 → `statusError("GitHub identity", status)`; `login` must be a string matching `/^[A-Za-z0-9-]{1,39}$/` else `Failure{Code: "unsupported_response", Message: "GitHub returned an unsupported identity response."}` (the `.mjs` code `identity_response` maps to the canonical `unsupported_response` — recorded in Decisions). Returns the login.

6. **Usage request** (`fetchCopilotUsage`/`copilotUsageHeaders`, `copilot.mjs:93-97,111-120`): `GET https://api.github.com/copilot_internal/user`, allow-list `[COPILOT_USAGE_URL]`, exactly six headers: `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`, `Editor-Version: vscode/1.96.2`, `Editor-Plugin-Version: copilot-chat/0.26.7`, `User-Agent: GitHubCopilotChat/0.26.7`, `X-GitHub-Api-Version: 2025-04-01` (verbatim `.mjs`, including the `Bearer` scheme — the `.mjs` is the parity oracle and its tests assert these header sets by sorted key; a `token`-scheme reading is superseded). Non-200 → `statusError("GitHub Copilot", status)`. The identity gate MUST have passed for this token in the same run before this call is made.

7. **Request discipline** mirrors F15/F16 (identical helper contract): exact-URL allow-list (`endpoint_refused`), redirect hard-fail (`redirect_refused`), bounded body via F05 (`response_too_large`), empty/non-object JSON (`response_json`), deadline (`timeout`), transport (`network`).

8. **Normalization** (`normalizeCopilotUsage`, `copilot.mjs:197-223`): `quota_snapshots` must be a non-array object else `unsupported_response` with verbatim message `GitHub Copilot returned an unsupported usage shape.`; windows in the fixed order `chat`, `completions`, `premium_interactions`; per window: skip unless `unlimited == true` OR `finiteNonNegative(remaining)` OR `finitePercent(percent_remaining)` is present; `Unlimited = (unlimited === true)`; `Remaining = remaining`; `Limit = entitlement`; `UsedPercent = 100 - percent_remaining` when present (annex-a §3.3 canonical mapping — the canonical `Window` has no remaining-percent field; recorded in Decisions); `ResetsAt = resetTime(source.reset_at ?? quota_reset_date)` (ISO string or epoch number; date-only strings parse at UTC midnight); `UsageKnown: true`; `Unit: requests`. Window IDs/labels: `chat`/`chat`, `completions`/`completions`, `premium`/`premium interactions` (label = key with `_`→space). Zero windows → `unsupported_response`.

9. **Device flow is exported for F12/F25** (ports of `startDeviceFlow`/`pollDeviceFlow`, `copilot.mjs:122-195`):
   - `StartDeviceFlow`: `POST https://github.com/login/device/code`, headers `Accept: application/json`, `Content-Type: application/x-www-form-urlencoded`, body `client_id=Iv1.b507a08c87ecfe98&scope=read:user`; non-200 → `statusError("GitHub device login", status)`. Validations (each failure → `Failure{Code: "unsupported_response", Message: "GitHub returned an unsupported device-login response."}` — the `.mjs` code `device_response` maps to the canonical `unsupported_response`): `device_code` must pass the opaque-token shape; `user_code` must match `/^[A-Z0-9-]{4,32}$/`; `verification_uri` must parse and its href equal EXACTLY `https://github.com/login/device`; `expires_in` finite 1..1800; `interval` (default 5) finite 1..30.
   - `PollDeviceFlow`: local deadline `now + expiresIn*1000`; per iteration: if the deadline is already reached or past, stop without requesting (the `.mjs`'s "never poll at/after the deadline"); sleep `min(interval*1000, remaining)`; `POST https://github.com/login/oauth/access_token` (same headers; body adds `device_code`, `grant_type=urn:ietf:params:oauth:grant-type:device_code`); non-200 → `statusError("GitHub device login", status)`; `access_token` present → opaque-shape check → return it; `error` switch: `authorization_pending` → continue silently; `slow_down` → `interval += 5` (unbounded, `.mjs` verbatim); `access_denied` → `Failure{Code: "access_denied", Message: "GitHub device login was denied or cancelled."}`; `expired_token` → `Failure{Code: "device_expired", Message: "GitHub device login expired."}`; anything else → `unsupported_response` device-login message. Loop exit (deadline) → `device_expired`.
   - The clock/sleep injection points exist for tests exactly as the `.mjs` (`now`/`sleep`); the `.mjs` wait sequences `[1000, 1000, 6000]` and `[1000, 6000, 11000]` are pinned by tests.
   - F12's `AuthOAuthDeviceFlow` resolver MUST delegate to these two functions (no duplicated state machine); the OAuth constants live here.

10. **Snapshot assembly.** `Snapshot{Provider:"copilot", Windows, Account: <login>, FetchedAt: now UTC, Source: SourceOAuth, Confidence:"live"}`. `Account` is always the verified login (output gated by F24's `--show-identity`); `Plan` empty.

11. **Descriptor constants.** `Timeout: 15s`, `CacheTTL: 60s` (annex-a §5/§6), `Kind: KindSubscription`, `Tier: 1`, `DisplayName: "GitHub Copilot"` (the `.mjs` report header, `copilot.mjs:279`). `Windows` descriptor list (all Optional): `premium` (requests), `chat` (requests), `completions` (requests). `init()` registers; duplicate ID panics.

## 3. Error behaviour

All failures return `(zero Snapshot, *Error)` with `Error{Code, Message}` carrying a global `Failure.Code` (§1.6) and a sanitized fixed message. Codes this feature emits: `login_required`, `unauthorized`, `rate_limited`, `provider_status`, `unsupported_response`, `access_denied`, `device_expired`, `endpoint_refused`, `redirect_refused`, `response_too_large`, `response_json`, `timeout`, `network`. The `.mjs`-only codes `identity_response`, `device_response`, `identity_validation` have no canonical equivalent and are folded into `unsupported_response`/`login_required` (Decisions). No message or log line ever contains tokens, device codes, or logins (global SPEC §6 item 5; canary-tested — the `.mjs` asserts the output never matches `denied-global|denied-system|denied-gh|canary-secret`).

## 4. Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Token discovery is F12's declarative chain; this feature contributes the chain literal + `ValidateIdentity` | env → git global → git system → gh → device flow | F12's resolver rules (skip-on-validate-failure, CLI-degrade) express `.mjs` discovery verbatim; the `.mjs` discovery port has no runtime home here |
| D2 | CLI semantics contracted to F12: 3s timeout, 32 KiB cap, swallow-to-no-candidate, strip one trailing newline | — | `defaultCommandRunner`/`tokenCandidate` (`copilot.mjs:26-42`); `.mjs` test 8 pins order and the multi-line skip |
| D3 | Fetch re-validates the chain-resolved token (identity gate inside Fetch) | +1 `/user` call on the chain path | `Snapshot.Account` (the verified login) cannot travel in `Credential`; the invariant "private endpoint never called without a same-run identity check" is preserved; device-flow path call counts match `.mjs` test 14 exactly |
| D4 | Identity `User-Agent: which-model/0.4.0` (annex-a §3.3), superseding the `.mjs`'s `CENtreeUsageAllowance/1.0` | header SET stays exactly 3 | annex-a is normative for the final product; F22 wires the real version later |
| D5 | Usage `Authorization: Bearer <token>` (verbatim `.mjs`), superseding any `token`-scheme reading | 6 headers, sorted-key-tested | the recorded fixtures use Bearer; `.mjs` test 15 asserts the exact header sets |
| D6 | `identity_response` → `unsupported_response`; `device_response` → `unsupported_response`; `identity_validation` (Go: unconditional gate) → n/a | messages verbatim from `.mjs` | the global §1.6 table has no such codes; `unsupported_response` = "JSON parsed but wrong shape" is their exact semantic |
| D7 | `percent_remaining` maps to `UsedPercent = 100 - percent_remaining` (annex-a §3.3) | canonical `Window` has no remaining-percent field | data parity holds (the value 75 is represented as used 25; F24 derives `75% available` from `UsedPercent` like every other provider) |
| D8 | Resets: `source.reset_at ?? quota_reset_date`; date-only strings parse at UTC midnight | — | `.mjs` `resetText` + test 15 fixture `quota_reset_date: "2030-01-01"` |
| D9 | `Access_denied`/`expired_token`/deadline are terminal failures (`access_denied`/`device_expired`) | no retry loops | `.mjs` verbatim; tests 11-13 |
| D10 | `slow_down` increments the interval by 5s, unbounded, until the deadline | — | `.mjs` verbatim; test 13 pins `[1000, 6000, 11000]` |
| D11 | `AuthOAuthDeviceFlow` (F12) MUST delegate to this package's `StartDeviceFlow`/`PollDeviceFlow` | constants live here | single state machine; the device flow is GitHub-specific beyond F12's generic OAuth spec |
| D12 | Empty cred → `login_required` with verbatim `.mjs` message | `--login` is F25's domain | global §1.6: `login_required` = "no credential found, no `--login`" — the exact `.mjs` semantics |
| D13 | Window IDs `premium`/`chat`/`completions`; labels verbatim (`premium interactions`, `chat`, `completions`) | all Optional | annex-a §3.3 IDs; `.mjs` label = key with `_` → space |
| D14 | `DisplayName: "GitHub Copilot"`, `Timeout: 15s`, `CacheTTL: 60s` | — | `.mjs` report header; annex-a §5/§6 |

## 5. Out of scope

- The interactive `--login` command flow (`writeLogin`, message emission, F25's `auth login copilot`) — F25 consumes the exported `DeviceFlow`/`StartDeviceFlow`/`PollDeviceFlow`.
- The web/budget fetcher (`https://github.com/settings/copilot` budget pages) — the prototype uses the private API only.
- Token discovery runtime (`exec.CommandContext` wiring, env reading, chain walking) — F12's resolver; this feature's CONTRACTS §9 pins the semantics.
- Refresh of expired Copilot tokens — the prototype never refreshes; failure is terminal.
- Build-tag `nousage` stubbing — F21's domain (global SPEC §4); DEPENDENCY-GRAPH §2 lists no `blocks` for F17.
- Text rendering (`formatUsageReport` port, `--show-identity` gating) — F24's domain; this feature guarantees data-level parity.
