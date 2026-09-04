---
kind: feature-contracts
feature: F17-provider-copilot
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F17 — provider-copilot: Contracts

Package: `internal/usage/provider/copilot`
Files: `internal/usage/provider/copilot/copilot.go` (constants, Descriptor, `init`), `internal/usage/provider/copilot/copilot_identity.go`, `internal/usage/provider/copilot/copilot_normalize.go`, `internal/usage/provider/copilot/copilot_device.go`, `internal/usage/provider/copilot/copilot_fetch.go`, `internal/usage/provider/copilot/copilot_check.go`, plus `_test.go` twins and `internal/usage/provider/copilot/testdata/usage/copilot/*.json` fixtures.

Consumed canonical types (cite verbatim, never redefine): `usage.Window`, `usage.Snapshot`, `usage.Failure`, `usage.Unit`, `usage.Kind`, `usage.Source`, `usage.Credential`, `usage.Descriptor`, `usage.AuthSource`, `usage.WindowSpec`, `usage.ShellSpec`, `usage.OAuthSpec` — `specs/global/CONTRACTS.md §1` and F11's `internal/usage/types.go` (annex-a §5; `ShellSpec`/`OAuthSpec` field names per F12's CONTRACTS — the fields required here are stated inline). F05 helpers cited verbatim per the pinned F05 surface: `security.ValidateOpaqueToken`, `security.ValidateExactHTTPS`, `security.ReadResponseBounded`, `security.MaxResponseBytes`.

## 1. Exported symbols

```go
package copilot

import (
    "context"
    "net/http"
    "time"

    "github.com/WD-Mitchell/which-model/internal/usage"
)

// Endpoint and OAuth constants (verbatim copilot.mjs:9-15).
const (
    GitHubDeviceCodeURL  = "https://github.com/login/device/code"
    GitHubDeviceTokenURL = "https://github.com/login/oauth/access_token"
    GitHubUserURL        = "https://api.github.com/user"
    CopilotUsageURL      = "https://api.github.com/copilot_internal/user"
    CopilotClientID      = "Iv1.b507a08c87ecfe98"
    APIVersion           = "2025-04-01"
)

// IdentityUserAgent is the identity-gate User-Agent (annex-a §3.3; SPEC D4).
const IdentityUserAgent = "which-model/0.4.0"

// ValidateIdentity is the port of verifyGithubIdentity (copilot.mjs:99-110):
// GET GitHubUserURL with exactly the three headers {Accept:
// application/vnd.github+json, Authorization: Bearer <token>, User-Agent:
// IdentityUserAgent}; non-200 → Error per mapStatus("GitHub identity", status);
// login must match ^[A-Za-z0-9-]{1,39}$ else Error{Code: "unsupported_response",
// Message: "GitHub returned an unsupported identity response."}. Returns the
// login. This is the AuthSource.Validate hook for every chain entry of this
// provider (SPEC §2.1, D1).
func ValidateIdentity(ctx context.Context, token string, client *http.Client) (string, error)

// NormalizeUsage is the port of normalizeCopilotUsage (copilot.mjs:197-223)
// per SPEC §2.8. Window order: chat, completions, premium_interactions.
// Returns Error{Code: "unsupported_response", Message: "GitHub Copilot
// returned an unsupported usage shape."} when quota_snapshots is absent/
// non-object/array or zero windows are produced.
func NormalizeUsage(raw []byte) ([]usage.Window, error)

// DeviceFlow is the validated startDeviceFlow result (copilot.mjs:130-155).
type DeviceFlow struct {
    DeviceCode     string
    UserCode       string
    VerificationURI string // always "https://github.com/login/device"
    ExpiresIn      int     // 1..1800
    Interval       int     // 1..30
}

// StartDeviceFlow is the port of startDeviceFlow (copilot.mjs:122-155):
// POST GitHubDeviceCodeURL, headers {Accept: application/json, Content-Type:
// application/x-www-form-urlencoded}, body client_id=CopilotClientID&scope=read:user.
// Non-200 → Error per mapStatus("GitHub device login", status). Validation
// failures → Error{Code: "unsupported_response", Message: "GitHub returned an
// unsupported device-login response."} (device_code opaque shape, user_code
// ^[A-Z0-9-]{4,32}$, verification_uri href == "https://github.com/login/device",
// expires_in 1..1800, interval (default 5) 1..30).
func StartDeviceFlow(ctx context.Context, client *http.Client) (DeviceFlow, error)

// PollOptions injects the clock for tests (mirrors copilot.mjs pollDeviceFlow's
// now/sleep parameters); nil opts use time.Now/time.Sleep.
type PollOptions struct {
    Now   func() time.Time
    Sleep func(d time.Duration)
}

// PollDeviceFlow is the port of pollDeviceFlow (copilot.mjs:157-195): local
// deadline now+ExpiresIn*1000; never requests at/after the deadline; per
// iteration sleeps min(Interval*1000, remaining); POST GitHubDeviceTokenURL
// with the form headers plus device_code and
// grant_type=urn:ietf:params:oauth:grant-type:device_code. access_token →
// opaque check → returned. error: authorization_pending → continue;
// slow_down → Interval += 5; access_denied → Error{Code: "access_denied",
// Message: "GitHub device login was denied or cancelled."}; expired_token →
// Error{Code: "device_expired", Message: "GitHub device login expired."};
// other → Error{Code: "unsupported_response", Message: "GitHub returned an
// unsupported device-login response."}. Deadline exit → device_expired.
func PollDeviceFlow(ctx context.Context, client *http.Client, flow DeviceFlow, opts *PollOptions) (string, error)

// Fetch is the FetchFunc port of checkCopilotUsage (copilot.mjs:244-285)
// minus the interactive --login leg (SPEC §2.3-§2.4, §2.10): empty cred.Token
// → Failure{Code: "login_required", Message: "No usable GitHub token was
// found; rerun with --login to start device login."}; else ValidateIdentity,
// then the usage request, Snapshot.Account = login.
func Fetch(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error)

// Error is the provider failure type. Code is always a value from
// specs/global/CONTRACTS.md §1.6; Message is a sanitized fixed string.
type Error struct {
    Code    string
    Message string
}

func (e *Error) Error() string
```

