---
kind: feature-contracts
feature: F15-provider-claude
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F15 — provider-claude: Contracts

Package: `internal/usage/provider/claude`
Files: `internal/usage/provider/claude/claude.go` (constants, Descriptor, `init`), `internal/usage/provider/claude/claude_credential.go`, `internal/usage/provider/claude/claude_normalize.go`, `internal/usage/provider/claude/claude_fetch.go`, plus `_test.go` twins and `internal/usage/provider/claude/testdata/usage/claude/*.json` fixtures.

Consumed canonical types (cite verbatim, never redefine): `usage.Window`, `usage.Snapshot`, `usage.Failure`, `usage.Unit`, `usage.Kind`, `usage.Source`, `usage.Credential`, `usage.Descriptor`, `usage.AuthSource`, `usage.WindowSpec` — `specs/global/CONTRACTS.md §1` (Window §1.4, Snapshot §1.5, Failure §1.6) and F11's `internal/usage/types.go` (Descriptor/AuthSource/WindowSpec/Credential per `docs/plan/annex-a-provider-matrix.md` §5). F05 helpers cited verbatim per the pinned F05 surface: `security.ValidateOpaqueToken`, `security.HasBroadPermissions`, `security.ReadBoundedFile`, `security.ValidateExactHTTPS`, `security.ReadResponseBounded`, `security.MaxCredentialBytes`, `security.MaxResponseBytes`.

## 1. Exported symbols

```go
package claude

import (
    "context"
    "net/http"
    "time"

    "github.com/WD-Mitchell/which-model/internal/usage"
)

// UsageURL is the exact allow-listed endpoint (claude.mjs:15).
const UsageURL = "https://api.anthropic.com/api/oauth/usage"

// UserAgent is the fixed client identity sent on the usage request
// (annex-a §3.2; CodexBar fallback version, survey:135).
const UserAgent = "claude-code/2.1.0"

// DefaultFilePaths are the ordered credential file candidates, dot-file
// first (annex-a §3.2 items 3-4). "$HOME" is expanded by the caller (F12).
const (
    DotFileRelativePath    = ".claude/.credentials.json"
    PlainFileRelativePath  = ".claude/credentials.json"
)

// FileCredential is the enriched file-leg credential the Fetch derives when
// the chain resolved via AuthFile (SPEC D2).
type FileCredential struct {
    Token           string
    ExpiresAt       *time.Time // nil when the file carries no expiry
    BroadPermissions bool
}

// LoadFileCredential re-reads dotPath then plainPath (bounded, via
// security.ReadBoundedFile with security.MaxCredentialBytes) and extracts the
// tolerant shape value.claudeAiOauth ?? value.oauth ?? value, token
// accessToken ?? access_token, expiry expiresAt ?? expires_at (claude.mjs:17-31).
//
// Returns (FileCredential{}, nil) when neither file exists or neither carries
// a token — the caller falls back to the chain credential. Returns an error
// only for hard failures: credential_file (exists but unreadable/oversized,
// message "Claude credentials were not found; sign in with Claude Code first."
// for missing), credential_json (unparseable/non-object), unsafe_credential
// (token fails security.ValidateOpaqueToken).
func LoadFileCredential(dotPath, plainPath string, now time.Time) (FileCredential, error)

// NormalizeUsage is the port of normalizeClaudeUsage (claude.mjs:33-56) plus
// the annex-a §3.2 extraUsage/limits mapping (SPEC §2 items 7-9). Input is the
// raw JSON object body of a 200 response. Returns windows in fixed order:
// 5h, weekly, sonnet_7d, opus_7d, oauth_apps_7d, routines_7d, extra_usage,
// then dynamic limit:* windows in response order. Returns Error{Code:
// "unsupported_response", Message: "Claude returned an unsupported usage
// shape."} when zero windows are produced and the 5h synthetic rule does not
// apply.
func NormalizeUsage(raw []byte) ([]usage.Window, error)

// Fetch is the FetchFunc port of checkClaudeUsage (claude.mjs:58-91) per
// SPEC §2 items 2-6, 10. It returns (Snapshot, nil) with Snapshot.Failure set
// for provider-level failures, or (Snapshot{}, error) for programming errors
// only; provider errors are *Error with a global Failure.Code.
func Fetch(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error)

// Error is the provider failure type. Code is always a value from
// specs/global/CONTRACTS.md §1.6; Message is a sanitized fixed string that
// never contains credential material.
type Error struct {
    Code    string
    Message string
}

func (e *Error) Error() string
```

