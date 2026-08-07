# CodexBar Provider Usage Acquisition — Exhaustive Findings

Repo: `/Users/will/Projects/Software/CodexBar` (Swift, SPM). Scope: usage/quota/rate-limit data acquisition + normalization only (no SwiftUI/widget/packaging code touched).

## LICENSE

Verbatim from `/Users/will/Projects/Software/CodexBar/LICENSE`:
```
MIT License
Copyright (c) 2026 Peter Steinberger
```
Full standard MIT text follows (permission to use/copy/modify/merge/publish/distribute/sublicense/sell, "AS IS" no warranty). **CodexBar is MIT-licensed — porting its logic (including verbatim struct/algorithm reuse) is permitted with attribution retained.**

---

## 1. Architecture: registry, normalization model, dispatch

### 1.1 Provider registry
`Sources/CodexBarCore/Providers/ProviderDescriptor.swift`:
- `ProviderDescriptor` (struct, lines 80–115): `id: UsageProvider`, `metadata: ProviderMetadata`, `branding: ProviderBranding`, `tokenCost: ProviderTokenCostConfig`, `pace: ProviderPaceCapability`, `fetchPlan: ProviderFetchPlan`, `cli: ProviderCLIConfig`.
- `ProviderDescriptorRegistry` (enum, 117–246) is a static dictionary `[UsageProvider: ProviderDescriptor]` (125–192) mapping the `UsageProvider` enum cases to their `XProviderDescriptor.descriptor`. **Correction applied during review: the count is 66, not 68** — verified against the primary source, `Providers.swift:6-71` yields exactly 66 `case` declarations, and `descriptorsByID` has 66 entries. Annex A is authoritative on the count. This is the master provider list (verbatim case list, `ProviderDescriptor.swift:126–191`):
```
codex, openai, azureopenai, claude, clinepass, cursor, opencode, opencodego,
alibaba, alibabatokenplan, qwencloud, factory, gemini, antigravity, copilot,
devin, zai, minimax, manus, kimi, kilo, kiro, vertexai, augment, jetbrains,
moonshot, amp, t3chat, ollama, synthetic, openrouter, elevenlabs, warp,
windsurf, zed, perplexity, mimo, doubao, sakana, abacus, mistral, deepseek,
deepinfra, codebuff, crof, venice, commandcode, qoder, stepfun, bedrock,
grok, groq, llmproxy, litellm, deepgram, poe, chutes, neuralwatt,
clawrouter, longcat, sub2api, wayfinder, zenmux, aiand, zoommate, xai
```
Each entry funnels through `ProviderDescriptor.fetch(context:) → fetchPlan.fetchOutcome(context:provider:) → ProviderFetchPipeline.resolveStrategies` which picks among ordered `ProviderFetchStrategy` implementations per provider (kinds seen: `.apiToken`, `.web` (cookie), `.oauth`, `.localProbe`, `.cli`) with `shouldFallback(on:context:)` controlling cascade (e.g. Copilot API→budget web enrichment; Claude auto→api→oauth→web→cli; Alibaba web→api).

### 1.2 Core normalization types (verbatim)
`Sources/CodexBarCore/UsageFetcher.swift:3–590`:
```swift
public struct RateWindow: Codable, Equatable, Sendable {
    public let usedPercent: Double
    public let windowMinutes: Int?
    public let resetsAt: Date?
    public let resetDescription: String?      // free-text reset description (Claude CLI UI scrape uses this)
    public let nextRegenPercent: Double?       // % restored on next regen tick (rolling-recovery providers)
    public let isSyntheticPlaceholder: Bool    // marks a synthesized 0% window standing in for a missing lane
    public var remainingPercent: Double { max(0, 100 - usedPercent) }
    public func backfillingResetTime(from cached: RateWindow?, now: Date = .init()) -> RateWindow
}

public struct NamedRateWindow: Codable, Equatable, Sendable {
    public let id: String
    public let title: String
    public let window: RateWindow
    public let usageKnown: Bool   // false = reset metadata known but usedPercent not yet real
}

public struct UsageSnapshot: Codable, Sendable {
    public let primary: RateWindow?      // e.g. 5h session
    public let secondary: RateWindow?    // e.g. weekly
    public let tertiary: RateWindow?     // e.g. opus/model-scoped
    public let extraRateWindows: [NamedRateWindow]?
    public let providerCost: ProviderCostSnapshot?
    // ~25 provider-specific optional payloads carried alongside the generic windows, e.g.:
    public let kiroUsage: KiroUsageDetails?
    public let ampUsage: AmpUsageDetails?
    public let zaiUsage: ZaiUsageSnapshot?
    public let minimaxUsage: MiniMaxUsageSnapshot?
    public let openAIAPIUsage: OpenAIAPIUsageSnapshot?
    public let claudeAdminAPIUsage: ClaudeAdminAPIUsageSnapshot?
    public let xaiUsage: XAIUsageSnapshot?
    public let cursorRequests: CursorRequestUsage?
    public let mistralUsage: MistralUsageSnapshot?
    public let deepgramUsage: DeepgramUsageSnapshot?
    public let poeUsage: PoeUsageHistorySnapshot?
    // ... (openRouterUsage, sakanaPayAsYouGo, clawRouterUsage, sub2APIUsage, wayfinderUsage,
    //      groqConsoleUsage, codexResetCredits, mimoUsage, opencodegoUsage, deepseekUsage,
    //      zoommateCreditsHistory, subscriptionExpiresAt/RenewsAt)
    public let updatedAt: Date
    public let identity: ProviderIdentitySnapshot?
    public let dataConfidence: UsageDataConfidence
}
```
(`UsageFetcher.swift:143–590`.) `primary`/`secondary`/`tertiary` + `extraRateWindows` are the **universal** rate-limit lanes; the provider-specific fields are additive payloads for richer UI (credits, per-model breakdowns, cost charts). Nearly every provider's `toUsageSnapshot()` maps into `RateWindow(usedPercent:windowMinutes:resetsAt:resetDescription:...)`.

`ProviderPaceCapability` (`ProviderDescriptor.swift:53–78`) classifies whether a provider's window supports "pace" (burn-rate) projections: `.unsupported`, `.resetDatePresent`, `.windowDurationPresent`, `.windowDuration(minutes:)`, or `.calendarMonthResetWindow` (sentinel `30*24*60` minutes for monthly quotas).

### 1.3 Errors / availability
`UsageError` (623–642): `.noSessions`, `.noRateLimitsFound`, `.decodeFailed` (Codex-specific, local-log driven). `UsageLimitsAvailability.resolve(provider:snapshot:...)` (644–688) has provider-specific unavailability logic hardcoded for `.claude`, `.doubao`, `.antigravity`, `.codex`.

---

## 2. THE THREE PRIORITY PROVIDERS

### 2.1 Codex (ChatGPT / Codex CLI) — id `.codex`

**Credential file**: `$CODEX_HOME/auth.json`, default `~/.codex/auth.json` (`CodexOAuthCredentials.swift:56–63`, via `CodexHomeScope.ambientHomeURL`). JSON shape (parsed `CodexOAuthCredentials.swift:92–154`):
```json
{ "tokens": {"access_token":"...","refresh_token":"...","id_token":"...","account_id":"..."},
  "last_refresh": "2026-...ISO8601",
  "OPENAI_API_KEY": "sk-..."   // alt: raw API-key mode, no tokens object
}
```
Both snake_case and camelCase key variants are tried. Written back atomically via a `0600`-permission staged temp file + `rename()` (`CodexOAuthCredentials.swift:189–237`).

