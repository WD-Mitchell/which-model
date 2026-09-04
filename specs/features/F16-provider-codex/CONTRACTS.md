---
kind: feature-contracts
feature: F16-provider-codex
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F16 — provider-codex: Contracts

Package: `internal/usage/provider/codex`
Files: `internal/usage/provider/codex/codex.go` (constants, Descriptor, `init`), `internal/usage/provider/codex/codex_config.go`, `internal/usage/provider/codex/codex_credential.go`, `internal/usage/provider/codex/codex_normalize.go`, `internal/usage/provider/codex/codex_fetch.go`, plus `_test.go` twins and `internal/usage/provider/codex/testdata/usage/codex/*.json` fixtures.

Consumed canonical types (cite verbatim, never redefine): `usage.Window`, `usage.Snapshot`, `usage.Failure`, `usage.Unit`, `usage.Kind`, `usage.Source`, `usage.Credential`, `usage.Descriptor`, `usage.AuthSource`, `usage.WindowSpec` — `specs/global/CONTRACTS.md §1` and F11's `internal/usage/types.go` (annex-a §5). F05 helpers cited verbatim per the pinned F05 surface: `security.ValidateOpaqueToken`, `security.ReadBoundedFile`, `security.ValidateExactHTTPS`, `security.ReadResponseBounded`, `security.MaxCredentialBytes`, `security.MaxResponseBytes`.

## 1. Exported symbols

```go
package codex

import (
    "context"
    "net/http"
    "time"

    "github.com/WD-Mitchell/which-model/internal/usage"
)

// UsageURL is the exact allow-listed primary endpoint (codex.mjs:5).
const UsageURL = "https://chatgpt.com/backend-api/wham/usage"

// FallbackStatuses are the only statuses that may trigger the trusted-origin
// fallback (codex.mjs:6).
var FallbackStatuses = map[int]bool{404: true, 405: true, 410: true, 501: true}

// WithTrustedOrigin stores the per-invocation trusted origin (F25's
// --trust-configured-origin) for Fetch. Absent value == untrusted.
func WithTrustedOrigin(ctx context.Context, origin string) context.Context

// TrustedOriginFrom extracts the value set by WithTrustedOrigin ("" when unset).
func TrustedOriginFrom(ctx context.Context) string

// ParseConfig is the port of parseCodexConfig (codex.mjs:8-35): returns the
// active provider's [model_providers.<id>] base_url, else the root base_url,
// else "" (no error for absent values).
func ParseConfig(text string) string

// Credential is the loader result (port of loadCodexCredential's return).
type Credential struct {
    Token            string
    AccountID        string
    ConfiguredBaseURL string // "" when neither auth.json nor config.toml configured one
}

// LoadCredential is the port of loadCodexCredential (codex.mjs:52-61).
// Paths default to $CODEX_HOME/{auth.json,config.toml} when CODEX_HOME is set,
// else ~/.codex/{auth.json,config.toml} (SPEC D2). auth.json is read bounded
// (security.ReadBoundedFile + security.MaxCredentialBytes). Errors:
// credential_file "Codex credentials were not found; sign in with Codex
// first." (missing), credential_file "The credential file has an invalid
// size." (oversized), credential_json "The credential file is not valid JSON.",
// unsafe_credential "The Codex access token is missing or unsafe.",
// unsafe_credential "The ChatGPT account identifier is missing or unsafe.".
// A missing/unreadable/oversized config.toml is silently ignored (returns ""
// for ConfiguredBaseURL).
func LoadCredential(authPath, configPath string) (Credential, error)

// NormalizeUsage is the port of normalizeCodexUsage (codex.mjs:63-81) plus the
// annex-a §3.1 additional_rate_limits mapping (SPEC §2.10-11). Input is the raw
// JSON object body of a 200 response. Window order: 5h, weekly, credits, then
// additional:* in response order. Returns Error{Code: "unsupported_response",
// Message: "Codex returned an unsupported usage shape."} when zero windows are
// produced.
func NormalizeUsage(raw []byte) ([]usage.Window, error)

// Fetch is the FetchFunc port of checkCodexUsage (codex.mjs:102-117) per SPEC
// §2.5-§2.9, §2.12. Provider failures are returned as
// (Snapshot{Provider:"codex", Failure: ...}, nil); programming errors as
// error. The trusted origin is read from ctx (WithTrustedOrigin).
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
    ID:          "codex",
    DisplayName: "Codex",
    Kind:        usage.KindSubscription,
    Tier:        1,
    Auth: []usage.AuthSource{
        // one entry per tolerated token shape, over [$CODEX_HOME, ~/.codex]/auth.json
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "tokens.access_token"},
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "tokens.accessToken"},
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "auth.access_token"},
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "auth.accessToken"},
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "access_token"},
        {Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "accessToken"},
    },
    Windows: []usage.WindowSpec{
        {ID: "5h", Label: "primary window", Unit: usage.UnitPercent, Optional: true},
        {ID: "weekly", Label: "secondary window", Unit: usage.UnitPercent, Optional: true},
        {ID: "credits", Label: "credits", Unit: usage.UnitCredits, Optional: true},
    },
    Timeout:  15 * time.Second,
    CacheTTL: 60 * time.Second,
    Fetch:    Fetch,
})
```

