# Annex A — Provider Matrix and Credential-Acquisition Plan (`internal/usage/`)

This annex is the authoritative port plan for every usage/quota provider `which-model` ships in `internal/usage/`: the Tier rubric, the full provider inventory with a Tier and port-risk rating, exhaustive request/response/credential specs for the ten Tier-1 providers, the credential-acquisition capability matrix (which Go libraries do the OS-integration work), the concrete `Descriptor`/`AuthSource`/`WindowSpec` Go types and registry pattern, fetch orchestration (concurrency, caching, partial failure), the `Failure.Code` taxonomy, and the drift-resistance policy for these private, undocumented, unversioned APIs. It does not cover the catalog/model-score pipeline (Annex B), agent skills/hooks (Annex C), or the CLI flag surface (Annex D) — see the [master plan](./README.md) for architecture, the routing join, and band/strategy design.

## 1. Scope and tiering rubric

Every provider below corresponds to one `UsageProvider` case in CodexBar's registry (`/Users/will/Projects/Software/CodexBar/Sources/CodexBarCore/Providers/Providers.swift:6-71`). That enum has **66 cases**, not 67 or 68 — `docs/plan/research/codexbar-provider-survey.md:21` describes it as "68 `UsageProvider` enum cases," which this annex corrects against the primary source (`grep -c '^    case ' Providers.swift` inside lines 5-72 → 66). This annex ports all 66; none are silently dropped. Each becomes one `internal/usage/provider/<id>/` package and one `Descriptor` registration (§5).

Tier is assigned by **what the provider is**, not by how hard it is to authenticate against — auth/decode difficulty is captured independently in the `port risk` column so a hard-to-reach provider (browser cookies, AK/SK, protobuf) does not get silently deprioritized if it is actually a primary coding-agent backend.

- **Tier 1** — the provider is a live LLM/model backend that a coding agent (Codex, Claude Code, Copilot, Cursor, Antigravity, Gemini CLI, an OpenRouter-routed client, the Kimi/Z.ai coding CLIs, …) actually dispatches inference requests to, **and** its usage surface exposes a genuine quota/rate-limit *window* — percent-used, tokens, or a countdown of credits with a reset cadence, not just a single cumulative balance — reachable through a scriptable, non-interactive-beyond-OAuth API. Tier 1 is the set the [master plan §3.2](./README.md) canonical `Window` type is designed for *and* has a completed, exhaustively-cited deep spec in this pass: `codex, claude, copilot, gemini, openai, openrouter, cursor, antigravity, zai, kimi`. Other providers that plausibly meet the substantive criteria but were not fully deep-spec'd in this pass are assigned Tier 2 with a note, not silently promoted.
- **Tier 2** — either (a) a real, fetchable usage/quota/credits/balance API exists, but the provider is not itself a model-dispatch target for a coding agent (a proxy/aggregator status page, a speech/TTS billing API, a cloud cost-explorer surface), or (b) it *is* a dispatch target but the account/audience is narrow (single-region, invite-only, enterprise-only), or its quota surface is balance-only (one number, no reset cadence) rather than a rate-limit window. Second-wave port.
- **Tier 3** — presence/validity-only (no usage numbers reachable at all), local-tool-only (no usage network call exists), or the survey explicitly flags the private endpoint as undocumented/field-names-unstable (`research/codexbar-provider-survey.md` §9, §14) such that porting risk currently outweighs value. Deferred indefinitely pending upstream stabilization or a dedicated reverse-engineering pass.

Every Tier-1 and Tier-2 row below cites the CodexBar survey section (or a direct `path:line` when this annex went past the survey to re-verify against source) that backs its endpoint/unit/window claim; rows with no confirmed usage-response shape in the available evidence are marked `unconfirmed` in the *declared window IDs* column and rated `high` port risk with that reason, rather than inventing field names.

## 1a. Usage toggle and the no-credential-access guarantee

Everything in this annex is reachable **only** when three independent gates all pass. See [master plan §6](./README.md) for the toggle design; this section states what it means for credential resolvers, the registry, and fetch.

| Gate | Level | Failing it means |
| --- | --- | --- |
| Binary built without `nousage` | L2, compile time | The code does not exist in the binary |
| `[usage] enabled` resolves true | L1, config; or L0 per-run via `--no-usage` | The code exists but is never invoked |
| `[providers.<id>].enabled = true` | L1a, config, per provider | This provider specifically is never touched |

### 1a.1 Provider default-deny (correction to the earlier default)

An earlier draft of `annex-d-cli-reference.md` §4.2 stated that unlisted providers default to `enabled = true`. That was wrong: it meant a fresh install would attempt all 66 providers and read every credential store on the machine. **Corrected: unlisted providers default to `enabled = false`.**

Hard invariant. `which-model` MUST NOT, for any provider that is not explicitly enabled:

- read a credential file (`~/.codex/auth.json`, `~/.claude/.credentials.json`, `~/.gemini/oauth_creds.json`, …);
- query any Keychain item (`Claude Code-credentials`, `zed.dev`, …);
- open any browser cookie database (Chromium, Safari `binarycookies`, Firefox);
- shell out to any provider CLI (`gh`, `codex`, `claude`, `kiro-cli`, `arkcli`, `grok`, `gcloud`, `auggie`).

A fresh install with no config performs **zero** credential access. Enablement is opt-in, per provider, always.

### 1a.2 Build-tag layout

Every file in `internal/usage/**` that touches credentials, endpoints, or the registry carries:

```go
//go:build !nousage
```

A single sibling stub, `internal/usage/disabled.go`, carries the inverse tag and satisfies the same exported surface:

```go
//go:build nousage

package usage

import (
	"context"
	"errors"
)

// ErrUsageCompiledOut is returned by every exported entry point when the binary
// was built with -tags nousage. It is a sentinel: callers compare with errors.Is.
var ErrUsageCompiledOut = errors.New("usage subsystem compiled out (-tags nousage)")

const Compiled = false // true in the !nousage build

func Registry() []Descriptor                                    { return nil }
func Lookup(id string) (Descriptor, bool)                        { return Descriptor{}, false }
func Fetch(context.Context, []string, Options) ([]Snapshot, error) { return nil, ErrUsageCompiledOut }
func CacheDir() (string, error)                                  { return "", ErrUsageCompiledOut }
```

`Registry()` returning `nil` is not a runtime filter — under `nousage` the `provider/<id>` packages are not compiled or imported at all, so **no `init()` self-registration runs and the registry is empty by construction.** That distinction is the whole point: a runtime filter can be bypassed by a future code path or a config bug, whereas absent code cannot. It is the difference between "chooses not to" and "cannot".

### 1a.3 Auditability

A reviewer verifies a `nousage` binary genuinely cannot reach a provider by checking all three:

| Check | Command | Expected |
| --- | --- | --- |
| No provider endpoint constants linked | `strings which-model \| grep -c 'chatgpt.com/backend-api\|api.anthropic.com\|copilot_internal'` | `0` |
| No credential-store modules linked | `go version -m which-model` | No `browserutils/kooky`, no `zalando/go-keyring` |
| Binary self-reports the variant | `which-model version` | Line reading `usage: compiled-out` |

`which-model usage` and `which-model auth` MUST NOT be registered in the cobra command tree under `nousage` — absent, not present-and-refusing, so `which-model --help` does not advertise capabilities the binary lacks.

---

## 2. Full provider matrix (66 of 66)

`Kind` values: **Sub** = Subscription, **Key** = APIKeyBilling, **Gw** = Gateway, **Loc** = LocalTool. Unit values use the canonical `Unit` enum (`percent`, `tokens`, `credits`, `usd`, `requests`, `kwh`, `none`). Endpoints are `GET` unless noted. Section references are to `research/codexbar-provider-survey.md` unless a `path:line` is given.