## 2. Descriptor literal (registered in `init()`)

```go
usage.Register(usage.Descriptor{
    ID:          "claude",
    DisplayName: "Claude",
    Kind:        usage.KindSubscription,
    Tier:        1,
    Auth: []usage.AuthSource{
        // 1. operator override (annex-a §3.2 item 1)
        {Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_CLAUDE_OAUTH_TOKEN"},
        // 2. macOS keychain (annex-a §3.2 item 2); F12 gates to darwin
        {Kind: usage.AuthKeychainGeneric, Keychain: &usage.KeychainSpec{Service: "Claude Code-credentials"}},
        // 3-8. files, dot-file first; one entry per tolerated token shape
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "claudeAiOauth.accessToken"},
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "claudeAiOauth.access_token"},
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "oauth.accessToken"},
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "oauth.access_token"},
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "accessToken"},
        {Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "access_token"},
    },
    Windows: []usage.WindowSpec{
        {ID: "5h", Label: "five hour", Unit: usage.UnitPercent},                 // always present (real or synthetic, SPEC D5)
        {ID: "weekly", Label: "seven day", Unit: usage.UnitPercent, Optional: true},
        {ID: "sonnet_7d", Label: "seven day Sonnet", Unit: usage.UnitPercent, Optional: true},
        {ID: "opus_7d", Label: "seven day Opus", Unit: usage.UnitPercent, Optional: true},
        {ID: "oauth_apps_7d", Label: "seven day OAuth apps", Unit: usage.UnitPercent, Optional: true},
        {ID: "routines_7d", Label: "seven day Routines", Unit: usage.UnitPercent, Optional: true},
        {ID: "extra_usage", Label: "Extra usage", Unit: usage.UnitUSD, Optional: true},
    },
    Timeout:  15 * time.Second,
    CacheTTL: 60 * time.Second,
    Fetch:    Fetch,
})
```

## 3. Request contract

- Method/URL/allow-list: `GET` `UsageURL`, allow-list `[]string{UsageURL}` (exact match only).
- Headers (exact set, annex-a §3.2):

| Header | Value |
|---|---|
| `Accept` | `application/json` |
| `Authorization` | `Bearer <token>` |
| `Content-Type` | `application/json` |
| `anthropic-beta` | `oauth-2025-04-20` |
| `User-Agent` | `claude-code/2.1.0` |

- Enforcement (per request): `security.ValidateExactHTTPS(UsageURL, allowed)` → `endpoint_refused`; client copy with `CheckRedirect` → `http.ErrUseLastResponse`, any 3xx → `redirect_refused`; `security.ReadResponseBounded(resp, security.MaxResponseBytes)` → `response_too_large`; JSON object decode → `response_json` (empty body included); `ctx.Err() == context.DeadlineExceeded` → `timeout`; other transport errors → `network`.
- The response body is read once, bounded; the token appears only in the `Authorization` header and never in any error text.

## 4. Response JSON shape (field names verbatim from `claude.mjs`/survey:136-143)

```jsonc
{
  "five_hour":        { "utilization": 25, "used_percent": 25, "resets_at": "2030-01-01T00:00:00Z", "reset_at": "..." },
  "seven_day":        { ... same shape ... },
  "seven_day_sonnet": { ... },
  "seven_day_opus":   { ... },
  "seven_day_oauth_apps": { ... },
  // routines try-keys, first present non-null object wins:
  "seven_day_routines" | "seven_day_claude_routines" | "claude_routines" | "routines" | "routine" | "seven_day_cowork" | "cowork": { ... },
  "extraUsage": { "isEnabled": true, "monthlyLimit": 40, "usedCredits": 7.5, "utilization": 18.75, "currency": "USD" },
  "limits": [ { "kind": "weekly", "group": "sonnet", "percent": 41, "resetsAt": "2026-08-07T18:00:00Z", "scope": { "model": { "id": "claude-sonnet-4-5", "display_name": "Claude Sonnet 4.5" } }, "isActive": true } ]
}
```

Per window: `utilization ?? used_percent` (number or numeric string, 0..100 finite), `resets_at ?? reset_at` (ISO-8601 string or epoch number; number `> 10_000_000_000` is ms, else seconds). Null `five_hour`/non-object → synthetic `5h` (SPEC D5).