## 2. Descriptor literal (registered in `init()`)

```go
usage.Register(usage.Descriptor{
    ID:          "copilot",
    DisplayName: "GitHub Copilot",
    Kind:        usage.KindSubscription,
    Tier:        1,
    Auth: []usage.AuthSource{
        // 1. operator override
        {Kind: usage.AuthEnvVar, EnvVar: "COPILOT_API_TOKEN", Validate: ValidateIdentity},
        // 2-4. prototype discovery order (copilot.mjs:48-52); never --local
        {Kind: usage.AuthShell, Shell: &usage.ShellSpec{Command: "git", Args: []string{"config", "--global", "--get", "github.copilot.oauthToken"}, Timeout: 3 * time.Second, MaxOutputBytes: 32 * 1024}, Validate: ValidateIdentity},
        {Kind: usage.AuthShell, Shell: &usage.ShellSpec{Command: "git", Args: []string{"config", "--system", "--get", "github.copilot.oauthToken"}, Timeout: 3 * time.Second, MaxOutputBytes: 32 * 1024}, Validate: ValidateIdentity},
        {Kind: usage.AuthShell, Shell: &usage.ShellSpec{Command: "gh", Args: []string{"auth", "token", "--hostname", "github.com"}, Timeout: 3 * time.Second, MaxOutputBytes: 32 * 1024}, Validate: ValidateIdentity},
        // 5. --login path only (F25); F12 MUST delegate to StartDeviceFlow/PollDeviceFlow
        {Kind: usage.AuthOAuthDeviceFlow, OAuth: &usage.OAuthSpec{ClientID: CopilotClientID, DeviceCodeURL: GitHubDeviceCodeURL, TokenURL: GitHubDeviceTokenURL, Scope: "read:user"}, Validate: ValidateIdentity},
    },
    Windows: []usage.WindowSpec{
        {ID: "premium", Label: "premium interactions", Unit: usage.UnitRequests, Optional: true},
        {ID: "chat", Label: "chat", Unit: usage.UnitRequests, Optional: true},
        {ID: "completions", Label: "completions", Unit: usage.UnitRequests, Optional: true},
    },
    Timeout:  15 * time.Second,
    CacheTTL: 60 * time.Second,
    Fetch:    Fetch,
})
```