| id | display name | Kind | auth source(s) | primary endpoint | unit | declared window IDs | Tier | port risk |
|---|---|---|---|---|---|---|---|---|
| `codex` | Codex / ChatGPT | Sub | OAuth file `~/.codex/auth.json` + local RPC (§2.1) | `chatgpt.com/backend-api/wham/usage`; RPC `account/rateLimits/read` | percent+credits | `5h`,`weekly`,`credits`,`additional:<slug>` | 1 | low — dual HTTP/RPC transport, both well-typed (§2.1) |
| `openai` | OpenAI Platform (API-key billing) | Key | env `OPENAI_API_KEY`/`OPENAI_ADMIN_KEY` (§4.1) | `api.openai.com/v1/organization/{costs,usage/completions}` | usd+tokens | `trailing_30d` | 1 | low — documented public API, paginated (§4.1) |
| `azureopenai` | Azure OpenAI | Key | env `AZURE_OPENAI_*` (§4.2) | validation-only `chat/completions` | none | *(none — presence only)* | 3 | low effort, zero value — no usage API exists (§4.2, §9) |
| `claude` | Claude (Anthropic) | Sub | Keychain `Claude Code-credentials` / `~/.claude/.credentials.json` / `~/.claude/credentials.json`; Admin API key; web cookie; CLI (§2.2) | `api.anthropic.com/api/oauth/usage` | percent(+usd,+credits) | `5h`,`weekly`,`opus_7d`,`sonnet_7d`,`oauth_apps_7d`,`routines_7d`,`extra_usage`,`limit:<slug>` | 1 | med — 4-source credential fan-out, one macOS-only (§2.2) |
| `clinepass` | Cline (ClinePass) | Sub | API key (Bearer implied) (§8) | `api.cline.bot/api/v1/users/me/plan/usage-limits` | unconfirmed | `usage_limits` | 2 | low — plain API-key REST, deep spec deferred |
| `cursor` | Cursor | Sub | browser cookie (`WorkosCursorSessionToken` et al.) / local SQLite (§5) | `cursor.com/api/usage-summary` | usd (cents) | `billing_cycle`,`on_demand`,`team` | 1 | med — cookie-primary, multi-cookie-name fallback (§5) |
| `opencode` | OpenCode | Sub | browser cookie (§5) | `opencode.ai/_server` (opaque server-ID RPC) | unconfirmed | `workspace_balance` | 2 | high — opaque hardcoded server-ID RPC, cookie auth (§5) |
| `opencodego` | OpenCode Go (Zen) | Sub | browser cookie (§5) | `opencode.ai/_server` (distinct billing server ID) | unconfirmed | `subscription_balance` | 2 | high — same opaque RPC family as `opencode` (§5) |
| `alibaba` | Alibaba Coding Plan | Sub | browser cookie + API-token dual (§6) | `modelstudio.console.alibabacloud.com` / `bailian.console.aliyun.com` | quota units | `plan_quota` | 2 | high — multi-stage cookie→SEC-token pipeline shared w/ QwenCloud (§6) |
| `alibabatokenplan` | Alibaba Token Plan | Sub | same as `alibaba` (§6) | same region hosts as `alibaba` | quota units | `plan_quota` | 2 | high — same pipeline as `alibaba` (§6) |
| `qwencloud` | QwenCloud | Sub | cookie→SEC-token pipeline (§6) | `home.qwencloud.com` + `cs-data.qwencloud.com` | quota units | `5h`,`weekly` | 2 | high — same SEC-token pipeline; explicit 5h+weekly dual window (§6) |
| `factory` | Factory ("Droid") | Sub | browser cookie + WorkOS auth (§5) | `app.factory.ai`, `auth.factory.ai`, `api.factory.ai`, `api.workos.com/user_management/authenticate` | unconfirmed | `account_status` | 2 | med — cookie + WorkOS session auth, no confirmed quota field names (§5) |
| `gemini` | Gemini (CLI + Cloud Code) | Sub | `~/.gemini/oauth_creds.json` OAuth (§3.1) | `cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota` | percent | `pro_24h`,`flash_24h`,`flash_lite_24h` | 1 | low — standard Google OAuth refresh, JSON response (§3.1) |
| `antigravity` | Antigravity | Sub | own Google OAuth, `~/.codexbar/antigravity/oauth_creds.json` (§3.2) | `cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota` | percent | `model_quota:<bucket>` | 1 | low — same Cloud Code backend as Gemini, own OAuth client (§3.2) |
| `copilot` | GitHub Copilot | Sub | own GitHub OAuth device flow; `git config`/`gh auth token` chain (§2.3) | `api.github.com/copilot_internal/user` | percent | `premium`,`chat`,`budget` | 1 | med — private endpoint, exact header/scheme sensitivity (§2.3, §14) |
| `devin` | Devin | Sub | browser cookie, `app.devin.ai` (§5) | `app.devin.ai` (cookie-authenticated) | unconfirmed | *(unconfirmed)* | 3 | high — no confirmed quota field names, presence-only evidence (§5) |
| `zai` | Z.ai | Sub | API key, region-selected (§6) | `api.z.ai/api/monitor/usage/quota/limit` (or `open.bigmodel.cn`) | percent+tokens | `limit:time`,`limit:tokens` | 1 | low — plain Bearer API key, typed JSON response (`ZaiUsageStats.swift:363-423`) |
| `minimax` | MiniMax | Sub | browser cookie (§6) | `platform.minimax(i).{io,com}` coding-plan "remains" API | percent+points | `service:<name>`,`points_balance` | 2 | med — cookie auth, multi-service ranked response (§6) |
| `manus` | Manus | Sub | Bearer session token (§8) | `api.manus.im/user.v1.UserService/GetAvailableCredits` (Connect-RPC) | credits | `credits`,`refresh_cycle` | 2 | med — Connect-RPC transport, rich but non-REST shape (§8) |
| `kimi` | Kimi (Moonshot Code API) | Sub | env `KIMI_CODE_API_KEY` / `~/.kimi-code/credentials/kimi-code.json` OAuth / cookie `kimi-auth` (`KimiSettingsReader.swift:1-38`) | `api.kimi.com/coding/v1/usages`; web fallback `www.kimi.com/apiv2/kimi.gateway.billing.v1.BillingService/GetUsages` | string-numeric | `coding_weekly`,`subscription_balance`,`ratelimit_7d` | 1 | med — Code API is plain REST; web fallback is Connect-RPC w/ 10+ spoofed headers (`KimiUsageFetcher.swift:234-262`) |
| `kilo` | Kilo | Sub | tRPC session + REST fallback (§5) | `app.kilo.ai/api/trpc`; `api.kilo.ai/api/profile` | credits | `credits_balance`,`pass_balance` | 2 | med — tRPC batch call, multi-org scoping (§5) |
| `kiro` | Kiro | Sub | CLI shell-out to `kiro-cli` (§5) | none direct — CLI-mediated | credits+percent | `credits`,`bonus_credits`,`context_usage` | 2 | med — subprocess transport only, no raw HTTP (§5) |
| `vertexai` | Vertex AI | Sub | ADC file / service-account + `gcloud` shell-out (§3.3) | Cloud Monitoring API (24h rolling) | percent | `cloud_monitoring_24h` | 2 | high — ADC vs. service-account dual credential shape, `gcloud` shell-out fallback (§3.3) |
| `augment` | Augment | Sub | browser cookie, NextAuth session probing (§5) | `app.augmentcode.com` | unconfirmed | *(unconfirmed)* | 3 | high — probes 3 candidate session endpoints, no confirmed usage fields (§5) |
| `jetbrains` | JetBrains AI | Loc | local IDE detection only (§5) | none | none | *(none)* | 3 | n/a — no usage API exists, local probe only (§5, §9) |
| `moonshot` | Moonshot | Key | standard API-key billing (§6) | `api.moonshot.ai` (intl) / `api.moonshot.cn` (China) | unconfirmed | `usage` | 2 | low — same model family as Kimi, plain API-key billing (§6) |
| `amp` | Amp (Sourcegraph) | Sub | browser cookie (§8) | `ampcode.com/api/internal?userDisplayBalanceInfo` | unconfirmed | `display_balance` | 2 | med — cookie-authenticated internal endpoint (§8) |
| `t3chat` | T3.chat | Sub | browser cookie (§8) | `t3.chat/api/trpc/getCustomerData` (tRPC batch) | unconfirmed | *(unconfirmed — opaque blob)* | 3 | high — not a coding-agent backend, opaque customer-data blob (§8) |
| `ollama` | Ollama | Loc | browser cookie for cloud page scrape (§8) | `ollama.com/settings` (scrape); `/api/tags` local | none | *(none)* | 3 | n/a — local tool, no documented cloud usage API (§8, §9) |
| `synthetic` | Synthetic | Key | API key (§8) | `api.synthetic.new/v2/quotas` | unconfirmed | `quotas` | 2 | low — plain API-key REST, clean single-endpoint quota API (§8) |
| `warp` | Warp | Sub | API key from Warp CLI config (§5) | `POST app.warp.dev/graphql/v2?op=GetRequestLimitInfo` | unconfirmed | `request_limit_info` | 2 | med — GraphQL, strict `User-Agent` fingerprint requirement (§5) |
| `openrouter` | OpenRouter | Gw | env `OPENROUTER_API_KEY` (`OpenRouterProviderDescriptor.swift:40`) | `openrouter.ai/api/v1/credits` | usd | `credits_balance`,`key_quota` | 1 | low — plain Bearer API key, typed JSON (`OpenRouterUsageStats.swift:220-256`) |
| `elevenlabs` | ElevenLabs | Key | API key (§8) | `api.elevenlabs.io` | unconfirmed | *(unconfirmed)* | 3 | n/a — speech/TTS API, not a coding-agent dispatch target |
| `windsurf` | Windsurf | Sub | browser cookie + local-storage import (§5) | `POST windsurf.com/_backend/exa.seat_management_pb.SeatManagementService/GetPlanStatus` | percent/credits | `plan_status` | 2 | high — raw protobuf wire format, hand-rolled decoder needed (§5) |
| `zed` | Zed | Sub | macOS Keychain internet-password `zed.dev` (§5) | `cloud.zed.dev/client/users/me` | unconfirmed | `account_identity` | 2 | med — macOS-only primary credential source, trusted-server allow-list (§5) |
| `perplexity` | Perplexity | Key | browser cookie (§8) | `perplexity.ai/rest/billing/credits` | credits (cents) | `credit_grants` | 2 | med — not a coding-agent backend, but real typed credit-grant data (§8, §14) |
| `mimo` | MiMo (Xiaomi) | Key | browser cookie (§8) | `platform.xiaomimimo.com/api/v1/balance` | unconfirmed | `balance` | 3 | low effort, narrow audience — single-number regional balance (§8) |
| `doubao` | Doubao (ByteDance/Volcengine) | Sub | AK/SK signed; optional `arkcli` shell-out (§6) | `open.volcengineapi.com` (`GetCodingPlanUsage`/`GetAFPUsage`) | quota levels | `coding_plan_quota`,`agent_plan_quota` | 2 | high — Volcengine Top OpenAPI AK/SK signing, mutually-exclusive plan shapes (§6, §14) |
| `sakana` | Sakana AI | Key | API key (§6) | `console.sakana.ai/billing` | unconfirmed | `pay_as_you_go_balance` | 2 | low — plain billing GET, best-effort 200ms secondary (§6, §13) |
| `abacus` | Abacus.AI | Key | browser cookie (§8) | `POST apps.abacus.ai/api/_getOrganizationComputePoints` | unconfirmed | `compute_points` | 2 | med — cookie-authenticated internal endpoint (§8) |
| `mistral` | Mistral | Key | browser cookie, header `X-CSRFTOKEN` (§8) | `admin.mistral.ai/api/billing/v2/usage`; tRPC `console.mistral.ai/api-ui/trpc/billing.vibeUsage` | per-category | `category:<name>`,`vibe_usage` | 2 | med — CSRF-token cookie auth, dual REST+tRPC (§8) |
| `deepseek` | DeepSeek | Key | API key / cookie (§6) | `api.deepseek.com/user/balance`; `platform.deepseek.com/api/v0/usage/*` | usd (string decimal) | `wallet_balance` | 2 | low — API-key REST, string-encoded decimals (§6) |
| `deepinfra` | DeepInfra | Key | API key (§8) | `api.deepinfra.com/payment/{checklist,usage}` | usd (cents) | `checklist_balance`,`monthly_usage` | 2 | low — plain API-key REST (§8) |
| `codebuff` | Codebuff | Sub | local auth file (§8) | `www.codebuff.com` | unconfirmed | *(unconfirmed)* | 3 | high — no confirmed endpoint/response shape in evidence (§8) |
| `crof` | Crof | Key | API key (§8) | `crof.ai/usage_api/` | unconfirmed | `usage` | 2 | low — plain API-key REST (§8) |
| `venice` | Venice | Key | API key (§8) | `api.venice.ai/api/v1/billing/balance` | unconfirmed | `balance` | 2 | low — plain API-key REST (§8) |
| `commandcode` | CommandCode | Sub | browser cookie, origin `commandcode.ai` (§8) | `api.commandcode.ai/internal/billing/{credits,subscriptions}` | unconfirmed | `credits`,`subscription` | 2 | med — cookie auth + hardcoded plan-catalog fallback to maintain (§8) |
| `qoder` | Qoder | Key | cookie/API (§6) | `qoder.com/api/v2/me/usages/big_model_credits` (or `.com.cn`) | credits | `credits` | 2 | low — simple credits model, dual region hosts (§6) |
| `stepfun` | StepFun | Sub | device-registration + password login (§6) | `platform.stepfun.com` (`QueryStepPlanRateLimit`, `GetStepPlanStatus`) | percent or credits | `five_hour_usage_left`,`weekly_usage_left`,`plan_credit` | 2 | high — proprietary (non-OAuth2) login, dual billing-model shape detection, inconsistent JSON typing (§6, §14) |
| `bedrock` | AWS Bedrock | Key | AWS creds (env/profile/CLI), SigV4 (§4.3) | Cost Explorer + CloudWatch | tokens+usd | `cost_explorer_mtd`,`cloudwatch_14d` | 2 | high — full SigV4 request signing, 14-day paginated CloudWatch query (§4.3) |
| `grok` | Grok (CLI) | Sub | local `~/.grok` + CLI RPC / cookie (§8) | local RPC (`grok agent stdio`, ACP JSON-RPC) / `grok.com` gRPC-Web | usd (cents) | `billing_cycle` | 2 | high — gRPC-Web+proto over cookie session, or local ACP RPC (§8) |
| `groq` | Groq | Key | Stytch B2B session / API key (§8) | `api.stytchb2b.groq.com` → `platform/v1/organizations/{org}/activity`; `api.groq.com/v1` | usd/tokens | `per_model_per_day` | 2 | med — Stytch session auth for console path, org ID parsed from JWT claim (§8) |
| `llmproxy` | LLMProxy | Gw | user-configured base URL (§7) | user base URL | usd | `personal_spend`,`team_spend` | 2 | med — self-hosted, endpoint shape varies by deployment (§7) |
| `litellm` | LiteLLM | Gw | user-configured base URL (§7) | user base URL, key/user/team-info endpoints | usd | `personal_spend`,`team_spend` | 2 | med — self-hosted proxy, same family as `llmproxy` (§7) |
| `deepgram` | Deepgram | Key | API key (§8) | `api.deepgram.com/v1/projects` | requests | `usage_by_resolution` | 3 | n/a — speech-to-text API, not an LLM/coding-agent backend (§8) |
| `poe` | Poe (Quora) | Key | API key (§8) | `api.poe.com/usage/current_balance` | points | `points_balance` | 2 | low — plain API-key REST, narrow audience for coding-agent routing (§8) |
| `chutes` | Chutes | Key | API key (§8) | `api.chutes.ai` | unconfirmed | *(unconfirmed)* | 3 | high — no field names/window shape in evidence (§8) |
| `neuralwatt` | NeuralWatt | Key | API key (§7) | `api.neuralwatt.com` | kwh+usd | `monthly_kwh`,`monthly_usd` | 2 | low — plain API-key REST, only kWh-denominated provider (§7) |
| `clawrouter` | ClawRouter | Gw | env `CLAWROUTER_BASE_URL` override, API key (§7) | `clawrouter.openclaw.ai` | tokens+usd | `budget` | 2 | low — plain REST, full per-upstream-provider breakdown (§7) |
| `longcat` | LongCat (Meituan) | Sub | browser cookie (§6) | `longcat.chat/api/v1/user-current`, `/api/lc-platform/v1/tokenUsage` | unconfirmed | `token_usage` (unstable) | 3 | high — field names explicitly undocumented/unstable per survey (§6, §9, §14) |
| `sub2api` | Sub2API | Gw | user-configured base URL (§7) | user base URL | mixed | `quota`,`rate_limits` | 2 | med — self-hosted, generic multi-kind abstraction (§7) |
| `wayfinder` | Wayfinder | Gw | local gateway, no external auth (§7) | local health/models/savings endpoints | tokens/usd | *(savings telemetry, not a quota)* | 3 | n/a — routing/savings metric, not a usage quota (§7, §9) |
| `zenmux` | ZenMux | Gw | management-API key (§7) | `zenmux.ai/api/v1/management` | credits | `credits` | 2 | low — plain management-API REST (§7) |
| `aiand` | ai& (AiAnd) | Key | API key (§8) | `api.aiand.com/logs` (paged) | requests | *(per-request log rows, not a quota)* | 3 | n/a — explicitly not a quota API per survey (§8) |
| `zoommate` | ZoomMate | Sub | minted bearer via `/ai-computer/api/v1/login/` bootstrap (§8) | `ai.zoom.us/ai-computer/api/v1/credits/status` | credits | `credit_status` | 2 | med — bootstrap login flow, multi-host failover (§8) |
| `xai` | xAI (management console) | Key | API key (§8) | `management-api.x.ai` | unconfirmed | `management_billing` | 2 | low — distinct from `grok` CLI, standard management-API billing (§8) |