## 5. Window ID / unit table

| ID | Label | Unit | WindowMinutes | Notes |
|---|---|---|---|---|
| `5h` | five hour | percent | 300 | always emitted (real or synthetic) |
| `weekly` | seven day | percent | 10080 | |
| `sonnet_7d` | seven day Sonnet | percent | 10080 | `ModelScope: ["sonnet"]` |
| `opus_7d` | seven day Opus | percent | 10080 | `ModelScope: ["opus"]` |
| `oauth_apps_7d` | seven day OAuth apps | percent | 10080 | |
| `routines_7d` | seven day Routines | percent | 10080 | try-key chain |
| `extra_usage` | Extra usage | usd | — | `Used=usedCredits`, `Limit=monthlyLimit`, `UsedPercent=utilization` |
| `limit:<slug(kind_group)>` | group or kind | percent | — | dynamic, `ModelScope: [scope.model.id]` when present |

## 6. Failure.Code mapping (status → code)

| Condition | Code | Message (verbatim) |
|---|---|---|
| credential file missing | `credential_file` | `Claude credentials were not found; sign in with Claude Code first.` |
| file exists, unreadable/oversized | `credential_file` | `Claude credentials were not found; sign in with Claude Code first.` |
| file not valid JSON / not an object | `credential_json` | `The credential file is not valid JSON.` |
| token fails opaque shape | `unsafe_credential` | `The Claude access token is missing or unsafe.` |
| expiry known and past | `expired_credential` | `The Claude access token is expired.` |
| URL not exactly allow-listed | `endpoint_refused` | `The provider endpoint was refused.` |
| any 3xx | `redirect_refused` | `The provider attempted an unsafe redirect.` |
| body over 256 KiB | `response_too_large` | `The provider response exceeded the safe size limit.` |
| empty/unparseable JSON body | `response_json` | `The provider returned unsupported JSON.` |
| context deadline | `timeout` | `The provider request timed out.` |
| transport failure | `network` | `The provider request failed.` |
| HTTP 401/403 | `unauthorized` | `Claude rejected the credential.` |
| HTTP 429 | `rate_limited` | `Claude rate-limited the usage request.` |
| other non-200 | `provider_status` | `Claude usage is unavailable (HTTP <status>).` |
| zero windows, no synthetic | `unsupported_response` | `Claude returned an unsupported usage shape.` |

## 7. Config keys, flags, env

- Config keys owned: none. Enablement is `[providers.claude]` (F01/F21 domain).
- Flags owned: none. `--show-identity`/`--include-cost`/`--refresh-usage` are F24's; this feature carries no identity or cost-gating data beyond the windows above.
- Env: consumes `WHICH_MODEL_CLAUDE_OAUTH_TOKEN` (declared via the Auth chain; F12 reads it).
- File paths: `~/.claude/.credentials.json`, `~/.claude/credentials.json` (dot-file first).

## 8. Requirements on F12/F14 (consumed contracts)

1. F12 `AuthFile` resolver: missing file → skip; missing JSONPath on a valid-JSON file → no candidate, continue; malformed JSON/non-object → hard `credential_json`; token failing `security.ValidateOpaqueToken` → hard `unsafe_credential`. `~` and `$VAR` in `FilePaths` are expanded. The resolved `Credential` carries `Source: usage.AuthFile` and `Mode` (POSIX mode) when the file leg won.
2. F12 `AuthEnvVar` resolver: `WHICH_MODEL_CLAUDE_OAUTH_TOKEN` must be trimmed/quote-stripped; empty or shape-invalid values yield no candidate.
3. F12 keychain resolver: service `"Claude Code-credentials"`, darwin-only; "not found" → no candidate; other errors → `keychain_unavailable`.
4. F14 MUST invoke `Fetch` with the chain-resolved `usage.Credential` (zero value when the chain yielded nothing), under a `context.WithTimeout(ctx, descriptor.Timeout)`, with an `*http.Client` whose transport the tests may inject. F14 MUST NOT attach failure codes itself; it reads `Snapshot.Failure`.
5. Fetch returns provider failures as `(Snapshot{Provider:"claude", Failure: ...}, nil)` — the `error` return is reserved for programming errors (never expected).