**Token refresh**: `CodexTokenRefresher.swift`. `POST https://auth.openai.com/oauth/token`, JSON body `{"client_id":"app_EMoamEEZ73f0CkXaXp7hrann","grant_type":"refresh_token","refresh_token":"<token>","scope":"openid profile email"}`. Error codes mapped: `refresh_token_expired`→`.expired`, `refresh_token_reused`→`.reused`, `invalid_grant`/`refresh_token_invalidated`→`.revoked`, HTTP 401→`.expired`.

**Direct HTTP usage fetch**: `CodexOAuthUsageFetcher.swift`. Base `https://chatgpt.com/backend-api/`, path `/wham/usage` (or `/api/codex/usage` when the base URL doesn't contain `/backend-api`, config-overridable). `GET`, headers: `Authorization: Bearer <access_token>`, `User-Agent: CodexBar`, `Accept: application/json`, `ChatGPT-Account-Id: <accountId>` (when present). Response `CodexUsageResponse` (`CodexOAuthUsageFetcher.swift:6–318`):
```swift
struct CodexUsageResponse { planType, rateLimit: RateLimitDetails?, credits, individualLimit, additionalRateLimits: [AdditionalRateLimit]? }
struct RateLimitDetails { primaryWindow: WindowSnapshot?, secondaryWindow: WindowSnapshot?, individualLimit }  // keys "primary_window"/"secondary_window"
struct WindowSnapshot { usedPercent: Int, resetAt: Int /*epoch seconds*/, limitWindowSeconds: Int }           // keys "used_percent"/"reset_at"/"limit_window_seconds"
struct AdditionalRateLimit { limitName, meteredFeature, rateLimit: RateLimitDetails }  // per-model sub-limit, e.g. "GPT-5.3-Codex-Spark"
struct SpendControlLimitSnapshot { limit, used, remainingPercent, resetsAt }
```
Secondary endpoint: `fetchRateLimitResetCredits` → same base + `/wham/rate-limit-reset-credits`, adds headers `OpenAI-Beta: codex-1`, `originator: Codex Desktop`, `ChatGPT-Account-ID` (capitalized differently than the usage call).

**Local RPC (subprocess) transport**: `UsageFetcher.swift:925–1181`, `CodexRPCClient`. Spawns `codex -s read-only -a untrusted app-server` via `/usr/bin/env` with an enriched `PATH`, speaks **JSON-RPC 2.0 over stdin/stdout, newline-delimited**. Methods: `initialize` (params `clientInfo:{name,version}`) → notification `initialized`; `account/read`; `account/rateLimits/read`. Timeouts: init 8s, per-request 3s default; process is terminated on timeout. Response structs `RPCAccountResponse`/`RPCRateLimitsResponse`/`RPCRateLimitSnapshot`/`RPCRateLimitWindow{usedPercent,windowDurationMins,resetsAt}`/`RPCCreditsSnapshot{hasCredits,unlimited,balance}`/`RPCSpendControlLimitSnapshot` (`UsageFetcher.swift:692–889`, tolerant snake/camel key decoding throughout).

**Local session-log fallback (cost/token estimation)**: `Sources/CodexBarCore/Vendored/CostUsage/CostUsageScanner.swift:1094–1120`. Default root `$CODEX_HOME/sessions` else `~/.codex/sessions`, plus `~/.codex/archived_sessions`. Parses Codex CLI rollout JSONL logs (`CostUsageScanner+CodexFastJSON.swift`, `+CodexPriority.swift`, `+CodexTruncatedPrefix.swift`) and prices tokens via `ModelsDevPricing.swift` (models.dev pricing table) — this is the local token-cost estimator used when no live rate-limit API data exists.

**Windows**: primary = 5h rolling, secondary = weekly (window minutes come directly from `limit_window_seconds`/1440-style API fields, not hardcoded like Claude). Plan-tier via `RPCAccountDetails.chatgpt(email, planType)` / `CodexUsageResponse.planType`. Per-model sub-limits via `additional_rate_limits`.

### 2.2 Claude (Anthropic) — id `.claude`

Five data sources selectable via `ClaudeUsageDataSource` enum (`ClaudeUsageDataSource.swift`): `.auto | .api (Admin key) | .oauth (OAuth API) | .web (cookies) | .cli (PTY)`.

**Credential discovery**:
- Legacy file: `~/.claude/.credentials.json` (`ClaudeOAuthCredentials.swift:2735–2739`, `historicalDefaultCredentialsFilePath`).
- **macOS Keychain**: service name `"Claude Code-credentials"` (`ClaudeOAuthCredentials.swift:15`), read via `kSecClassGenericPassword`/`kSecAttrService` queries with persistent-ref tracking, fingerprint-based change detection, and a 1800s in-memory cache (`ClaudeOAuthCredentials.swift:171–219`).
- Local session logs at `~/.claude/projects`, `~/.config/claude/projects`, and Claude Desktop's embedded stores (`ClaudeProviderDescriptor.swift:162–165`, `ClaudeDesktopProjectsLocator.swift:19–39`).
- Env overrides: `CODEXBAR_CLAUDE_OAUTH_TOKEN`, `CODEXBAR_CLAUDE_OAUTH_SCOPES`, `CODEXBAR_CLAUDE_OAUTH_CLIENT_ID`.

**OAuth client ID**: `"9d1c250a-e61b-44d9-88ed-5944d1962f5e"` (`ClaudeOAuthCredentials.swift:26`, comment: "Claude CLI's OAuth client ID — this is a public identifier... same client ID used by Claude Code CLI for OAuth PKCE flow").

**Token refresh endpoint**: `POST https://platform.claude.com/v1/oauth/token` (`ClaudeOAuthCredentials.swift:28`).

**OAuth usage API**: `ClaudeOAuthUsageFetcher.swift`. Base `https://api.anthropic.com`, `GET /api/oauth/usage`. Headers: `Authorization: Bearer <access_token>`, `Accept: application/json`, `Content-Type: application/json`, `anthropic-beta: oauth-2025-04-20`, `User-Agent: claude-code/<version>` (version auto-detected, fallback `2.1.0`). Also `GET /api/oauth/profile` for `emailAddress`/`organizationUuid`. Response `OAuthUsageResponse` (247–318):
```swift
fiveHour: OAuthUsageWindow?, sevenDay, sevenDayOAuthApps, sevenDayOpus, sevenDaySonnet,
sevenDayRoutines (tried keys: seven_day_routines/seven_day_claude_routines/claude_routines/routines/routine/seven_day_cowork/cowork),
extraUsage: OAuthExtraUsage?, limits: [OAuthLimitEntry]?
struct OAuthUsageWindow { utilization: Double?, resetsAt("resets_at"): String? }  // ISO8601
struct OAuthLimitEntry { kind, group, percent, resetsAt, scope: {model:{id,display_name}}, isActive }
struct OAuthExtraUsage { isEnabled, monthlyLimit, usedCredits, utilization, currency }  // "Extra usage" spend-limit lane, $ credits
```
429 responses trigger a rate-limit gate (`ClaudeOAuthUsageRateLimitGate`) reading `Retry-After` header.

**Admin API** (org-level, API-key based — for Console/API billing, NOT the subscription): `ClaudeAdminAPIUsageFetcher.swift`. `GET https://api.anthropic.com/v1/organizations/cost_report` and `GET https://api.anthropic.com/v1/organizations/usage_report/messages`, both with query params `starting_at`/`ending_at` (RFC3339)/`bucket_width=1d`/`limit=31`/`group_by[]=description|model`. Headers: `anthropic-version: 2023-06-01`, `x-api-key: <apiKey>`, `Accept: application/json`, `User-Agent: CodexBar/1.0`. Costs are lowest-USD-unit strings (`/100` → USD). This is the **API-key/Console cost-tracking path**, distinct from subscription usage.

**Claude Web (cookie) fetcher**: `ClaudeWebAPIFetcher.swift:122–125` (doc comment, verbatim):
```
GET https://claude.ai/api/organizations                              → org UUID
GET https://claude.ai/api/organizations/{org_id}/usage               → usage percentages + reset times
GET https://claude.ai/api/organizations/{org_id}/prepaid/credits     → remaining "Extra usage" balance
```
Uses browser-cookie import (`BrowserCookieClient`). When `five_hour` is null, synthesizes a `0%` placeholder session window flagged `isSyntheticPlaceholder: true` so UI lane classifiers don't render a phantom active session (`UsageFetcher.swift:16–20`, `ClaudeUsageFetcher.swift:1199–1215`).

**CLI (PTY) path**: `ClaudeUsageFetcher.swift`, `sessionWindowMinutes = 5*60`, `weeklyWindowMinutes = 7*24*60` (line 102–103) — these are the two hardcoded window durations (5h + weekly) used across all Claude sources when the API doesn't supply an explicit duration. Drives the `claude` binary in a PTY and scrapes its usage-summary text output; `resetDescription` carries the free-text reset phrase straight from CLI UI text (per `RateWindow.resetDescription` doc comment).

### 2.3 GitHub Copilot — id `.copilot`

**Auth**: **CodexBar performs its own GitHub OAuth Device Flow** — it does NOT shell out to `gh auth token` and does NOT read `~/.config/github-copilot/hosts.json`. `CopilotDeviceFlow.swift`:
```swift
private let clientID = "Iv1.b507a08c87ecfe98" // VS Code Client ID (comment, line 9)
private let scopes = "read:user"
host = "github.com" (or an enterprise host override)
deviceCodeURL  = https://<host>/login/device/code
accessTokenURL = https://<host>/login/oauth/access_token
```
`requestDeviceCode()`: `POST` form body `client_id`+`scope` → `DeviceCodeResponse{device_code,user_code,verification_uri,verification_uri_complete,expires_in,interval}`. `pollForToken(deviceCode:interval:)`: loops `Task.sleep(interval)`, `POST` `client_id`+`device_code`+`grant_type=urn:ietf:params:oauth:grant-type:device_code`; handles `authorization_pending` (continue), `slow_down` (+5s), `expired_token` (throw `.timedOut`). Returns `access_token`. The resulting token is stored in CodexBar's own settings store (`SettingsStore.copilotAPIToken`, backed by its config snapshot / `~/.codexbar/config.json`), and resolved at fetch time via `ProviderTokenResolver.copilotToken` reading env var `COPILOT_API_TOKEN` (`ProviderTokenResolver.swift:283–287`) — i.e. a user-pasted `gh auth token` value also works, but CodexBar's own login flow is the primary path.

**Usage endpoint**: `CopilotUsageFetcher.swift`. `apiHost` = `api.github.com` for default host, else `api.<enterpriseHost>`. `GET https://api.github.com/copilot_internal/user`. Headers: `Authorization: token <token>` (note: **`token` scheme, not `Bearer`**, comment: "Use the GitHub OAuth token directly, not the Copilot token"), `Accept: application/json`, `Editor-Version: vscode/1.96.2`, `Editor-Plugin-Version: copilot-chat/0.26.7`, `User-Agent: GitHubCopilotChat/0.26.7`, `X-Github-Api-Version: 2025-04-01`.

**Response shape** `CopilotUsageResponse` (`Sources/CodexBarCore/CopilotUsageModels.swift`):
```swift
struct QuotaSnapshot { entitlement: Double, remaining: Double, percentRemaining: Double, quotaId: String, hasPercentRemaining: Bool, unlimited: Bool; usedPercent = max(0,100-percentRemaining) }
struct QuotaSnapshots { premiumInteractions: QuotaSnapshot?, chat: QuotaSnapshot? }
quotaSnapshots ("quota_snapshots"), copilotPlan ("copilot_plan"), tokenBasedBilling ("token_based_billing"), assignedDate ("assigned_date"), quotaResetDate ("quota_reset_date")
```
`primary` = Premium Interactions window (labelled "Premium"), `secondary` = Chat (labelled "Chat") — see `ProviderMetadata(sessionLabel:"Premium", weeklyLabel:"Chat")` in `CopilotProviderDescriptor.swift:12–13`. `unlimited`/`tokenBasedBilling` quotas surface with `primary=secondary=nil` rather than a fake 0%/100%. `quotaResetDate` parsed via ISO8601 (fractional→plain→`yyyy-MM-dd` fallback chain).

**Extra**: `CopilotBudgetWebFetcher.swift` optionally enriches with a spend-budget window scraped from `GET https://github.com/settings/billing/budgets` (paged, `X-Requested-With: XMLHttpRequest`, `GitHub-Verified-Fetch: true`) when `settings.copilot.budgetExtrasEnabled` and cookie access is configured — this is a **second, cookie-based fallback source layered on top of the token-based API**, added into `UsageSnapshot.extraRateWindows`.

---

## 3. Google family (Gemini / Antigravity / VertexAI) — shared OAuth patterns

### 3.1 Gemini (`.gemini`) — CLI + Cloud Code Private API
Credential files (`GeminiStatusProbe.swift:182–184`): `~/.gemini/oauth_creds.json`, `~/.gemini/settings.json` (auth type read from `json["security"]["auth"]`, enum `GeminiAuthType`: `oauth-personal | gemini-api-key | vertex-ai`). Token refresh: `https://oauth2.googleapis.com/token`. Cloud Code endpoints (all `https://cloudcode-pa.googleapis.com`): `v1internal:loadCodeAssist`, `v1internal:retrieveUserQuota`, `v1internal:fetchAvailableModels`; also `GET https://cloudresourcemanager.googleapis.com/v1/projects`. `GeminiStatusSnapshot.toUsageSnapshot()` (lines 43–88) groups per-model quota into **three tiers by name substring**: Pro→`primary`, Flash→`secondary`, Flash-Lite→`tertiary`, each a fixed 1440-minute (24h) `windowMinutes`, `usedPercent = 100 - percentLeft`. Detects and surfaces Google's own "consumer tier deprecated / migrate to Antigravity" shutdown signal in response bodies (`isConsumerTierDeprecationSignal`, 132–147).

### 3.2 Antigravity (`.antigravity`) — Google OAuth, same Cloud Code backend
OAuth constants (`AntigravityOAuthCredentialsStore.swift:152–158`):
```
authURL  = https://accounts.google.com/o/oauth2/v2/auth
tokenURL = https://oauth2.googleapis.com/token
userInfoURL = https://www.googleapis.com/oauth2/v2/userinfo
scopes = ["https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email"]
```
Credentials cached at `~/.codexbar/antigravity/oauth_creds.json` (`defaultURL`, line 471–474, own app-private cache, not shared with the Antigravity desktop app's ambient install — but the app also probes an installed Antigravity.app bundle at `~/Applications`/`/Applications`). `AntigravityRemoteUsageFetcher.swift`: base `https://cloudcode-pa.googleapis.com`, endpoints `v1internal:loadCodeAssist`, `v1internal:onboardUser`, `v1internal:fetchAvailableModels`, plus `retrieveUserQuota` → `RetrieveUserQuotaResponse{buckets:[RetrieveUserQuotaBucket]}`. User-Agent `"antigravity"`.

### 3.3 VertexAI (`.vertexai`) — reuses `gcloud`'s own OAuth client
`VertexAIOAuthCredentials.swift`: credentials file = `$GOOGLE_APPLICATION_CREDENTIALS` else `$CLOUDSDK_CONFIG/application_default_credentials.json` else `~/.config/gcloud/application_default_credentials.json` (67–90). Two credential shapes: (a) **user ADC** — JSON has `client_id`+`client_secret`+`refresh_token` (Google's own installed-app OAuth client embedded in the gcloud SDK, reused as-is — CodexBar never registers its own client for this provider); (b) **service account** — JSON has `client_email`+`private_key`, in which case CodexBar shells out to `gcloud auth application-default print-access-token` (`/usr/bin/env gcloud ...`, 20s timeout) rather than doing JWT-signing itself. Project ID resolved from `$CLOUDSDK_CONFIG/configurations/config_default` (INI `project=` line) or env `GOOGLE_CLOUD_PROJECT`/`GCLOUD_PROJECT`/`CLOUDSDK_CORE_PROJECT`. Token refresh: `POST https://oauth2.googleapis.com/token`, form body `client_id`+`client_secret`+`refresh_token`+`grant_type=refresh_token` (reuses the gcloud client's own secret). Usage fetch (`VertexAIUsageFetcher.swift`): `GET https://monitoring.googleapis.com/v3/projects/<project>/...` (Cloud Monitoring API time-series query over a 24h window), aggregated into `requestsUsedPercent`/`tokensUsedPercent` against Vertex AI quota metrics.

---

## 4. OpenAI-family cloud/enterprise providers

### 4.1 OpenAI Platform API (`.openai`) — API-key billing, NOT the Codex/ChatGPT subscription
Env vars (`OpenAIAPISettingsReader.swift`): `OPENAI_API_KEY`, `OPENAI_ADMIN_KEY` (tried first), `OPENAI_PROJECT_ID`. Endpoints (`OpenAIAPIUsageFetcher.swift:36–38`, `OpenAIAPICreditBalanceFetcher.swift:122`):
```
GET https://api.openai.com/v1/organization/costs
GET https://api.openai.com/v1/organization/usage/completions
GET https://api.openai.com/v1/dashboard/billing/credit_grants
```
Paginated daily buckets (max 31 days, max 100 pages), tokens (input/cached/output/total) and USD cost per model, accumulated into `OpenAIAPIUsageSnapshot.DailyBucket`. **This is the org/API-key billing surface, explicitly separate from Codex's ChatGPT-subscription rate limits** — confirms the API-key-vs-subscription split the architecture must replicate.

### 4.2 AzureOpenAI (`.azureopenai`)
Env vars: `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_DEPLOYMENT_NAME`, `AZURE_OPENAI_API_VERSION` (default `"2024-10-21"`). No usage/quota API is called — `AzureOpenAIUsageFetcher` only issues a tiny validation `chat/completions` request against the deployment to confirm the key/deployment work and surfaces `endpointHost`/`deploymentName`/`model`/`apiVersion` as identity metadata (`AzureOpenAIUsageSnapshot`, no `RateWindow`). **Azure OpenAI has NO usage-quota API in CodexBar — it's presence/validity-only.**

### 4.3 Bedrock (`.bedrock`) — AWS
Env vars (`BedrockSettingsReader.swift:9–17`): `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`/`AWS_DEFAULT_REGION`, `AWS_PROFILE`; falls back to AWS CLI profile resolution (`BedrockCredentialResolver.swift`) when the CLI is installed. Two APIs: (a) Cost Explorer `https://ce.<region>.amazonaws.com` (`BedrockUsageStats.swift:244`, SigV4-signed via `BedrockAWSSigner`), (b) CloudWatch `https://monitoring.<region>.<suffix>` (`BedrockCloudWatchUsage.swift:190`) queried for Claude-on-Bedrock `InputTokenCount`/`OutputTokenCount`/invocation-count metrics over a 14-day lookback, paginated (max 20 pages, 4MB response cap). Both require full SigV4 request signing (implemented locally, not via AWS SDK).

---

## 5. Coding-agent / IDE tool providers

| Provider | Credential source | Endpoint(s) | Notes |
|---|---|---|---|
| **Cursor** (`.cursor`) | Browser cookie (`WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `wos-session`, `authjs.session-token`, ...) from `cursor.com`/`cursor.sh` domains; local SQLite DB fallback (`CursorStatusProbe.swift:462`, `resolveDefaultDBPath`) | `cursor.com` API (session-cookie authenticated) | `CursorUsageSummary{billingCycleStart/End, membershipType, limitType, isUnlimited, individualUsage:{plan:{used,limit,remaining in **cents**, autoPercentUsed, apiPercentUsed}, onDemand, overall}, teamUsage}` — cents-based USD units (`CursorStatusProbe.swift:235-300`) |
| **Windsurf** (`.windsurf`) | Browser cookie + `WindsurfDevinSessionAuth` local-storage import from `app.devin.ai`/`windsurf.com` origins | `POST https://windsurf.com/_backend/exa.seat_management_pb.SeatManagementService/GetPlanStatus` | **Protobuf wire format**, hand-rolled decoder `WindsurfPlanStatusProtoCodec`/`ProtoReader` (`WindsurfWebFetcher.swift:373-676`) — not JSON |
| **Devin** (`.devin`) | Browser cookie, `storageOrigin = https://app.devin.ai` | `app.devin.ai` (cookie-authenticated) | Chrome UA spoofed (`Chrome/143.0.0.0`) |
| **Augment** (`.augment`) | Browser cookie; NextAuth-pattern session probing across `/api/auth/session`, `/api/session`, `/api/user` (`AugmentSessionKeepalive.swift:371-389`) | `https://app.augmentcode.com` | Keeps session alive by opening `NSWorkspace` browser tab |
| **Factory** (`.factory`, aka "Droid") | Browser cookie + Safari LocalStorage container read; WorkOS auth (`client_01HXRMBQ9BJ3E7QSTQ9X2PHVB7` etc, `FactoryStatusProbe.swift:614-616`); env `FACTORY_API_KEY` | `app.factory.ai`, `auth.factory.ai`, `api.factory.ai`, `api.workos.com/user_management/authenticate` | Headers `x-factory-client: web-app`, `Origin`/`Referer: https://app.factory.ai` |
| **JetBrains AI** (`.jetbrains`) | Local IDE detection only (`JetBrainsIDEDetector`) | none (local probe, `ProviderFetchKind = .localProbe`) | **No usage API in CodexBar** — `JetBrainsStatusFetchStrategy` just reports installed-IDE status |
| **Kiro** (`.kiro`) | Shells out to `kiro-cli` binary (via `TTYCommandRunner.which`), timeouts: account 3s/usage 20s/context 8s | none direct — CLI-mediated | `KiroUsageSnapshot{planName, creditsUsed/Total/Percent, bonusCreditsUsed/Total, bonusExpiryDays, overagesStatus, overageCreditsUsed, estimatedOverageCostUSD, contextUsage:{totalPercentUsed,contextFilesPercent,toolsPercent,...}}`; secondary window = bonus-credit pool with expiry-date reset |
| **Kilo** (`.kilo`) | tRPC session (`https://app.kilo.ai/api/trpc`) + REST fallback `GET https://api.kilo.ai/api/profile`; multi-org via `KiloOrganization`/`KiloUsageScope` | `app.kilo.ai`, `api.kilo.ai` | `KiloUsageSnapshot{creditsUsed/Total/Remaining, passUsed/Total/Remaining/Bonus, passResetsAt, planName, autoTopUpEnabled/Method}` |
| **Zed** (`.zed`) | macOS Keychain: `kSecClassInternetPassword`/`kSecAttrServer` for `zed.dev` server credentials, plus generic-password fallback | `GET https://cloud.zed.dev/client/users/me`; trusted server allow-list `zed.dev`/`staging.zed.dev` | `ZedStatusProbe.defaultKeychainServiceURL = "https://zed.dev"` |
| **Warp** (`.warp`) | API key from Warp CLI config | `POST https://app.warp.dev/graphql/v2?op=GetRequestLimitInfo` | GraphQL; requires `User-Agent` matching official `Warp/1.0`-style pattern or the edge limiter 429s; `clientID = "warp-app"` |
| **OpenCode** (`.opencode`) / **OpenCodeGo** (`.opencodego`) | Browser cookie via `OpenCodeCookieImporter`/`OpenCodeGoZenBalanceFetcher` | `opencode.ai/_server` (opaque server-ID RPC: `workspacesServerID`, `billingServerID`/`subscriptionServerID` — 64-char hex IDs baked in as constants) | Two near-identical providers (OpenCode vs. "OpenCode Go"/Zen) sharing the same backend but different billing server IDs |

---

## 6. Chinese/regional "coding-plan" providers — mostly cookie-scraped or region-forked

All of these follow the same **browser-cookie import → first-party API call** pattern via `BrowserCookieClient`/per-provider `*CookieImporter.swift`, and most support two regional API hosts (global vs. China-mainland) via a `*Region` enum.

| Provider | Region hosts | Endpoint(s) | JSON shape highlights |
|---|---|---|---|
| **MiniMax** (`.minimax`) | `platform.minimax.io` / `platform.minimaxi.com` (`MiniMaxAPIRegion.swift:24-37`) | Coding-plan "remains" API (`codingPlanRemainsPath`) | `MiniMaxUsageSnapshot{planName, availablePrompts, currentPrompts, remainingPrompts, windowMinutes, usedPercent, resetsAt, services:[MiniMaxServiceUsage], billingSummary, pointsBalance, subscriptionExpiresAt/RenewsAt}` — multi-service, ranked by quota-window priority |
| **Z.ai** (`.zai`) | `api.z.ai` (global) / `open.bigmodel.cn` (`ZaiAPIRegion.swift:22-24`) | quota-limit + model-usage APIs (per-hour bars) | `ZaiUsageSnapshot`/`ZaiLimitEntry` (`ZaiLimitType`, `ZaiLimitUnit`), team-scoped (`ZaiUsageScope`), plus per-model hourly-token chart data (`ZaiHourlyBars`) |
| **QwenCloud** (`.qwencloud`) | `home.qwencloud.com` (gateway) + `cs-data.qwencloud.com` (data gateway), product codes `sfm_tokenplansolo_public_intl`/`sfm_bailian` | multi-stage cookie→SEC-token→plan API pipeline (`Shared/AliyunOneConsole/OneConsoleSECTokenResolver.swift`) | `QwenCloudUsageSnapshot{planName, usedQuota/totalQuota/remainingQuota, resetsAt, fiveHourUsedPercent/TotalQuota/ResetsAt, weeklyUsedPercent/TotalQuota/ResetsAt}` — explicit **5h + weekly dual-window** |
| **Alibaba Coding Plan** (`.alibaba`) / **Token Plan** (`.alibabatokenplan`) | `modelstudio.console.alibabacloud.com` (intl) / `bailian.console.aliyun.com` (CN) (`AlibabaCodingPlanAPIRegion.swift:20-24`) | web (cookie) + API-token dual strategy, `AlibabaCodingPlanWebFetchStrategy`/`AlibabaCodingPlanAPIFetchStrategy` | Shares `Shared/AliyunOneConsole` cookie/SEC-token machinery with QwenCloud |
| **Kimi** (`.kimi`) | `api.kimi.com` (Code API, `kimi_code_cli` platform id) / `www.kimi.com` (web) | `POST .../kimi.gateway.billing.v1.BillingService/GetUsages`, `.../kimi.gateway.membership.v2.MembershipService/GetSubscriptionStats` (Connect-RPC style paths) | `KimiUsageDetail{limit,used,remaining,resetTime}` (all string-typed numerics), `KimiSubscriptionBalance{feature,type,amountUsedRatio,expireTime}`, `KimiSubscriptionRateLimit{ratio,enabled,resetTime}` — cookie header `kimi-auth=<token>` |
| **Moonshot** (`.moonshot`) | `api.moonshot.ai` (intl) / `api.moonshot.cn` (China) | standard API-key billing | Same underlying model family as Kimi (Moonshot AI) |
| **DeepSeek** (`.deepseek`) | `api.deepseek.com` (balance), `platform.deepseek.com` (usage/cost/summary) | `GET /user/balance`, `GET /api/v0/usage/amount`, `/usage/cost`, `/users/get_user_summary` | `DeepSeekBalanceResponse{is_available, balance_infos:[{currency,total_balance,granted_balance,topped_up_balance}]}` (string-encoded decimals); platform summary wraps wallets in a `biz_code`/`biz_data` envelope with `normal_wallets`/`bonus_wallets` |
| **Doubao** (`.doubao`, ByteDance/Volcengine) | `ark.cn-beijing.volces.com`, `open.volcengineapi.com` (`Action=GetCodingPlanUsage`/`Action=GetAFPUsage`, Version `2024-01-01`) | AK/SK-signed Volcengine Top OpenAPI, optionally via local `arkcli usage plan` CLI shell-out | Account holds Coding Plan **or** Agent Plan (`GetAFPUsage` = "Agent Flow Points"), mutually exclusive per comment `DoubaoUsageFetcher.swift:269-274`; `DoubaoCodingPlanUsage{status,updateTime,quotas:[Quota]}` |
| **LongCat** (`.longcat`, Meituan) | `longcat.chat` | `/api/v1/user-current`, `/api/lc-platform/v1/tokenUsage` | Meituan-style envelope `{code,message,data}`; **field names undocumented/unminifiable** — extraction is deliberately lenient, walking candidate keys (`LongCatModels.swift:3-8`, explicit comment admitting this) |
| **StepFun** (`.stepfun`) | `platform.stepfun.com` | `QueryStepPlanRateLimit`, `GetStepPlanStatus`, plus **device-registration login flow**: `RegisterDevice`→`SignInByPassword`→`RefreshToken` (all under `/passport/proto.api.passport.v1.PassportService/...`) | Dual billing models post-2026-06-18: legacy **Coding Plan** (5h/weekly rolling, `five_hour_usage_left_rate`/`weekly_usage_left_rate`) vs. current **Token Plan** (monthly credit pool, `plan_credit_rate_limit`) — classified by payload shape, not just `plan_family` (see extensive fragility comment `StepFunUsageFetcher.swift:84-95`). API returns numbers/timestamps inconsistently as JSON ints, floats, OR strings — custom flexible decoders (`StepFunFlexibleNumber`/`StepFunFlexibleTimestamp`) work around this |
| **Qoder** (`.qoder`) | `qoder.com` (intl) / `qoder.com.cn` (China) | `GET /api/v2/me/usages/big_model_credits` | `QoderUsageSnapshot{usedCredits,totalCredits,remainingCredits,usagePercentage,unit,resetsAt}` — simple credits model |
| **Sakana** (`.sakana`) | `console.sakana.ai` | `GET /billing`, `GET /billing?tab=payAsYouGo` | Pay-as-you-go usage enrichment race-budgeted to 200ms so a slow secondary call never blocks the primary fetch |

---

## 7. Aggregator / proxy / gateway providers

| Provider | Endpoint | Unit / shape |
|---|---|---|
| **OpenRouter** (`.openrouter`) | `https://openrouter.ai/api/v1` — `GET /credits` (`{data:{total_credits, total_usage}}`, USD), `GET /key` (quota + `RateLimit`, `KeyQuotaStatus`) | USD credits balance = `totalCredits - totalUsage` |
| **LiteLLM** (`.litellm`, self-hosted proxy) | User-configured base URL (`LiteLLMUsageError.missingBaseURL`) — key-info/user-info/team-info endpoints | `LiteLLMUsageSnapshot{userID,accountEmail,personalSpendUSD,personalBudgetUSD,personalResetAt,teamUsage:{...},keyExpiresAt}` — self-hosted, spend-vs-budget in USD |
| **LLMProxy** (`.llmproxy`) | User-configured base URL | Generic proxy usage (same family as LiteLLM/Sub2API) |
| **ClawRouter** (`.clawrouter`) | `https://clawrouter.openclaw.ai` (env override `CLAWROUTER_BASE_URL`) | `ClawRouterUsageSnapshot{budgetConfigured,budgetLedger,budgetLimitUSD,budgetSpentUSD,budgetRemainingUSD,budgetResetsAt,requestCount,successCount,errorCount,inputTokens,outputTokens,totalTokens,actualCostUSD,providers:[ProviderSummary]}` — full token+cost breakdown per upstream provider |
| **Sub2API** (`.sub2api`) | User-configured base URL | `Sub2APIUsageSnapshot{mode,isValid,status,planName,remaining,unit,balance,quota:Quota,rateLimits:[RateLimit],subscription:Subscription,todayUsage/totalUsage:UsageTotals,expiresAt}` — generic multi-kind (subscription-relay vs API-key) abstraction over other providers' plans |
| **Wayfinder** (`.wayfinder`, local LLM routing gateway) | Local gateway (health/models/savings endpoints) | `WayfinderUsageSnapshot{gatewayStatus,offline,dryRun,missingKeys,modelCount,requests,tokens,realized,baseline,saved,savedPct,priced,routes:[RouteSummary],avgDecisionMs}` — cost-optimization/savings metric, not a quota at all |
| **NeuralWatt** (`.neuralwatt`) | `https://api.neuralwatt.com` | `NeuralWattBalance{credits_remaining_usd,total_credits_usd,credits_used_usd,accounting_method}`, `NeuralWattUsage{lifetime,current_month:{cost_usd,requests,tokens,energy_kwh}}`, `NeuralWattSubscription{...,kwh_included,kwh_used,kwh_remaining,in_overage}` — **energy (kWh)-denominated quota**, unique among all providers |
| **ZenMux** (`.zenmux`) | `https://zenmux.ai/api/v1/management` | Management-API credits |
| **Sub2API-family generic** | — | — |

---

## 8. Remaining providers (compact, grep-verified endpoint+header evidence)

| Provider | Endpoint | Auth | Notes |
|---|---|---|---|
| **Abacus.AI** (`.abacus`) | `POST https://apps.abacus.ai/api/_getOrganizationComputePoints`, `_getBillingInfo` | Cookie | Compute-points unit |
| **ai& / AiAnd** (`.aiand`) | `GET https://api.aiand.com/logs` (paged, 100/page, 10 pages max) | API key | Per-request log rows, not a quota API |
| **Amp** (`.amp`, Sourcegraph) | `GET https://ampcode.com/api/internal?userDisplayBalanceInfo` | Cookie | Headers `origin: https://ampcode.com`, `referer` |
| **Chutes** (`.chutes`) | `https://api.chutes.ai` | API key | |
| **ClinePass** (`.clinepass`, Cline) | `GET https://api.cline.bot/api/v1/users/me/plan/usage-limits` | API key (Bearer implied) | |
| **Codebuff** (`.codebuff`) | `https://www.codebuff.com` | Local auth file (`homeDirectory`-relative) | |
| **CommandCode** (`.commandcode`) | `https://api.commandcode.ai/internal/billing/credits`, `/internal/billing/subscriptions` | Cookie/web-origin `commandcode.ai` | Plan totals not returned by API — hardcoded `CommandCodePlanCatalog` keyed by `planId` |
| **Crof** (`.crof`) | `GET https://crof.ai/usage_api/` | API key | |
| **DeepInfra** (`.deepinfra`) | `GET https://api.deepinfra.com/payment/checklist?compute_owed=true`, `/payment/usage?from=current` | API key | `DeepInfraChecklistResponse{stripe_balance,recent,limit,suspended,suspend_reason}`, `DeepInfraUsageResponse{months:[{period,total_cost}]}` (cents) |
| **Deepgram** (`.deepgram`) | `https://api.deepgram.com/v1`, `GET /v1/projects`, usage-by-resolution endpoint | API key | `DeepgramUsageResult{hours,total_hours,agent_hours,tokens_in,tokens_out,tts_characters,requests}` — speech/TTS usage, not LLM tokens |
| **ElevenLabs** (`.elevenlabs`) | `https://api.elevenlabs.io` | API key | |
| **Grok** (`.grok`, xAI CLI) | Local `~/.grok` dir (`GrokAuth.swift:108-110`); CLI shells `grok agent stdio` (ACP JSON-RPC, mirrors Codex's app-server protocol per doc comment) `x.ai/billing` extension method; also web `POST https://grok.com/grok_api_v2.GrokBuildBilling/GetGrokCreditsConfig` (gRPC-Web+proto, headers `x-grpc-web:1`, `Content-Type: application/grpc-web+proto`) | CLI RPC or cookie | `GrokBillingResponse{billingCycle,monthlyLimit:{val:cents},onDemandCap,onDemandEnabled,usage}` — all monetary values `{val:<cents>}` |
| **Groq** (`.groq`) | Console: `POST api.stytchb2b.groq.com` (Stytch B2B session auth) → `GET /platform/v1/organizations/{org}/activity` (org ID parsed from session JWT claim `https://groq.com/organization`, fallback `https://stytch.com/organization`); direct API `https://api.groq.com/v1` | Cookie session (console) or API key | Per-model-per-day spend/usage rows |
| **Manus** (`.manus`) | `POST https://api.manus.im/user.v1.UserService/GetAvailableCredits` (Connect-RPC) | Bearer session token | `ManusCreditsResponse{totalCredits,freeCredits,periodicCredits,addonCredits,refreshCredits,maxRefreshCredits,proMonthlyCredits,eventCredits,nextRefreshTime,refreshInterval}` — richest credits breakdown of any provider |
| **MiMo** (`.mimo`, Xiaomi) | `https://platform.xiaomimimo.com/api/v1/balance` | Cookie | Header `x-timeZone: UTC+01:00` |
| **Mistral** (`.mistral`) | `GET https://admin.mistral.ai/api/billing/v2/usage` (per-category: completion/ocr/connectors/librariesApi/fineTuning/audio), `POST console.mistral.ai/api-ui/trpc/billing.vibeUsage` (tRPC batch call) | Cookie, header `X-CSRFTOKEN` | `MistralBillingResponse` per-model nested usage + `MistralPrice`; separate "Vibe" (agentic coding) usage percentage |
| **Ollama** (`.ollama`) | `https://ollama.com/settings` (cookie scrape), `GET /api/tags`, `/api/web_search` (validation) | Cookie | Local-model tool; cloud usage is web-scraped, no true quota API found |
| **Perplexity** (`.perplexity`) | `GET https://www.perplexity.ai/rest/billing/credits?version=2.18&source=default` | Cookie | `PerplexityCreditsResponse.creditGrants[]` typed `recurring`/`promotional`/`purchased`; usage waterfall-attributed recurring→purchased→promotional; renewal/promo-expiry as Unix-seconds timestamps |
| **Poe** (`.poe`, Quora) | `GET https://api.poe.com/usage/current_balance`, `/usage/points_history` | API key | `PoeUsageSnapshot{currentPointBalance, history:PoeUsageHistorySnapshot}` — points, not $ |
| **Synthetic** (`.synthetic`) | `GET https://api.synthetic.new/v2/quotas` | API key | |
| **T3Chat** (`.t3chat`) | `GET https://t3.chat/api/trpc/getCustomerData` (tRPC batch) | Cookie | |
| **Venice** (`.venice`) | `GET https://api.venice.ai/api/v1/billing/balance` | API key | |
| **XAI** (`.xai`, distinct from Grok CLI) | `https://management-api.x.ai` | API key | Separate provider from `.grok` — API/management-console billing vs. CLI/web usage |
| **ZoomMate** (`.zoommate`, Zoom AI Companion) | `GET https://ai.zoom.us/ai-computer/api/v1/credits/status` (`data.credit_status`, epoch-ms dates), `GET .../credits/history` (paginated) | Minted bearer token via `/ai-computer/api/v1/login/` bootstrap | `ZoomMateCreditStatus{budgetCap,usedCredit}`; multi-host failover between `ai.zoom.us`/`zoommate.zoom.us` |

---

## 9. Providers with NO true usage/quota API (local-only or presence-only)
- **JetBrains AI** — local IDE/CLI detection only, no network call for usage.
- **AzureOpenAI** — validation-only `chat/completions` probe; no billing/usage endpoint called.
- **Ollama** — local server + web-scraped account page; no documented cloud usage API found.
- **Wayfinder** — a local routing gateway; its "usage" is savings/routing telemetry, not a quota.
- **LongCat** — has an API, but its field names are explicitly unknown/unstable (undocumented private API scraped from a minified bundle); extraction is best-effort key-guessing.

---

## 10. OAuth device-flow / token-refresh implementations — summary table

| Provider | Flow | Client ID | Token endpoint | Notes |
|---|---|---|---|
| Copilot | GitHub OAuth **Device Flow** (RFC 8628) | `Iv1.b507a08c87ecfe98` (VS Code's public client ID) | `https://github.com/login/oauth/access_token` | Polls per server `interval`; scope `read:user` |
| Codex | Refresh-token grant (PKCE login is external, via `codex login`) | `app_EMoamEEZ73f0CkXaXp7hrann` | `https://auth.openai.com/oauth/token` | scope `openid profile email` |
| Claude | Refresh-token grant (PKCE login external, via `claude` CLI) | `9d1c250a-e61b-44d9-88ed-5944d1962f5e` | `https://platform.claude.com/v1/oauth/token` | Client ID documented as "public, not secret" |
| Gemini / Antigravity | Google OAuth (installed-app flow, external login via `gemini`/Antigravity app) | (Google's own — reused from CLI/app credential files) | `https://oauth2.googleapis.com/token` | Antigravity additionally defines its own authURL for a from-scratch flow: `accounts.google.com/o/oauth2/v2/auth` |
| VertexAI | Refresh-token grant, reusing `gcloud`'s embedded OAuth client | (from ADC file `client_id`/`client_secret`) | `https://oauth2.googleapis.com/token` | Service-account creds instead shell out to `gcloud auth application-default print-access-token` |
| StepFun | Custom device-registration + password login (not OAuth) | webid-based | `.../PassportService/RegisterDevice`→`SignInByPassword`→`RefreshToken` | Proprietary protocol, not OAuth2 |

---

## 11. API-key vs subscription-plan handling

CodexBar treats these as genuinely separate data sources per provider, often both present simultaneously:
- **Claude**: `.api` (Admin API key → Console/org cost tracking, `ClaudeAdminAPIUsageFetcher` → `cost_report`/`usage_report/messages`) is explicitly distinct from `.oauth`/`.web`/`.cli` (Claude.ai/Claude Code **subscription** rate-limit windows). Both can be enabled at once (`ClaudeUsageDataSource.auto` picks in priority order).
- **OpenAI**: `.openai` (Platform API key, org billing — `/v1/organization/costs`) is a **separate registry entry** from `.codex` (ChatGPT/Codex subscription, OAuth-based). They are two different `UsageProvider` cases, two different descriptors, never conflated.
- **Copilot**: single subscription-quota surface (`copilot_internal/user`); no separate API-key billing view exists for Copilot in CodexBar.
- General pattern: providers with both a per-key API console AND a subscription plan (Claude, OpenAI) get **two registry entries or two source modes**; providers that are purely pay-as-you-go API products (DeepInfra, Deepgram, ElevenLabs, Venice, Synthetic, Poe, OpenRouter, etc.) only ever expose credit-balance/spend endpoints — there is no subscription-window concept for them, and their `RateWindow.windowMinutes` is typically `nil` (a balance, not a rolling window).

## 12. Multi-window / plan-tier / per-model sub-limit patterns observed
- **5h + weekly**: Codex (RPC/OAuth), Claude (`sessionWindowMinutes=300`, `weeklyWindowMinutes=10080`), QwenCloud (`fiveHour*`/`weekly*` fields), StepFun (Coding Plan variant).
- **Per-model sub-limits**: Codex `additionalRateLimits` (e.g. GPT-5.3-Codex-Spark), Claude `sevenDayOpus`/`sevenDaySonnet`/`limits[].scope.model`, Gemini Pro/Flash/Flash-Lite tiers, MiniMax `services:[MiniMaxServiceUsage]`.
- **Plan-tier detection**: Codex `RPCAccountDetails.chatgpt(planType)`/`CodexUsageResponse.planType`; Claude `ClaudePlan.oauthLoginMethod`; Copilot `copilot_plan` string; Gemini `GeminiUserTierId` (`free-tier`/`legacy-tier`/`standard-tier`); Cursor `membershipType`; Kiro `planName`/`displayPlanName`.
- **Weighted usage bands** are NOT implemented in CodexBar itself — it exposes raw `usedPercent`/`remainingPercent` per window and leaves banding/coloring to the UI layer (`UsagePercent.swift`, `UsageChartScale.swift` — out of scope per this investigation's focus, but confirms the source signal (`RateWindow.usedPercent`, 0–100+ double, can exceed 100 for over-quota) is exactly what a ranking engine would band into >75%/>50%/>25% tiers.

## 13. Refresh / caching / rate-limiting behavior
- Claude OAuth: in-memory credential cache valid 1800s (`ClaudeOAuthCredentials.swift:176`); keychain-change fingerprinting checked at minimum 60s intervals; 429 responses gate future calls via `ClaudeOAuthUsageRateLimitGate` honoring `Retry-After`.
- Codex RPC: init timeout 8s, per-call timeout 3s (configurable), process killed on timeout.
- Most HTTP fetchers use per-request timeouts of 15–30s (`timeoutSeconds` constants throughout, e.g. Claude Admin API 20s, ClinePass 15s, Poe 15s, Perplexity default, OpenAI Admin no explicit override shown but consistent 15-30s pattern).
- Bedrock CloudWatch: paginated up to 20 pages, 4MB response cap, 14-day lookback window fixed.
- Sakana: secondary pay-as-you-go enrichment budgeted to 200ms so it never blocks the primary fetch — a "best-effort secondary, never block primary" pattern worth replicating.

## 14. Fragility / drift notes (verbatim-sourced)
- **StepFun**: "StepFun runs two Step Plan billing models side by side after the 2026-06-18 upgrade... classify by the shape the payload actually carries rather than trusting `plan_family` alone" (`StepFunUsageFetcher.swift:84-95`) — API values also arrive as int/float/string inconsistently, requiring custom flexible decoders.
- **LongCat**: "exact `data` field names are not documented and cannot be derived from the minified front-end bundle, so extraction is intentionally lenient" (`LongCatModels.swift:3-8`).
- **Claude web**: explicit handling for a Claude-web quirk where a null `five_hour` session is reported as a synthetic `0%` window that must NOT be treated as a real active session (`RateWindow.isSyntheticPlaceholder` doc comment, `UsageFetcher.swift:16-20`).
- **Doubao**: "Agent Plan usage lives behind a sibling Volcengine Top OpenAPI action... An account holds a Coding Plan *or* an Agent Plan" — mutually exclusive, must try both (`DoubaoUsageFetcher.swift:269-274`).
- **Perplexity**: "Purchased credits may appear in the top-level field, in the credit_grants array..., or both. Take whichever is larger to avoid double-counting" (`PerplexityUsageSnapshot.swift:26-31`).
- **Copilot**: explicit comment that the request must use the *GitHub OAuth token* directly, not a separately-minted Copilot token (`CopilotUsageFetcher.swift:54`).
- **Codex**: RPC responses tolerate both snake_case and camelCase keys throughout (`RPCRateLimitSnapshot`, `SpendControlLimitSnapshot`) — evidence of past API key-casing churn from the `codex app-server` binary.

---

## 15. Final summary table: provider | auth source | endpoint | unit | windows

| Provider | Auth source | Primary endpoint | Unit | Windows |
|---|---|---|---|---|
| Codex | `~/.codex/auth.json` OAuth + local `codex app-server` RPC | `chatgpt.com/backend-api/wham/usage`; local JSON-RPC `account/rateLimits/read` | percent (+credits) | 5h + weekly + per-model additional |
| Claude | Keychain `Claude Code-credentials` / `~/.claude/.credentials.json` OAuth; Admin API key; web cookie; CLI | `api.anthropic.com/api/oauth/usage`; `/v1/organizations/{cost_report,usage_report/messages}`; `claude.ai/api/organizations/{id}/usage` | percent (+USD for Admin API, +credits for Extra usage) | 5h (300min) + weekly (10080min) + Opus/Sonnet scoped |
| Copilot | Own GitHub OAuth Device Flow, token stored in `~/.codexbar/config.json` | `api.github.com/copilot_internal/user` | percent | Premium Interactions + Chat (+ optional budget window) |
| OpenAI (Platform) | `OPENAI_API_KEY`/`OPENAI_ADMIN_KEY` env | `api.openai.com/v1/organization/{costs,usage/completions}` | tokens + USD | daily buckets (31d) |
| AzureOpenAI | env vars, deployment key | validation-only `chat/completions` | n/a | none (no quota API) |
| Bedrock | AWS creds (env/profile/CLI), SigV4 | Cost Explorer + CloudWatch | tokens + USD | 14-day lookback |
| Gemini | `~/.gemini/oauth_creds.json` | `cloudcode-pa.googleapis.com` (retrieveUserQuota) | percent | Pro/Flash/Flash-Lite, 24h each |
| Antigravity | `~/.codexbar/antigravity/oauth_creds.json` (Google OAuth) | `cloudcode-pa.googleapis.com` | percent | quota buckets |
| VertexAI | gcloud ADC file / service account | Cloud Monitoring API | percent | 24h rolling |
| Cursor | browser cookie / local SQLite | `cursor.com` API | USD cents | billing cycle |
| Windsurf | browser cookie / local storage | `windsurf.com/_backend` (protobuf) | percent/credits | plan status |
| MiniMax | browser cookie | `platform.minimax(i).{io,com}` | percent + points | multi-service ranked |
| Z.ai | browser cookie | `api.z.ai` / `open.bigmodel.cn` | percent | per-limit-type |
| QwenCloud | cookie→SEC-token pipeline | `home.qwencloud.com` | quota units | 5h + weekly |
| Kimi | cookie | `www.kimi.com/apiv2/...BillingService` | string-numeric | subscription 7d |
| Doubao | AK/SK signed | `open.volcengineapi.com` | quota levels | Coding Plan or Agent Plan |
| StepFun | device-reg login | `platform.stepfun.com` | percent or credits | 5h+weekly OR monthly credit |
| Qoder | cookie/API | `qoder.com/api/v2` | credits | none (balance) |
| OpenRouter | API key | `openrouter.ai/api/v1/credits` | USD | none (balance) |
| LiteLLM/LLMProxy/Sub2API | self-hosted key | user base URL | USD spend/budget | varies |
| ClawRouter | API key | `clawrouter.openclaw.ai` | tokens+USD | budget ledger |
| Grok (CLI) | local RPC / cookie | `grok agent stdio` ACP / `grok.com` gRPC-Web | USD cents | billing cycle |
| Groq | Stytch session / API key | `api.stytchb2b.groq.com` → `platform/v1/.../activity` | USD/tokens | per-model-per-day |
| Manus | Bearer session | `api.manus.im` Connect-RPC | credits | refresh cycle |
| Perplexity | cookie | `perplexity.ai/rest/billing/credits` | credits (cents) | monthly renewal |
| Poe | API key | `api.poe.com/usage/*` | points | balance + history |
| DeepSeek | cookie/API key | `api.deepseek.com`, `platform.deepseek.com` | USD (string decimal) | wallets |
| Mistral | cookie | `admin.mistral.ai/api/billing/v2/usage` | per-category usage | monthly |
| NeuralWatt | API key | `api.neuralwatt.com` | **kWh energy** + USD | monthly subscription |
| Wayfinder | local gateway | local health/savings API | tokens/USD savings | n/a |
| ZoomMate | minted bearer | `ai.zoom.us/ai-computer/api/v1/credits/status` | credits (epoch-ms) | budget cap |
| Ollama | cookie | `ollama.com/settings` (scrape) | n/a | none (local tool) |
| JetBrains | local only | none | n/a | none |