## 3. Request contract

- Primary: `GET` `UsageURL`, allow-list `[]string{UsageURL}`.
- Fallback: `GET` `<configured-base>/api/codex/usage` (target per `validateTrustedBaseUrl` semantics, SPEC §2.8), allow-list `[]string{target}`.
- Headers (exact set, verbatim `fetchCodex`, `codex.mjs:83-92` — no User-Agent):

| Header | Value |
|---|---|
| `Accept` | `application/json` |
| `Authorization` | `Bearer <token>` |
| `ChatGPT-Account-Id` | `<account_id>` |

- Enforcement (per request, same helper contract as F15 §3): exact-URL allow-list → `endpoint_refused`; redirects (3xx) via `CheckRedirect` → `http.ErrUseLastResponse` → `redirect_refused`; `security.ReadResponseBounded(resp, security.MaxResponseBytes)` → `response_too_large`; empty/non-object body → `response_json`; `context.DeadlineExceeded` → `timeout`; other transport errors → `network`.
- Fallback preconditions: primary status ∈ `FallbackStatuses`; `ConfiguredBaseURL != ""`; `TrustedOriginFrom(ctx)` non-empty and origin-equal to the configured base (SPEC §2.8). Violations: `fallback_unavailable`, `untrusted_origin`, `endpoint_refused` respectively. The fallback request itself never carries userinfo, query, or fragment.

## 4. Config file shapes (field names verbatim)

```toml
# ~/.codex/config.toml (or $CODEX_HOME/config.toml)
model_provider = "trusted"            # root key, quotes optional in practice; parser accepts "..." or '...'
base_url = "https://fallback.example" # root fallback
[model_providers.trusted]
base_url = "https://trusted.example/v1"   # wins when activeProvider == "trusted"
[model_providers.other]
base_url = "https://other.example/v1"     # ignored unless active
```

```jsonc
// ~/.codex/auth.json (or $CODEX_HOME/auth.json)
{
  "tokens": { "access_token": "...", "account_id": "..." },  // or value.auth / flat value; accessToken/accountId/chatgpt_account_id variants
  "base_url": "https://...",  // or baseUrl / openai_base_url; else config.toml per ParseConfig
  "openai_base_url": "https://..."  // legacy fallback
}
```

## 5. Response JSON shape (verbatim from `codex.mjs`/annex-a §3.1)

```jsonc
{
  "rate_limit": {                      // or "rateLimit"; absent → the top-level object is used
    "primary_window": {                // or "primaryWindow"; ID "5h", label "primary window"
      "used_percent": 20,              // or "usedPercent"; number or numeric string, 0..100
      "reset_at": 1900000000,          // or "resetAt"; epoch seconds (>10e9 = ms)
      "limit_window_seconds": 18000    // or "limitWindowSeconds"; WindowMinutes = /60 when finite positive integer
    },
    "secondary_window": { /* same shape */ }   // ID "weekly", label "secondary window"
  },
  "credits": { "balance": 12.5 },      // window ID "credits", Unit credits, Remaining = balance
  "additional_rate_limits": [          // or "additionalRateLimits" (annex-a §3.1)
    {
      "limit_name": "o1-mini-weekly",  // or "limitName" → ID "additional:o1-mini-weekly", label = value
      "metered_feature": "o1-mini",    // or "meteredFeature" → ModelScope [value]
      "rate_limit": { "primary_window": { /* same window shape */ } }  // or "rateLimit"; primary else secondary window
    }
  ]
}
```

## 6. Window ID / unit table

| ID | Label | Unit | Notes |
|---|---|---|---|
| `5h` | primary window | percent | snake or camel key; WindowMinutes from `limit_window_seconds/60` |
| `weekly` | secondary window | percent | same |
| `credits` | credits | credits | `Remaining = balance`, no percent |
| `additional:<slug(limit_name)>` | limitName | percent | `ModelScope [metered_feature]` when present; percent from primary else secondary window |