## 3. Tier 1 deep specs

Every subsection gives: exact credential path(s)/key(s), exact request (method, URL, every header), exact response fields consumed, and the exact `[]Window` mapping. Header casing follows Go's `net/http` canonicalization; the literal strings below are what CodexBar/prototype 1 send on the wire, which is what a golden-file test (§8) must match byte-for-byte.

### 3.1 Codex

**Credential.** `$CODEX_HOME/auth.json`, default `~/.codex/auth.json` (`codexbar-provider-survey.md:94`, confirmed by prototype `usage-allowance-checks/lib/codex.mjs` per `usage-allowance-checks-spec.md:161-167`). JSON: `tokens.access_token`/`tokens.accessToken`, `tokens.account_id`/`tokens.accountId` (falls back to top-level `account_id`/`chatgpt_account_id`), optional flat `OPENAI_API_KEY`. Both snake_case and camelCase tried (survey:94-100). Optional `~/.codex/config.toml` supplies a fallback `base_url` under `[model_providers.<id>]` matching the root `model_provider` key (`usage-allowance-checks-spec.md:151-159`); `auth.json`'s own `base_url`/`baseUrl`/`openai_base_url` takes priority over the config-file value (spec:166).

**Refresh.** `POST https://auth.openai.com/oauth/token`, JSON body `{"client_id":"app_EMoamEEZ73f0CkXaXp7hrann","grant_type":"refresh_token","refresh_token":"<token>","scope":"openid profile email"}` (survey:103).

**Primary request.** `GET https://chatgpt.com/backend-api/wham/usage`. Headers: `Accept: application/json`, `Authorization: Bearer <access_token>`, `ChatGPT-Account-Id: <account_id>` (survey:105; spec:174 confirms the same 3-header set with lowercase `accept`/`authorization`/`chatgpt-account-id` keys — case-insensitive, keep as-is).

**Fallback request** (only on `404,405,410,501` from the primary, never on 401/403/429 — `usage-allowance-checks-spec.md:150,178`): requires an explicit `--trust-configured-origin` argument that must equal `auth.json`'s configured base URL's origin exactly (protocol+host+port, no path/query/fragment on either side — spec:93-116). Target is always `<trusted-origin>/api/codex/usage`. Same headers as primary.

**RPC transport** (local, subprocess). Spawn `codex -s read-only -a untrusted app-server`, JSON-RPC 2.0 newline-delimited over stdin/stdout. `initialize` (8s timeout) → `account/rateLimits/read` (3s timeout per call, process killed on timeout) (survey:115). Response tolerant of snake_case and camelCase throughout.

**Response fields consumed.**
```
RateLimitDetails{ primaryWindow: WindowSnapshot?, secondaryWindow: WindowSnapshot?, individualLimit }
WindowSnapshot{ usedPercent: Int, resetAt: Int (epoch seconds), limitWindowSeconds: Int }
AdditionalRateLimit{ limitName, meteredFeature, rateLimit: RateLimitDetails }
credits.balance: number
```
(survey:106-112.)