## 3. Request contract

- Identity: `GET` `GitHubUserURL`, allow-list `[GitHubUserURL]`, exactly three headers:

| Header | Value |
|---|---|
| `Accept` | `application/vnd.github+json` |
| `Authorization` | `Bearer <token>` |
| `User-Agent` | `which-model/0.4.0` |

- Usage: `GET` `CopilotUsageURL`, allow-list `[CopilotUsageURL]`, exactly six headers (verbatim `copilotUsageHeaders`, `copilot.mjs:93-97`):

| Header | Value |
|---|---|
| `Accept` | `application/vnd.github+json` |
| `Authorization` | `Bearer <token>` |
| `Editor-Version` | `vscode/1.96.2` |
| `Editor-Plugin-Version` | `copilot-chat/0.26.7` |
| `User-Agent` | `GitHubCopilotChat/0.26.7` |
| `X-GitHub-Api-Version` | `2025-04-01` |

- Device code: `POST` `GitHubDeviceCodeURL`, headers `Accept: application/json`, `Content-Type: application/x-www-form-urlencoded`; body `client_id=Iv1.b507a08c87ecfe98&scope=read:user`.
- Device token: `POST` `GitHubDeviceTokenURL`, same headers; body `client_id=...&device_code=<code>&grant_type=urn:ietf:params:oauth:grant-type:device_code`.
- Enforcement (per request, same helper contract as F15 §3): exact-URL allow-list → `endpoint_refused`; redirects → `redirect_refused`; bounded body → `response_too_large`; empty/non-object JSON → `response_json`; deadline → `timeout`; transport → `network`.

## 4. Response JSON shapes (verbatim from `copilot.mjs`)

```jsonc
// GET /user
{ "login": "octocat", "id": 12345678 }   // login ^[A-Za-z0-9-]{1,39}$ required

// GET /copilot_internal/user
{
  "quota_snapshots": {                    // required, non-array object
    "chat":            { "remaining": 225, "entitlement": 300, "percent_remaining": 75, "reset_at": "2030-01-01T00:00:00Z", "unlimited": false },
    "completions":     { /* same shape */ },
    "premium_interactions": { /* same shape */ }
  },
  "quota_reset_date": "2030-01-01"        // per-window reset_at ?? this
}

// POST /login/device/code
{ "device_code": "...", "user_code": "ABCD-1234", "verification_uri": "https://github.com/login/device", "expires_in": 60, "interval": 1 }

// POST /login/oauth/access_token
{ "access_token": "..." }                                       // success
{ "error": "authorization_pending" | "slow_down" | "access_denied" | "expired_token" }
```

## 5. Window ID / unit table

| ID | Label | Unit | Notes |
|---|---|---|---|
| `chat` | chat | requests | from `quota_snapshots.chat` |
| `completions` | completions | requests | from `quota_snapshots.completions` |
| `premium` | premium interactions | requests | from `quota_snapshots.premium_interactions` |

Per window: `Unlimited = (unlimited === true)`; `Remaining = remaining` (finite ≥ 0); `Limit = entitlement`; `UsedPercent = 100 - percent_remaining` when present; `ResetsAt = resetTime(reset_at ?? quota_reset_date)`; skip unless `unlimited` OR `remaining` OR `percent_remaining` present; `UsageKnown: true`.

## 6. Failure.Code mapping