## 7. Failure.Code mapping

| Condition | Code | Message (verbatim) |
|---|---|---|
| auth.json missing | `credential_file` | `Codex credentials were not found; sign in with Codex first.` |
| auth.json oversized/unreadable | `credential_file` | `The credential file has an invalid size.` (or the safe-read message) |
| auth.json not valid JSON / not object | `credential_json` | `The credential file is not valid JSON.` |
| token fails opaque shape | `unsafe_credential` | `The Codex access token is missing or unsafe.` |
| account ID fails identifier shape | `unsafe_credential` | `The ChatGPT account identifier is missing or unsafe.` |
| config.toml missing/unreadable/oversized | (silently absent) | — |
| URL not exactly allow-listed | `endpoint_refused` | `The provider endpoint was refused.` |
| fallback target escapes origin | `endpoint_refused` | `The configured Codex fallback endpoint was refused.` |
| primary status 401/403 | `unauthorized` | `Codex rejected the credential.` |
| primary status 429 | `rate_limited` | `Codex rate-limited the usage request.` |
| primary other non-{404,405,410,501} | `provider_status` | `Codex usage is unavailable (HTTP <status>).` |
| primary ∈ fallback set, no configured base | `fallback_unavailable` | `Codex did not advertise a configured fallback endpoint.` |
| configured base untrusted/unparseable | `untrusted_origin` | `The configured Codex fallback origin was not explicitly trusted.` |
| fallback 401/403 | `unauthorized` | `Codex fallback rejected the credential.` |
| fallback 429 | `rate_limited` | `Codex fallback rate-limited the usage request.` |
| fallback other non-200 | `provider_status` | `Codex fallback usage is unavailable (HTTP <status>).` |
| any 3xx | `redirect_refused` | `The provider attempted an unsafe redirect.` |
| body over 256 KiB | `response_too_large` | `The provider response exceeded the safe size limit.` |
| empty/unparseable JSON body | `response_json` | `The provider returned unsupported JSON.` (empty: `...an empty response.`) |
| context deadline | `timeout` | `The provider request timed out.` |
| transport failure | `network` | `The provider request failed.` |
| zero windows | `unsupported_response` | `Codex returned an unsupported usage shape.` |

## 8. Config keys, flags, env

- Config keys owned: none. Enablement is `[providers.codex]` (F01/F21 domain).
- Flags owned: none. `--trust-configured-origin` belongs to F25 (`auth login`/usage invocation in annex-d); this feature only consumes its value via `WithTrustedOrigin`.
- Env: consumes `CODEX_HOME` (path override for `auth.json`/`config.toml`).
- File paths: `$CODEX_HOME/auth.json` else `~/.codex/auth.json`; `$CODEX_HOME/config.toml` else `~/.codex/config.toml`.

## 9. Requirements on F12/F14 (consumed contracts)

1. F12 `AuthFile` resolver rules as in F15 §8.1 (missing file → skip; missing JSONPath → no candidate; malformed → `credential_json`; unsafe token → `unsafe_credential`). `$CODEX_HOME` and `~` in `FilePaths` are expanded.
2. F12 chain entries for this provider MUST NOT carry `ExtraPaths` (SPEC D1) — the loader is the operational resolver.
3. F14 MUST invoke `Fetch` with the chain-resolved `Credential` (zero value when empty) under `context.WithTimeout(ctx, descriptor.Timeout)`, and MUST forward `WithTrustedOrigin` context when F25 provided it. F14 reads `Snapshot.Failure`; it never attaches failure codes itself.
4. Provider failures are `(Snapshot{Provider:"codex", Failure: ...}, nil)`; the `error` return is reserved for programming errors.

## Snapshot knowledge correction (#182)

Successful fetches set the existing `Snapshot.UsageKnown` field to whether any normalized window has `UsageKnown && !Synthetic` (global CONTRACTS §1.5). A real zero reading, credits-only reading, or unlimited known window counts; synthetic-only and failed snapshots remain false. This corrects an omitted aggregate assignment without changing canonical types or the JSON schema. The aggregate flag survives F14 live fetch, cache serialization/replay, and JSON output.

| Snapshot contents | `usage_known` |
|---|---|
| Real positive or zero reading | `true` |
| Real credits-only or unlimited known reading, when supported | `true` |
| Mixed real and synthetic windows, when supported | `true` |
| Synthetic-only windows, when supported | `false` |
| Provider failure | `false` |