**`[]Window` mapping.**
| Source field | Window field |
|---|---|
| `primaryWindow` present | `Window{ID:"5h", Label:"5h", Unit:UnitPercent, UsedPercent:&usedPercent, WindowMinutes:&(limitWindowSeconds/60), ResetsAt:parse(resetAt), UsageKnown:true}` |
| `secondaryWindow` present | same shape, `ID:"weekly"`, `Label:"Weekly"` |
| `additionalRateLimits[i]` | `ID:"additional:"+slug(limitName)`, `ModelScope:[meteredFeature]`, same percent/window mapping as above |
| `credits.balance` | `Window{ID:"credits", Unit:UnitCredits, Remaining:&balance, UsageKnown:true}` (no `Used`/`Limit` — absolute count only, matches prototype's `normalizeCodexUsage`, `usage-allowance-checks-spec.md:172`) |

`Source = SourceOAuth` for the HTTP path, `SourceCLI` for the RPC path. `Confidence = "live"`.

### 3.2 Claude

**Credential — merged ordered chain** (resolves the prototype-vs-CodexBar discrepancy explicitly):
1. Env override `WHICH_MODEL_CLAUDE_OAUTH_TOKEN` (mirrors CodexBar's `CODEXBAR_CLAUDE_OAUTH_TOKEN`, survey:129) — explicit operator override, highest priority, never silently preferred over a real credential without the operator asking for it.
2. **macOS only:** Keychain generic password, service `"Claude Code-credentials"` (survey:127, `ClaudeOAuthCredentials.swift:15`), 1800s in-memory cache with fingerprint-based invalidation (survey:346).
3. `~/.claude/.credentials.json` (leading dot — CodexBar's `historicalDefaultCredentialsFilePath`, survey:126).
4. `~/.claude/credentials.json` (no leading dot — prototype 1's path, `usage-allowance-checks-spec.md:129`).

*Resolution reasoning:* the actual Claude Code CLI on non-macOS platforms stores its OAuth credential as a dotfile (matching CodexBar's "historical default" naming, which implies it predates the Keychain becoming primary on macOS); prototype 1's non-dot path cannot be independently verified against a live Claude Code install from the available evidence. Rather than pick one and silently drop the other candidate, `which-model` probes both paths in the chain — dot-file first because it matches the verified, production-shipped 242k-LOC CodexBar port, non-dot second as a compatibility fallback. Whichever file is found first is loaded via `readCredentialJson` semantics (§7 `credential_json`/`credential_file` codes) with the `oauth = value.claudeAiOauth ?? value.oauth ?? value` nested-or-flat tolerance (`usage-allowance-checks-spec.md:131`), and the file's POSIX mode is checked for `hasBroadPermissions` (spec:38-39) — broad permissions produce a warning, never auto-chmod (§9 security invariant, inherited verbatim).

**OAuth client ID.** `9d1c250a-e61b-44d9-88ed-5944d1962f5e` (survey:131, "public identifier, not secret").

**Refresh.** `POST https://platform.claude.com/v1/oauth/token` (survey:133).

**Usage request.** `GET https://api.anthropic.com/api/oauth/usage`. Headers: `Accept: application/json`, `Authorization: Bearer <access_token>`, `Content-Type: application/json`, `anthropic-beta: oauth-2025-04-20`, `User-Agent: claude-code/<version>` (fallback `2.1.0`) (survey:135). Prototype 1's tested 3-header subset (`accept`/`authorization`/`anthropic-beta`, `usage-allowance-checks-spec.md:144`) is a strict subset of this; `which-model` sends the fuller CodexBar set since it is the one verified against the live, moving-target endpoint in production. Non-200 → `statusError('Claude', status)` (spec:145); 429 gates future calls via `Retry-After` (survey:144, §6 below).

**Response fields consumed.**
```
OAuthUsageResponse{
  fiveHour, sevenDay, sevenDayOAuthApps, sevenDayOpus, sevenDaySonnet,
  sevenDayRoutines (keys tried: seven_day_routines/seven_day_claude_routines/claude_routines/routines/routine/seven_day_cowork/cowork),
  extraUsage: {isEnabled, monthlyLimit, usedCredits, utilization, currency},
  limits: [{kind, group, percent, resetsAt, scope:{model:{id,display_name}}, isActive}]
}
OAuthUsageWindow{ utilization: Double? (0-100 scale, not fraction — bounded by prototype's finitePercent, spec:73), resetsAt: String? (ISO8601) }
```
(survey:136-143.)

**`[]Window` mapping.**
| Source | `ID` | `Label` | `WindowMinutes` | notes |
|---|---|---|---|---|
| `fiveHour` | `5h` | `5h` | 300 | `sessionWindowMinutes` hardcoded, survey:156 |
| `sevenDay` | `weekly` | `Weekly` | 10080 | `weeklyWindowMinutes` hardcoded, survey:156 |
| `sevenDayOpus` | `opus_7d` | `Weekly (Opus)` | 10080 | `ModelScope:["opus"]` |
| `sevenDaySonnet` | `sonnet_7d` | `Weekly (Sonnet)` | 10080 | `ModelScope:["sonnet"]` |
| `sevenDayOAuthApps` | `oauth_apps_7d` | `Weekly (OAuth apps)` | 10080 | |
| `sevenDayRoutines` | `routines_7d` | `Weekly (Routines)` | 10080 | try-key chain above |
| `extraUsage` | `extra_usage` | `Extra usage` | nil | `Unit:UnitUSD`, `Used:&usedCredits`, `Limit:&monthlyLimit`, `UsedPercent:&utilization` |
| each `limits[i]` | `"limit:"+slug(kind+"_"+group)` | `kind`/`group` | nil | `ModelScope:[scope.model.id]` when present |

All windows: `UsedPercent = utilization` directly (no scale conversion), `ResetsAt = parse(resetsAt)`, `UsageKnown:true`. `Source = SourceOAuth`.

**Claude web (cookie) fallback** (`GET claude.ai/api/organizations` → org UUID, `GET .../{org}/usage`, `GET .../{org}/prepaid/credits`, survey:150-153): when `five_hour` is null, synthesize `Window{ID:"5h", Synthetic:true, UsageKnown:false, UsedPercent:nil}` rather than a fake 0%/100% reading — this is the documented CodexBar quirk (survey:355) and must not be treated as "no active session." `Source = SourceWeb`.

**Admin API** (`GET api.anthropic.com/v1/organizations/cost_report`, `.../usage_report/messages`; headers `anthropic-version: 2023-06-01`, `x-api-key: <api_key>`, survey:146) is a **separate** `Descriptor` (`id:"claude-admin"`, `Kind:KindAPIKeyBilling`) — it is org/Console cost tracking, not the subscription surface, and CodexBar treats it as a distinct registry concern (survey §11). `which-model` mirrors that split rather than conflating the two.

### 3.3 GitHub Copilot

**Auth scheme — resolved conflict (a).** Prototype 1 uses `Authorization: Bearer <token>` on both the identity check and the private usage endpoint (`usage-allowance-checks-spec.md:205-206`). CodexBar uses `Authorization: token <token>` on the private `copilot_internal/user` endpoint specifically, with an explicit source comment: "Use the GitHub OAuth token directly, not the Copilot token" (survey:170, §14). **`which-model` uses `Bearer` for the public, documented `GET /user` identity gate** (matches prototype 1's tested header set, and GitHub's REST API documents `Bearer` as valid there) **and `token` for the private `GET /copilot_internal/user` call**, matching CodexBar. Reason: `copilot_internal/user` is an undocumented internal endpoint that must reproduce the exact request shape the real VS Code Copilot Chat client sends to avoid anti-abuse detection; CodexBar's comment is direct evidence from a 242k-LOC, 67-provider production port actively used against the live endpoint, whereas prototype 1's test suite only exercises a mocked `fetch` and never asserts against Copilot's real backend for this specific header (`usage-allowance-checks-spec.md:321` — the described test mocks `Response` objects, not live GitHub traffic).

**Token discovery — resolved conflict (b), merged ordered chain:**
1. Env override `COPILOT_API_TOKEN` (CodexBar's `ProviderTokenResolver.copilotToken`, survey:168) — still passed through the identity-validation gate below before use.
2. `git config --global --get github.copilot.oauthToken`
3. `git config --system --get github.copilot.oauthToken`
4. `gh auth token --hostname github.com`
5. If `--login` was passed and steps 1-4 yielded nothing: CodexBar's GitHub OAuth Device Flow.

Steps 2-4 and their exact ordering, the `--local`-never rule, and the "skip malformed/unauthorized candidates, try the next source" semantics are inherited **verbatim** from prototype 1 (`usage-allowance-checks-spec.md:196-204`) — this is a hardened, test-covered flow (17 tests, §8 of the spec) and CodexBar's own device flow slots in only as step 5, exactly where CodexBar puts it as the last resort when no ambient token exists (survey:160). Command runner: `execFile`-equivalent (Go: `os/exec.CommandContext`) with a **3s timeout**, output capped, any failure (including timeout) treated as "no candidate from this source," never propagated as a hard error (spec:194).

Device flow constants (survey §2.3, §4, §10 — both prototype and CodexBar agree exactly): client ID `Iv1.b507a08c87ecfe98` (VS Code's public client ID), scope `read:user`, `POST https://github.com/login/device/code` (form `client_id`+`scope`), `POST https://github.com/login/oauth/access_token` (form `client_id`+`device_code`+`grant_type=urn:ietf:params:oauth:grant-type:device_code`), `verification_uri` must equal exactly `https://github.com/login/device`, `user_code` format `/^[A-Z0-9-]{4,32}$/`, `expires_in` bounds 1-1800s, `interval` bounds 1-30s, `slow_down` → `interval += 5` (unbounded, cumulative), local wall-clock deadline authoritative (checked before **and** after each sleep — a poll is never issued at/after the deadline even if the server declares a longer window, spec:220-222). Device code and access token are never persisted to disk or printed; only the validated `verification_uri`/`user_code` are shown (spec §9 checklist).

**Identity gate (mandatory, before any usage call).** `GET https://api.github.com/user`, headers exactly `Accept: application/vnd.github+json`, `Authorization: Bearer <token>`, `User-Agent: which-model/<version>` — 3 headers, no editor-version/plugin/api-version keys (spec:205, sorted-key test asserted). Response `login` MUST match `/^[A-Za-z0-9-]{1,39}$/` or the candidate is rejected with `identity_response` (spec:207). Every discovered-or-issued token, from any chain source, MUST pass this gate before it is used against `copilot_internal/user` — a 401 here aborts with `login_required` after exactly 1 HTTP call; the private endpoint is never reached without a validated identity (spec:243,247,338 test 16).

**Usage request.** `GET https://api.github.com/copilot_internal/user`. Headers: `Authorization: token <token>`, `Accept: application/json`, `Editor-Version: vscode/1.96.2`, `Editor-Plugin-Version: copilot-chat/0.26.7`, `User-Agent: GitHubCopilotChat/0.26.7`, `X-Github-Api-Version: 2025-04-01` — 6 headers total, spoofing the VS Code Copilot Chat client for this private endpoint (survey:170, spec:206).

**Response fields consumed.**
```
quota_snapshots: { premium_interactions?, chat?, completions? }
QuotaSnapshot{ entitlement, remaining, percent_remaining, quota_id, unlimited }
copilot_plan, token_based_billing, assigned_date, quota_reset_date
```
(survey:172-178; spec:234-238 gives the exact 3-key iteration `['chat','completions','premium_interactions']` with `_`→space labels.)

**`[]Window` mapping.**
| Source key | `ID` | `Label` |
|---|---|---|
| `quota_snapshots.premium_interactions` | `premium` | Premium |
| `quota_snapshots.chat` | `chat` | Chat |
| `quota_snapshots.completions` | `completions` | Completions |

Per window: `Unlimited = source.unlimited == true`; `Remaining = source.remaining`; `Limit = source.entitlement`; `UsedPercent = 100 - source.percent_remaining` when present; skip the window unless `unlimited` OR `remaining` OR `percent_remaining` is present (entitlement alone is insufficient — spec:237). `ResetsAt = parse(source.reset_at ?? quota_reset_date)` (top-level fallback, spec:238). Optional `budget` window layered in from `CopilotBudgetWebFetcher`'s cookie-based `GET github.com/settings/billing/budgets` (survey:180) when configured — a second, cookie-based enrichment on top of the token-based primary source; failure to fetch it MUST NOT fail the primary snapshot. `Source = SourceOAuth`, `Confidence = "live"`.

### 3.4 Gemini

**Credential.** `~/.gemini/oauth_creds.json`; auth-type gate reads `~/.gemini/settings.json` → `json.security.auth.selectedType` (`GeminiStatusProbe.swift:159-213`); `api-key`/`vertex-ai` types are explicitly unsupported by this fetch path (throw before any request) — only `oauth-personal`/unknown proceed.

**Refresh.** `POST https://oauth2.googleapis.com/token` when `accessToken` is empty or `expiryDate < now` (`GeminiStatusProbe.swift:256-277`).

**Request.** `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota`. Headers: `Authorization: Bearer <access_token>`, `Content-Type: application/json`. Body: `{"project":"<project_id>"}` when a project ID was resolved (from `loadCodeAssist`'s response, else discovered via `GET cloudresourcemanager.googleapis.com/v1/projects`), else `{}` (`GeminiStatusProbe.swift:296-319`).

**Response fields consumed.**
```
QuotaResponse{ buckets: [QuotaBucket] }
QuotaBucket{ remainingFraction: Double?, resetTime: String?, modelId: String?, tokenType: String? }
```
(`GeminiStatusProbe.swift:946-955`.) Grouped by `modelId`, keeping the **lowest** `remainingFraction` per model when a model has multiple buckets (worst-case token type wins, `GeminiStatusProbe.swift:966-978`).

**`[]Window` mapping.** Models are bucketed by substring match on the lowercased `modelId`: `flash-lite` → tertiary, `flash` (and not flash-lite) → secondary, `pro` → primary (`GeminiStatusProbe.swift:90-100`); within each bucket the **minimum** `percentLeft` (i.e. most-consumed model) is surfaced.
| Bucket | `ID` | `Label` |
|---|---|---|
| pro | `pro_24h` | Pro |
| flash | `flash_24h` | Flash |
| flash-lite | `flash_lite_24h` | Flash-Lite |

Each: `UsedPercent = 100 - percentLeft` where `percentLeft = remainingFraction*100`, `WindowMinutes = &1440` (fixed 24h, `GeminiStatusProbe.swift:57,65,72`), `ResetsAt = parse(resetTime)` (ISO8601 with fractional-seconds), `ModelScope:[modelId]`. Missing tiers stay absent from `[]Window` rather than synthesized (`GeminiStatusProbe.swift:53`). `Source = SourceOAuth`.

### 3.5 OpenAI Platform (API-key billing — distinct from `codex`)

**Credential.** Env `OPENAI_ADMIN_KEY` tried first, then `OPENAI_API_KEY` (`OpenAIAPISettingsReader.swift:7-17`); optional `OPENAI_PROJECT_ID` scopes requests to one project (query param `project_ids`, `OpenAIAPIUsageFetcher.swift:210-213`). Values are unquoted/trimmed (`.swift:27-40`).

**Requests.** Both paginated, `GET`, 20s timeout, headers `Authorization: Bearer <api_key>`, `Accept: application/json` (`OpenAIAPIUsageFetcher.swift:222-226`):
```
GET https://api.openai.com/v1/organization/costs
    ?start_time=<epoch>&end_time=<epoch>&bucket_width=1d&limit=<chunkDays>&group_by=line_item[&page=<cursor>][&project_ids=<id>]
GET https://api.openai.com/v1/organization/usage/completions
    ?start_time=<epoch>&end_time=<epoch>&bucket_width=1d&limit=<chunkDays>&group_by=model[&page=<cursor>][&project_ids=<id>]
```
(`.swift:127-155,330-348`.) `start_time`/`end_time` chunked into ≤31-day windows over a 30-day trailing history (`maxDailyBucketLimit=31`, `historyDays=30` default); pagination capped at **100 pages** per chunk, a repeated or missing cursor is a hard `parseFailed` (`.swift:40-41,157-207`).

**Response fields consumed** (per daily bucket, per `results[]` entry): costs → `amount.value` (summed into `costUSD`), `line_item` (display name, fallback `"API"`); completions → `input_tokens`, `input_cached_tokens`, `output_tokens`, `input_audio_tokens`, `output_audio_tokens`, `num_model_requests`, `model` (fallback `"Responses and Chat Completions"`) (`.swift:264-303`).

**`[]Window` mapping.** This provider has **no fixed cap** — CodexBar's own snapshot sets `limit: 0` as an explicit sentinel because it is a spend ledger, not a quota (`OpenAIAPIUsageSnapshot.swift:214-219`). `which-model` mirrors that: `Window{ID:"trailing_30d", Unit:UnitUSD, Used:&costUSD (last-30-days sum), Limit:nil, Unlimited:false, WindowMinutes:&43200, ResetHint:"rolling 30-day", UsageKnown:true}` plus a second `Window{ID:"trailing_30d_tokens", Unit:UnitTokens, Used:&totalTokens}` for the token-count view. No `UsedPercent` is ever populated for this provider — there is nothing to divide by. `Source = SourceAPI`.

### 3.6 OpenRouter

**Credential.** Env `OPENROUTER_API_KEY` (`ProviderTokenResolver.openRouterToken`, `OpenRouterProviderDescriptor.swift:40`); base URL overridable via env (`OpenRouterSettingsReader`, validated HTTPS-or-bare-host, `.swift:57-66`).

**Primary request.** `GET https://openrouter.ai/api/v1/credits`. Headers: `Authorization: Bearer <api_key>`, `Accept: application/json`; optional `HTTP-Referer` (env `OPENROUTER_HTTP_REFERER`), `X-Title` (env `OPENROUTER_X_TITLE`, default `"CodexBar"` → `"which-model"` for this port). Timeout 15s (`OpenRouterUsageStats.swift:211-242`).

**Secondary request (best-effort, never blocks primary).** `GET https://openrouter.ai/api/v1/key`, same auth, **race-budgeted to 1.0s** (`rateLimitTimeoutSeconds`, `.swift:211,300-346`) — on timeout or any failure the primary credits snapshot is still returned with `keyDataFetched:false`. This is the same "best-effort secondary, never block primary" pattern documented for Sakana (survey §13); `which-model`'s orchestration layer (§6) should implement it as one reusable helper, not a one-off per provider.

**Response fields consumed.**
```
OpenRouterCreditsResponse{ data: { total_credits: Double, total_usage: Double } }
OpenRouterKeyResponse{ data: { limit, usage, usage_daily, usage_weekly, usage_monthly, rate_limit, quota_status } }  // best-effort
```
(`OpenRouterUsageStats.swift:7-33`.) `balance = max(0, total_credits - total_usage)`; `used_percent = total_usage / total_credits * 100` when `total_credits > 0`.

**`[]Window` mapping.** `Window{ID:"credits_balance", Unit:UnitUSD, Used:&total_usage, Limit:&total_credits, Remaining:&balance, UsedPercent:&used_percent, UsageKnown:true}`; when the secondary `/key` call succeeds, an additional `Window{ID:"key_quota", Unit:UnitUSD, Limit:keyLimit, Used:keyUsage, ResetHint:rateLimitDescription}` (no fixed reset cadence — a spend cap, not a rolling window). `Source = SourceAPI`.

### 3.7 Cursor

**Credential — cookie, primary.** One of `WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `next-auth.session-token`, `wos-session`, `__Secure-wos-session`, `authjs.session-token`, `__Secure-authjs.session-token` (`CursorStatusProbe.swift:26-36`), imported from domains `cursor.com`, `www.cursor.com`, `cursor.sh`, `authenticator.cursor.sh` (`.swift:39-44`) via the browser-cookie library (§4). Fallback: any non-empty cookie set from those domains, validated by a live API call rather than by name (`.swift:111-124`). Local SQLite DB fallback path exists (`resolveDefaultDBPath`) for the desktop app's own session store when no browser cookie is found. `Cookie` header is the semicolon-joined `name=value` pairs (`.swift:55-57`).

**Requests.** All `GET`, headers `Accept: application/json`, `Cookie: <cookieHeader>`, 15s timeout, base `https://cursor.com` (`CursorStatusProbe.swift:966-1001,1470-1531`):
```
GET https://cursor.com/api/usage-summary
GET https://cursor.com/api/auth/me
GET https://cursor.com/api/usage?user=<userId>
```

**Response fields consumed** (`CursorUsageSummary`, survey:227): `billingCycleStart`/`billingCycleEnd`, `membershipType`, `limitType`, `isUnlimited`, `individualUsage.plan{used,limit,remaining,autoPercentUsed,apiPercentUsed}` **in cents**, `individualUsage.onDemand`, `individualUsage.overall`, `teamUsage`.

**`[]Window` mapping.** `Window{ID:"billing_cycle", Unit:UnitUSD, Used:&(plan.used/100.0), Limit:&(plan.limit/100.0), Remaining:&(plan.remaining/100.0), UsedPercent:&plan.autoPercentUsed, Unlimited:isUnlimited, ResetHint:"billing cycle "+billingCycleStart+".."+billingCycleEnd}`; `Window{ID:"on_demand", Unit:UnitUSD, ...}` from `onDemand`; `Window{ID:"team", Unit:UnitUSD, ...}` from `teamUsage` when present. Cents are converted to whole USD before populating `Used`/`Limit`/`Remaining` — the canonical `Window` type has no cents concept, and silently leaving values 100x too large would corrupt every downstream ranking computation. `Source = SourceWeb`.

### 3.8 Antigravity

**Credential.** `~/.codexbar/antigravity/oauth_creds.json` → `~/.which-model/antigravity/oauth_creds.json` for this port (own app-private cache, `AntigravityOAuthCredentialsStore.swift:465-474`), file mode enforced `0600` (`.swift:490-496`). Google OAuth constants: `authURL=https://accounts.google.com/o/oauth2/v2/auth`, `tokenURL=https://oauth2.googleapis.com/token`, `userInfoURL=https://www.googleapis.com/oauth2/v2/userinfo`, scopes `cloud-platform`+`userinfo.email` (`.swift:152-158`).

**Refresh.** `POST https://oauth2.googleapis.com/token` when `expiryDate - now <= 60s` (`refreshSafetyWindow`, `AntigravityRemoteUsageFetcher.swift:41,106-120`).

**Requests.** All `POST https://cloudcode-pa.googleapis.com/v1internal:<method>`, headers `Authorization: Bearer <access_token>`, `Content-Type: application/json`, `User-Agent: antigravity` (`.swift:34-40,384-401`):
```
POST .../v1internal:loadCodeAssist    body {"metadata":{"ideType":"ANTIGRAVITY","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}
POST .../v1internal:fetchAvailableModels    body {"project":"<id>"} or {}
POST .../v1internal:retrieveUserQuota
```

**Response fields consumed.** `RetrieveUserQuotaResponse{buckets: [RetrieveUserQuotaBucket]}`, `RetrieveUserQuotaBucket{modelId, remainingFraction, resetTime}` (`.swift:746-754`) — **identical shape to Gemini's** `QuotaBucket` since both hit the same Cloud Code backend.

**`[]Window` mapping.** Same fraction→percent, ISO8601-reset logic as Gemini (§3.4), but Antigravity does not apply Gemini's Pro/Flash/Flash-Lite substring tiering — each `modelId` bucket becomes its own window: `Window{ID:"model_quota:"+slugify(modelId), ModelScope:[modelId], Unit:UnitPercent, UsedPercent:&(100-remainingFraction*100), ResetsAt:parse(resetTime), UsageKnown:true}`. `Source = SourceOAuth`.

### 3.9 Z.ai

**Credential.** Env `Z_AI_API_KEY` (`ZaiSettingsReader.swift:6`); resolves the quota URL via `Z_AI_QUOTA_URL` (full override) → `Z_AI_API_HOST` (host override) → region default (`.swift:19-25,327-344`).

**Request.** `GET <region-base>/api/monitor/usage/quota/limit`, region base `https://api.z.ai` (global) or `https://open.bigmodel.cn` (China, `ZaiAPIRegion.swift:19-26`). Headers: `Authorization: Bearer <api_key>`, `accept: application/json`; optional team-scope headers when `usageScope == .team` (`ZaiUsageStats.swift:385-389`).

**Response fields consumed.** `ZaiQuotaLimitResponse → ZaiQuotaLimitData → [ZaiLimitRaw]`, decoded into `ZaiLimitEntry{type: ZaiLimitType(TIME_LIMIT|TOKENS_LIMIT), unit: ZaiLimitUnit(days|hours|minutes|weeks), number, usage, currentValue, remaining, percentage, usageDetails, nextResetTime}` (`.swift:6-19,47-80`).

**`[]Window` mapping.** One `Window` per `ZaiLimitEntry`: `ID:"limit:"+lowercase(type)` (`limit:time_limit` / `limit:tokens_limit`), `Unit: UnitPercent` when `type==TIME_LIMIT` else `UnitTokens`, `UsedPercent:&percentage`, `Used:currentValue`, `Limit:number`, `Remaining:remaining`, `WindowMinutes` derived from `unit`×`number` when `unit` is a duration kind (days/hours/minutes/weeks converted to minutes), `ResetsAt:parse(nextResetTime)`. Secondary `GET <region-base>/api/monitor/usage/model-usage` supplies `ZaiHourlyBars` (per-model hourly token chart) — **not** mapped into `[]Window` (no canonical field for time-series chart data); surfaced only through `which-model usage --json --verbose` as informational, non-routing data outside the `Snapshot` contract. `Source = SourceAPI`.

### 3.10 Kimi

**Credential — ordered chain.**
1. Env `KIMI_CODE_API_KEY` — Code API key, used against `api.kimi.com` (`KimiSettingsReader.swift:4`).
2. `~/.kimi-code/credentials/kimi-code.json` (override dir via env `KIMI_CODE_HOME`) — OAuth `{access_token, refresh_token, expires_at}`, freshness gated to `expires_at > now+60s` (`.swift:88-99,176-203`).
3. Env `KIMI_AUTH_TOKEN` / `kimi_auth_token` — web session token for the Connect-RPC fallback (`.swift:11-14`).
4. Browser cookie `kimi-auth` from `www.kimi.com`/`kimi.com` (`KimiCookieImporter.swift:9,22-24`).

**Primary request (Code API).** `GET https://api.kimi.com/coding/v1/usages`. Headers: `Authorization: Bearer <api_key>`, `Accept: application/json`, plus identity headers `User-Agent: CodexBar/<version>` → `which-model/<version>`, `X-Msh-Platform: kimi_code_cli`, `X-Msh-Version`, `X-Msh-Device-Name`, `X-Msh-Device-Model`, `X-Msh-Os-Version`, `X-Msh-Device-Id` (persisted UUID at `~/.kimi-code/device_id`, `0600`, `KimiUsageFetcher.swift:36-37`; `KimiSettingsReader.swift:65-121`).

**Response fields consumed.** `KimiCodeAPIUsageResponse{usage: KimiUsageDetail, limits: [KimiRateLimit]?}`, `KimiUsageDetail{limit, used, remaining, resetTime}` — **all string-typed numerics**, tolerant decode via `stringValue(in:forKey:)` that accepts JSON string, int, or double (`KimiModels.swift:36-107`).

**Web fallback (Connect-RPC).** `POST https://www.kimi.com/apiv2/kimi.gateway.billing.v1.BillingService/GetUsages`, body `{"scope":["FEATURE_CODING"]}`; `POST https://www.kimi.com/apiv2/kimi.gateway.membership.v2.MembershipService/GetSubscriptionStats`, body `{}`. Headers (both calls, `KimiUsageFetcher.swift:234-262`): `Content-Type: application/json`, `Authorization: Bearer <token>`, `Cookie: kimi-auth=<token>`, `Origin: https://www.kimi.com`, `Referer: https://www.kimi.com/code/console`, `Accept: */*`, `Accept-Language: en-US,en;q=0.9`, `User-Agent: <Chrome/143 UA string>`, `connect-protocol-version: 1`, `x-language: en-US`, `x-msh-platform: web`, `r-timezone: <IANA TZ>`; optional `x-msh-device-id`/`x-msh-session-id`/`x-traffic-id` when a session JWT was decodable. Response: `KimiUsage{scope, detail: KimiUsageDetail, limits}` filtered to `scope=="FEATURE_CODING"` (`KimiUsageFetcher.swift:174-179`); `KimiSubscriptionStatsResponse{subscriptionBalance: KimiSubscriptionBalance{feature,type,amountUsedRatio,expireTime}, ratelimitCode7d: KimiSubscriptionRateLimit{ratio,enabled,resetTime}}` (`KimiModels.swift:12-28`).

**`[]Window` mapping.**
| Source | `ID` | notes |
|---|---|---|
| Code API `usage` | `coding_weekly` | `Unit:UnitTokens` (numeric strings parsed), `Used:&used`, `Limit:&limit`, `Remaining:&remaining`, `ResetsAt:parse(resetTime)` |
| Code API `limits[0].detail` when present | `ratelimit_7d` | rate-limit window layered on top of `coding_weekly` |
| web `subscriptionBalance` | `subscription_balance` | `Unit:UnitPercent`, `UsedPercent:&(amountUsedRatio*100)`, `ResetsAt:parse(expireTime)`, `Label:feature` |
| web `ratelimitCode7d` | `ratelimit_7d` (web variant, only used when Code API `limits` absent) | `Unit:UnitPercent`, `UsedPercent:&(ratio*100)`, `ResetsAt:parse(resetTime)`, `Unlimited:!enabled` inverted only if semantics confirm — mark `Warnings:["ratelimitCode7d.enabled semantics unconfirmed against live account"]` until verified |

`Source = SourceAPI` for the Code API path, `SourceWeb` for the Connect-RPC fallback.

## 4. Credential-acquisition capability matrix

Every Go module below was checked against `pkg.go.dev` before being named (`HTTP 200` on `https://pkg.go.dev/<module>`, verified 2026-08-07); none are guessed.

**Every capability in this matrix is reachable only behind the three gates in §1a**: the binary was built without `nousage`, usage is enabled, and the specific provider is explicitly enabled. Two rows are the reason level L2 exists at all — *macOS Keychain generic/internet password* can raise an OS authorisation prompt, and *browser cookie extraction* reads the user's live session material for every site the browser has cookies for. Neither is acceptable as an unavoidable property of installing a model picker, so both must be excludable at compile time, not merely disable-able at runtime.

| Capability | Providers needing it | Go approach | Cross-platform caveats | Security note |
|---|---|---|---|---|
| Plaintext JSON/TOML credential file | `codex`, `claude` (fallback), `gemini`, `antigravity`, `vertexai`, `kimi` (OAuth cache), `kiro` (via CLI, not direct) | stdlib `encoding/json` + `github.com/pelletier/go-toml/v2` (verified) for Codex's `config.toml` — **not** `github.com/BurntSushi/toml` (verified but unmaintained upstream; `go-toml/v2` is the actively-maintained successor) | none — pure Go, no cgo | bounded read (1 MiB cap, mirrors `MAX_CREDENTIAL_BYTES`), `0600`/`0700` permission check on POSIX, warn-only on broader bits (never `chmod`) |
| macOS Keychain generic password | `claude` (`Claude Code-credentials` service) | `github.com/zalando/go-keyring` (verified) | macOS + Windows Credential Manager + Linux Secret Service, but the *service name* lookup semantics are macOS-specific here — gate this AuthSource to `runtime.GOOS=="darwin"` | never log the retrieved token; keyring errors must not leak partial secret material into `Failure.Message` |
| macOS Keychain internet password | `zed` (`kSecClassInternetPassword`, server `zed.dev`) | `go-keyring` has no internet-password support — fall back to `github.com/keybase/go-keychain` (verified, exposes `kSecClassInternetPassword` directly via cgo) | **cgo required**, macOS-only; `zed` AuthSource is a no-op on Linux/Windows | same as above |
| Browser cookie extraction (Chromium AES-GCM, Safari binarycookies, Firefox) | `cursor`, `windsurf`, `devin`, `augment`, `factory`, `minimax`, `alibaba`, `alibabatokenplan`, `qwencloud`, `amp`, `t3chat`, `mimo`, `abacus`, `mistral`, `perplexity`, `commandcode`, `longcat`, `opencode`, `opencodego`, `kimi` (fallback) | `github.com/browserutils/kooky` (verified; MIT; `browser/chrome`, `browser/safari`, `browser/firefox` subpackages handle each engine's own decryption — Chromium's AES-GCM key comes from the OS Keychain/libsecret/DPAPI internally, Safari's `.binarycookies` binary format is parsed directly, Firefox's SQLite `cookies.sqlite` is read via a pure-Go SQLite reader) | kooky's own README: "basic functionality works on Windows, macOS, Linux; some functions might not yet be implemented on some platforms," API explicitly marked unstable — pin the exact module version and add a golden-file regression test per browser engine before relying on it in CI | reading another application's cookie jar is inherently a trust boundary crossing; `which-model` MUST require explicit per-provider opt-in (`which-model auth login <provider> --allow-browser-cookies`) rather than probing browsers by default, and MUST scope `kooky` queries to the exact provider domain list (never a wildcard cookie dump) |
| Env var | all 66 (as override tier) | stdlib `os.LookupEnv` | none | trim + strip matching quotes (mirrors CodexBar's `cleaned()`), reject empty-after-trim |
| CLI shell-out | `copilot` (`git config`, `gh auth token`), `vertexai` (`gcloud auth application-default print-access-token`), `kiro` (`kiro-cli`), `doubao` (`arkcli`, optional) | stdlib `os/exec` with `exec.CommandContext` for hard timeouts | `gh`/`gcloud`/provider CLIs may be absent — every shell-out AuthSource MUST degrade to "candidate unavailable," never a hard error, matching prototype 1's `defaultCommandRunner` swallow-all-failures contract (`usage-allowance-checks-spec.md:194`) | bounded output (32 KiB cap, mirrors prototype), fixed timeout (3s for Copilot chain per spec, 20s for `gcloud`), never interpolate raw candidate output back into a shell command |
| Subprocess JSON-RPC | `codex` (RPC transport) | stdlib `os/exec` + `bufio.Scanner` over newline-delimited JSON-RPC 2.0; no external RPC framework needed since the protocol is trivial (one request/response per line) | process spawn must set an enriched `PATH` (survey:115) to find `codex` on non-standard installs | init 8s / call 3s timeouts, hard-kill the process tree on timeout, never on a wall-clock miss leave a zombie subprocess |
| OAuth device flow | `copilot` | stdlib `net/http` + `golang.org/x/oauth2` (verified) for the token exchange leg; the device-code polling loop itself is protocol-trivial and implemented by hand per the exact state machine in §3.3 (x/oauth2's built-in `DeviceAuth` flow does not expose the local-deadline-before-and-after-sleep nuance prototype 1 requires, so the polling loop is hand-rolled against `golang.org/x/oauth2/clientcredentials`-adjacent primitives, not the packaged helper) | none | device code + token never touch disk; only validated `verification_uri`/`user_code` ever reach stdout |
| OAuth refresh grant | `codex`, `claude`, `gemini`, `antigravity`, `vertexai` (user ADC shape) | `golang.org/x/oauth2` (verified) `oauth2.Config.TokenSource` with a custom `TokenRefresher` per provider's token endpoint/client-id | none | refresh tokens live only in the credential file/keychain, never logged; `which-model` mirrors prototype 1's expiry-heuristic (seconds-vs-ms via the `>10_000_000_000` threshold, `usage-allowance-checks-spec.md:79`) in Go as a shared helper |
| AWS SigV4 | `bedrock` | `github.com/aws/aws-sdk-go-v2/aws/signer/v4` (verified) — or the full-service clients `github.com/aws/aws-sdk-go-v2/service/cloudwatch` and `.../service/costexplorer` (both verified) if the annex's "avoid needless allocation/dependency weight" principle is relaxed for this one Tier-2 provider; recommend the full service clients since Bedrock's CloudWatch pagination (20-page cap, 4 MiB response cap, survey:349) is exactly what the SDK's built-in paginators already implement correctly | credentials resolved via the SDK's own default chain (env → shared config → SSO → EC2/ECS role) — matches survey's "falls back to AWS CLI profile resolution" note | never widen the SDK's default credential chain beyond what the descriptor declares; region MUST come from `AWS_REGION`/`AWS_DEFAULT_REGION`/`AWS_PROFILE`, never hardcoded |
| Volcengine AK/SK | `doubao` | **no verified idiomatic Go SigV4-style module for Volcengine's "Top OpenAPI" signing scheme was found on pkg.go.dev** — flag unverified. Fallback: hand-roll the signer (HMAC-SHA256 canonical-request signing, same family as AWS SigV4 but with Volcengine's own header/query-param names) as a ~150-line internal package (`internal/usage/provider/doubao/sign.go`), unit-tested against a fixed request/signature golden vector from Volcengine's public docs | n/a (pure Go crypto, stdlib `crypto/hmac`+`crypto/sha256`) | the signer touches the secret key on every request; keep it allocation-light and never format the secret into an error string |
| Protobuf / gRPC-Web / Connect-RPC decode | `windsurf` (raw protobuf over `_backend`), `kimi` (Connect-RPC), `manus` (Connect-RPC), `grok` (gRPC-Web+proto) | `google.golang.org/protobuf` (verified) for wire decoding; `connectrpc.com/connect` (verified, first-party Connect-RPC client — handles Kimi/Manus's `POST .../<Service>/<Method>` framing with `connect-protocol-version` header natively) for Kimi/Manus; `github.com/improbable-eng/grpc-web` (verified) client transport for Grok's gRPC-Web+proto path. Windsurf's endpoint uses a bespoke protobuf schema CodexBar decoded by hand (`WindsurfPlanStatusProtoCodec`, survey:228) with no `.proto` file available — `which-model` must hand-roll the same tag-by-tag reader against `google.golang.org/protobuf/encoding/protowire` rather than fabricate a schema | none beyond the usual "private wire format can change without notice" — this is exactly why §8's drift-resistance policy exists | never trust field numbers/wire types without bounds-checking; malformed/unknown protobuf MUST decode to `unsupported_response`, not panic |
| HTML/JS scrape | `ollama` (cloud settings page), `t3chat` (tRPC blob) | stdlib `net/http` + `golang.org/x/net/html` (verified, standard extended-stdlib package) for any HTML fragment parsing needed; prefer regex/JSON-substring extraction over full DOM parsing where the target is a JSON blob embedded in a `<script>` tag | brittle by construction — every scrape-based provider is Tier 3 in this annex specifically because of this | scraped HTML MUST be size-bounded identically to any other HTTP response (256 KiB cap, §9 inherited invariant) before any parsing is attempted |

## 5. `Descriptor` + `AuthSource` + `WindowSpec` Go declarations

`Snapshot`, `Window`, `Unit`, and `Failure` are exactly as given in the [master plan §3.2](./README.md) and are not repeated here. This section defines the remaining types those canonical types reference.

```go
package usage

import (
	"context"
	"net/http"
	"time"
)

// Kind classifies how a provider's usage relates to billing.
type Kind int

const (
	KindSubscription Kind = iota // fixed-plan rate-limit windows (Codex, Claude, Copilot, ...)
	KindAPIKeyBilling             // pay-as-you-go spend/credit tracking against an API key (OpenAI Platform, Mistral, ...)
	KindGateway                   // aggregator/proxy that routes to many upstream models (OpenRouter, LiteLLM, ...)
	KindLocalTool                 // no usage network call exists; presence/validity only (JetBrains, Ollama, ...)
)

func (k Kind) String() string {
	switch k {
	case KindSubscription:
		return "subscription"
	case KindAPIKeyBilling:
		return "api_key_billing"
	case KindGateway:
		return "gateway"
	case KindLocalTool:
		return "local_tool"
	default:
		return "unknown"
	}
}

// Source records which transport actually produced a Snapshot.
type Source string

const (
	SourceOAuth Source = "oauth"
	SourceAPI   Source = "api"   // static/long-lived API key
	SourceCLI   Source = "cli"   // subprocess (RPC or shell-out)
	SourceWeb   Source = "web"   // browser-cookie-authenticated
	SourceLocal Source = "local" // local-only probe, no network usage call
	SourceCache Source = "cache" // served from the on-disk cache, not freshly fetched
)

// WindowSpec is descriptor-time metadata: which window IDs/labels/units a
// provider MAY report. It is NOT a runtime reading — that's Window (contract).
type WindowSpec struct {
	ID       string
	Label    string
	Unit     Unit
	Optional bool // provider may omit this window depending on plan/quota shape
}

// AuthKind discriminates AuthSource's populated fields. Exactly one of the
// kind-tagged sub-specs on an AuthSource is non-nil for a given Kind value.
type AuthKind int

const (
	AuthEnvVar AuthKind = iota
	AuthFile
	AuthKeychainGeneric
	AuthKeychainInternet
	AuthBrowserCookie
	AuthCLIShellOut
	AuthSubprocessRPC
	AuthOAuthDeviceFlow
	AuthOAuthRefreshGrant
	AuthAWSSigV4
	AuthVolcengineAKSK
	AuthGRPCWebToken // Connect-RPC / gRPC-Web / raw-protobuf token carriers
)

// Credential is the resolved secret handed to a FetchFunc. It never
// round-trips through logs or Failure.Message.
type Credential struct {
	Token  string            // opaque bearer/API token, already assertOpaque-validated
	Extra  map[string]string // secondary fields: account_id, project_id, cookie header, ...
	Source AuthKind          // which AuthSource entry in the chain produced it
	Mode   uint32            // credential file POSIX mode, when AuthKind == AuthFile (0 otherwise)
}

// KeychainSpec, CookieSpec, ShellSpec, RPCSpec, OAuthSpec are the per-Kind
// payloads an AuthSource carries. Only the field matching AuthSource.Kind is read.
type KeychainSpec struct {
	Service string // e.g. "Claude Code-credentials"
	Account string // "" = match any account for the service
	Server  string // internet-password only, e.g. "zed.dev"
}

type CookieSpec struct {
	Domains     []string // exact cookie-jar domains to query, never a wildcard
	CookieNames []string // strict-name pass first; empty = accept any non-empty set for Domains
}

type ShellSpec struct {
	Command string
	Args    []string
	Timeout time.Duration
}

type RPCSpec struct {
	Command     string
	Args        []string
	InitTimeout time.Duration
	CallTimeout time.Duration
	Method      string // JSON-RPC method invoked after initialize, e.g. "account/rateLimits/read"
}

type OAuthSpec struct {
	ClientID      string
	ClientSecret  string // empty for public clients (Codex, Claude, Copilot)
	DeviceCodeURL string // device flow only
	TokenURL      string
	Scope         string
}

// AuthSource is one ordered link in a provider's credential-resolution chain.
// Descriptor.Auth is walked in order; the first entry that both resolves a
// candidate AND (when Validate is set) passes validation wins.
type AuthSource struct {
	Kind AuthKind

	EnvVar    string   // AuthEnvVar
	FilePaths []string // AuthFile: ordered candidate paths, first existing+valid wins
	JSONPath  string   // AuthFile: dotted path to the token field, e.g. "tokens.access_token"

	Keychain *KeychainSpec // AuthKeychainGeneric / AuthKeychainInternet
	Cookie   *CookieSpec   // AuthBrowserCookie
	Shell    *ShellSpec    // AuthCLIShellOut
	RPC      *RPCSpec      // AuthSubprocessRPC
	OAuth    *OAuthSpec    // AuthOAuthDeviceFlow / AuthOAuthRefreshGrant

	// Validate gates candidate acceptance (e.g. Copilot's mandatory GET /user
	// identity check, §3.3). A candidate that fails Validate is skipped, not fatal.
	Validate func(ctx context.Context, candidate Credential, client *http.Client) error
}

// FetchFunc performs one provider's usage fetch given a resolved credential.
type FetchFunc func(ctx context.Context, cred Credential, client *http.Client) (Snapshot, error)

// Descriptor is exactly the contract type; repeated here for the fields this
// section defines the meaning of.
type Descriptor struct {
	ID          string
	DisplayName string
	Kind        Kind
	Tier        int // 1 = port first, 2 = second wave, 3 = deferred
	Auth        []AuthSource
	Windows     []WindowSpec
	Timeout     time.Duration
	CacheTTL    time.Duration
	Fetch       FetchFunc
}
```

**Registry and self-registration pattern** (`internal/usage/registry.go`), modeled on `database/sql`'s driver registry — compile-time-enforced, no reflection, no dynamic plugin loading:

```go
package usage

import "fmt"

type registry struct {
	descs map[string]Descriptor
	order []string // registration order, preserved for deterministic `which-model usage` iteration
}

var defaultRegistry = &registry{descs: make(map[string]Descriptor)}

// Register is called from each provider/<id> package's init(). Panics on a
// duplicate ID — a programming error caught at binary-startup time, never
// something a running command can trigger.
func Register(d Descriptor) {
	if _, dup := defaultRegistry.descs[d.ID]; dup {
		panic(fmt.Sprintf("usage: duplicate provider id %q", d.ID))
	}
	defaultRegistry.descs[d.ID] = d
	defaultRegistry.order = append(defaultRegistry.order, d.ID)
}

func All() []Descriptor {
	out := make([]Descriptor, 0, len(defaultRegistry.order))
	for _, id := range defaultRegistry.order {
		out = append(out, defaultRegistry.descs[id])
	}
	return out
}

func Lookup(id string) (Descriptor, bool) {
	d, ok := defaultRegistry.descs[id]
	return d, ok
}
```

Each `internal/usage/provider/<id>/<id>.go` self-registers in `init()`:

```go
package codex

import (
	"time"

	"github.com/wdmitchell-uk/which-model/internal/usage"
)

func init() {
	usage.Register(usage.Descriptor{
		ID:          "codex",
		DisplayName: "Codex / ChatGPT",
		Kind:        usage.KindSubscription,
		Tier:        1,
		Auth: []usage.AuthSource{
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "tokens.access_token"},
		},
		Windows: []usage.WindowSpec{
			{ID: "5h", Label: "5h", Unit: usage.UnitPercent},
			{ID: "weekly", Label: "Weekly", Unit: usage.UnitPercent},
			{ID: "credits", Label: "Credits", Unit: usage.UnitCredits, Optional: true},
		},
		Timeout:  15 * time.Second,
		CacheTTL: 300 * time.Second,
		Fetch:    fetch,
	})
}
```

`cmd/which-model/main.go` blank-imports every provider package, which is the ONLY place the full provider list is enumerated for linking purposes:

```go
import (
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/antigravity"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/claude"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/codex"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/copilot"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/gemini"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/kimi"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/openai"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/openrouter"
	_ "github.com/wdmitchell-uk/which-model/internal/usage/provider/zai"
	// ... one line per remaining provider/<id> package, alphabetical, no exceptions
)
```

This mirrors `image/jpeg`/`image/png` and `database/sql/driver`: a provider that isn't imported doesn't exist in the binary, there is no runtime discovery step to get wrong, and `go build` fails loudly if a package fails to compile rather than silently shipping a half-working provider.

## 6. Fetch orchestration

**Concurrency.** `which-model usage` (all providers) and `which-model pick` (routing-relevant providers) fan out via `golang.org/x/sync/errgroup` (verified) with `errgroup.SetLimit(n)` bounding parallelism (default `n = min(len(providers), 16)`, overridable via `--max-parallel`). Each provider fetch runs under `context.WithTimeout(ctx, descriptor.Timeout)` — the errgroup's shared context is **not** what bounds an individual provider; a slow provider must not delay or cancel its siblings. A canceled/timed-out fetch produces a `Snapshot{Failure: &Failure{Code:"timeout", ...}}`, not an error returned from the `errgroup.Group.Wait()` call — **partial failure MUST NOT fail the batch** (contract requirement, restated): one provider erroring is data, not a fatal condition, exactly like prototype 1 treats every failure mode as a caught, reported `UsageError` rather than a process crash (`research/usage-allowance-checks-spec.md` §9 first bullet's spirit, generalized from 1 provider to N).

**Per-provider `CacheTTL` defaults**, drawn from survey §13 (`codexbar-provider-survey.md:345-350`):
| Provider | `CacheTTL` | Justification |
|---|---|---|
| `claude` | 1800s (credential cache) / 60s (usage re-fetch floor) | matches CodexBar's in-memory OAuth credential cache lifetime and keychain-fingerprint re-check interval (survey:346) |
| `codex` (RPC path) | no cache — always live within the 3s call timeout | subprocess calls are cheap and local; caching the RPC result adds staleness with no cost benefit |
| `openrouter`/`sakana` secondary calls | not cached — race-budgeted (1.0s / 200ms) as a best-effort enrichment, re-attempted every fetch | these are optional enrichments layered onto a primary result, not the primary quota signal (survey:258,350) |
| all HTTP-fetched providers, default | 60s | matches the 15-30s per-request HTTP timeout pattern (survey:348) times a small safety multiplier — frequent enough that `which-model pick` sees fresh-enough data, infrequent enough that a tight loop of `which-model usage` calls doesn't hammer private endpoints |
| any provider that returned `rate_limited` | governed by the response's `Retry-After` header when present, else `CacheTTL * 4` | mirrors Claude's `ClaudeOAuthUsageRateLimitGate` honoring `Retry-After` (survey:144,346) — a provider that just 429'd must not be re-hit on the next 60s tick |

**Cache file layout.** `~/.cache/which-model/usage/<provider-id>.json` (XDG) / `~/Library/Caches/which-model/usage/<provider-id>.json` (macOS), one file per provider, written atomically (temp file + `rename()`, mirroring prototype 1's credential-write pattern) containing `{snapshot: Snapshot, fetched_at: time.Time}`. `--refresh` bypasses the cache read (always fetches live, still writes the new result back). `--offline` never fetches — returns the cached `Snapshot` with `Stale:true` when `fetched_at` exceeds `CacheTTL`, or `Failure{Code:"fallback_unavailable"}` when no cache file exists yet. `--max-age <duration>` overrides `CacheTTL` for that invocation only (does not persist), letting `which-model pick --max-age 10s` demand fresher-than-default data for a routing decision while `which-model usage` keeps the relaxed default.

**Disabled short-circuit.** With usage off at any level from §1a, **no fetch is scheduled at all** — the `errgroup` is never constructed, the usage cache is neither read nor written, and existing cache files are left intact rather than invalidated (re-enabling usage should not have thrown away good data). Critically, the disabled check MUST short-circuit **before** credential resolution, not after: resolving a Claude AuthSource is what raises a macOS Keychain authorisation prompt, so a `--no-usage` run that resolved credentials and then discarded them would still prompt the user, which is precisely the behaviour the toggle exists to prevent. Registry membership is compile-time, enablement is config-time, and **both** gates MUST pass before any resolver function is entered.

## 7. Error taxonomy

`Failure.Code` values are reused **verbatim** from prototype 1 (`usage-allowance-checks-spec.md:16`) wherever the same failure mode applies to the Go port; new codes are added only where the 66-provider surface introduces a failure shape prototype 1 never had to handle (multi-provider batching, cache staleness, OAuth device-flow specifics already named, protobuf/RPC decode).

| Code | Meaning | Exit code |
|---|---|---|
| `unauthorized` | provider rejected the credential (401/403) | 5 |
| `rate_limited` | provider rate-limited the request (429) | 1 |
| `provider_status` | non-2xx status not covered by a more specific code | 1 |
| `expired_credential` | locally-known expiry timestamp is in the past | 5 |
| `unsupported_response` | response parsed as JSON/protobuf but didn't match the expected shape (§8 drift policy) | 1 |
| `login_required` | no usable credential found and no `--login`/interactive flow was requested | 5 |
| `endpoint_refused` | outbound URL failed the exact-allow-list check | 1 |
| `untrusted_origin` | a configured fallback origin wasn't explicitly trusted via a per-invocation flag | 1 |
| `redirect_refused` | server attempted a 3xx redirect (never followed) | 1 |
| `response_too_large` | response body exceeded the 256 KiB cap | 1 |
| `timeout` | request or subprocess call exceeded its bounded timeout | 1 |
| `network` | transport-level failure (DNS, TCP, TLS) | 1 |
| `response_json` | body was empty or failed to parse as the expected JSON shape | 1 |
| `credential_file` | credential file missing, unreadable, or oversized | 5 |
| `credential_json` | credential file was not valid JSON / not an object | 5 |
| `unsafe_credential` | a token/identifier failed `assertOpaque`/`assertIdentifier` shape checks | 5 |
| `access_denied` | user explicitly denied/cancelled an OAuth device-flow grant | 5 |
| `device_expired` | OAuth device-flow deadline passed before authorization completed | 5 |
| `fallback_unavailable` | a fallback path was needed but no fallback target was configured/cached | 1 |
| `keychain_unavailable` | *(new)* macOS Keychain API returned an error other than "not found" (locked, denied) | 1 |
| `cookie_unavailable` | *(new)* browser-cookie extraction failed for every configured browser/domain (kooky error, profile locked, Full Disk Access missing on macOS Safari) | 5 |
| `signing_failed` | *(new)* AWS SigV4 or Volcengine AK/SK request signing failed (bad/missing secret key shape) | 5 |
| `rpc_protocol` | *(new)* subprocess JSON-RPC or Connect-RPC/gRPC-Web framing was malformed (not a shape-drift issue — a transport-level protocol violation) | 1 |
| `cache_stale` | *(new)* `--offline` was requested and the cache exceeded `CacheTTL` (non-fatal marker, sets `Snapshot.Stale`, not necessarily a hard failure — surfaced as `Failure` only when no cache exists at all, see `fallback_unavailable`) | 0 (informational, not an error path) |
| `partial_batch_failure` | *(new, batch-level only, never on a single `Snapshot`)* one or more providers in a `which-model usage`/`which-model pick` batch failed; the batch itself still exits 0 unless `pick`'s stricter rules (exit 3/4, Annex D) apply | 0 |
| `usage_disabled` | *(new)* usage is off by `--no-usage` or `[usage] enabled = false`; `which-model usage`/`which-model auth` refuse and name the disabling source (§1a) | 2 |
| `usage_compiled_out` | *(new)* binary was built with `-tags nousage`; sentinel `ErrUsageCompiledOut`. Not retryable and not fixable at runtime — requires a different binary | 2 |

## 8. Drift-resistance policy

These are private, undocumented, unversioned endpoints reverse-engineered from a Swift client's binary/source and a set of hand-written Node scripts — every one of them can change shape without notice, and the survey documents several that already have (§14: StepFun's dual billing-model detection, LongCat's explicitly-unstable field names, Claude's synthetic-placeholder quirk, Perplexity's ambiguous credit-field precedence, Doubao's mutually-exclusive plan shapes, Codex's snake/camel key churn). `which-model` treats every Tier-1/Tier-2 fetch as adversarial input, not as a stable contract:

- **Golden-file response fixtures under `testdata/usage/<provider-id>/`.** One fixture per known response shape per provider (e.g. `codex/primary_5h_weekly.json`, `codex/fallback_404_trusted_origin.json`, `claude/synthetic_placeholder.json`, `stepfun/legacy_coding_plan.json`, `stepfun/token_plan.json`) — captured from real (credential-redacted) responses where available, else hand-constructed from the survey's documented field names. Every `Fetch` implementation is unit-tested exclusively against these fixtures (an injected `http.RoundTripper`, mirroring prototype 1's `memoryFs`/mocked-`fetch` test strategy, `usage-allowance-checks-spec.md:321`), never against the live endpoint in CI.
- **Tolerant snake/camel key decoding.** Every response struct that CodexBar itself decodes tolerantly (Codex's `RPCRateLimitSnapshot`, Claude's multi-key-name `sevenDayRoutines`, Kimi's string-or-number `KimiUsageDetail`) is ported with the same tolerance, not narrowed — narrowing "for cleanliness" during the port is exactly how a real API's casing/typing churn turns into a silent parse failure in production six months later.
- **`unsupported_response` on shape drift, never a runtime-weakened validation.** When a response decodes as valid JSON/protobuf but a required field is absent or fails a `finitePercent`/`finiteNonNegative`-style bounds check, the fetch fails closed with `unsupported_response` (§7) rather than substituting a zero, `nil`, or best-guess value into a `Window` that `internal/pick/` will then rank against real numbers. A ranking engine silently treating "we don't know" as "0% used" is worse than an explicit, visible failure.
- **Per-provider `LastVerified` date, surfaced by `which-model usage --json`.** Each `Descriptor` carries a `LastVerified time.Time` (the date this annex's spec, or a subsequent maintainer's re-verification, was last checked against a live account) — the `usage --json` output includes it per-provider so an agent consuming `which-model`'s JSON contract can distrust an adapter that hasn't been re-verified recently (e.g. flag anything `> 180d` stale in its own reasoning) without `which-model` itself needing to guess whether an upstream API silently changed. This directly operationalizes the survey's fragility notes (§14) into a machine-readable trust signal instead of leaving them as static documentation nobody re-reads.