| Condition | Code | Message (verbatim) |
|---|---|---|
| empty cred token | `login_required` | `No usable GitHub token was found; rerun with --login to start device login.` |
| identity/login 401/403 | `unauthorized` | `GitHub identity rejected the credential.` / `GitHub Copilot rejected the credential.` |
| identity/login 429 | `rate_limited` | `GitHub identity rate-limited the usage request.` / `GitHub Copilot rate-limited the usage request.` |
| identity/login other non-200 | `provider_status` | `GitHub identity usage is unavailable (HTTP <status>).` / `GitHub Copilot usage is unavailable (HTTP <status>).` |
| login shape invalid | `unsupported_response` | `GitHub returned an unsupported identity response.` |
| device flow non-200 | per `mapStatus("GitHub device login", status)` | e.g. `GitHub device login usage is unavailable (HTTP <status>).` |
| device response validation failure | `unsupported_response` | `GitHub returned an unsupported device-login response.` |
| `access_denied` | `access_denied` | `GitHub device login was denied or cancelled.` |
| `expired_token` / deadline | `device_expired` | `GitHub device login expired.` |
| quota_snapshots absent/non-object/array or zero windows | `unsupported_response` | `GitHub Copilot returned an unsupported usage shape.` |
| URL not exactly allow-listed | `endpoint_refused` | `The provider endpoint was refused.` |
| any 3xx | `redirect_refused` | `The provider attempted an unsafe redirect.` |
| body over 256 KiB | `response_too_large` | `The provider response exceeded the safe size limit.` |
| empty/unparseable JSON body | `response_json` | `The provider returned an empty response.` / `...unsupported JSON.` |
| context deadline | `timeout` | `The provider request timed out.` |
| transport failure | `network` | `The provider request failed.` |

## 7. Config keys, flags, env

- Config keys owned: none. Enablement is `[providers.copilot]` (F01/F21 domain).
- Flags owned: none. `--login`, `--show-identity`, and the login banner line `Open <verification_uri> and enter code <user_code>.` belong to F25/F24; F25 consumes `StartDeviceFlow`/`PollDeviceFlow`/`DeviceFlow`.
- Env: consumes `COPILOT_API_TOKEN` (declared via the Auth chain; F12 reads it).
- Commands consumed (via the Auth chain, F12's CLI resolver): `git config --global --get github.copilot.oauthToken`, `git config --system --get github.copilot.oauthToken`, `gh auth token --hostname github.com` — each 3s timeout, 32 KiB cap, failure → no candidate, stdout gets exactly one trailing newline stripped.

## 8. Requirements on F12/F14 (consumed contracts)

1. F12 CLI resolver MUST implement SPEC §2.2 semantics (3s `exec.CommandContext`, 32 KiB cap, swallow-to-no-candidate, strip one trailing `\r\n`/`\n`, then opaque shape check; shape failure → no candidate).
2. F12 chain rule: any candidate failing its `Validate` (this package's `ValidateIdentity`) is skipped and the chain continues; after the last source, resolution yields no candidate (F14 gets a zero-value `Credential` and `Fetch` produces `login_required`). Hard failures before validation (`credential_json`, `unsafe_credential` from env/files) are F12's to emit.
3. F12's `AuthOAuthDeviceFlow` resolver for this provider MUST call `copilot.StartDeviceFlow` and `copilot.PollDeviceFlow` — no duplicated state machine (SPEC D11).
4. F14 MUST invoke `Fetch` with the chain-resolved `Credential` (zero value when empty) under `context.WithTimeout(ctx, descriptor.Timeout)`; it reads `Snapshot.Failure` and never attaches codes itself. `Snapshot.Account` (verified login) is output-gated by F24's `--show-identity`.
5. Provider failures are `(Snapshot{Provider:"copilot", Failure: ...}, nil)`; the `error` return is reserved for programming errors.

## Snapshot knowledge correction (#182)

Successful fetches set the existing `Snapshot.UsageKnown` field to whether any normalized window has `UsageKnown && !Synthetic` (global CONTRACTS §1.5). A real zero reading, credits-only reading, or unlimited known window counts; synthetic-only and failed snapshots remain false. This corrects an omitted aggregate assignment without changing canonical types or the JSON schema. The aggregate flag survives F14 live fetch, cache serialization/replay, and JSON output.

| Snapshot contents | `usage_known` |
|---|---|
| Real positive or zero reading | `true` |
| Real credits-only or unlimited known reading, when supported | `true` |
| Mixed real and synthetic windows, when supported | `true` |
| Synthetic-only windows, when supported | `false` |
| Provider failure | `false` |
