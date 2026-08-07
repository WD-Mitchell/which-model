---
kind: feature-spec
version: "1.0"
feature: F25-cmd-auth
project: which-model
---

# F25 — cmd-auth: SPEC

## 1. Purpose

`which-model auth status|login|logout` manages the credential lifecycle of usage providers. `status` reports per-provider credential presence, source, expiry, and a redacted fingerprint — never the token itself. `login` runs only interactive flows, refusing unattended invocation outright, and explicitly defers browser-handoff login to a later milestone. `logout` removes only credential material `which-model` itself wrote, with explicit confirmation, and never auto-remediates file permissions. The command is the agent-facing credential surface (annex-c §4.7 exit 5 routes agents to "the explicit, user-present --login flow"), so refusal behaviour and redaction are security invariants, not conveniences.

## 2. Behaviour

1. **Command shape.** `which-model auth status [provider...]`, `which-model auth login <provider>`, `which-model auth logout <provider> [--yes]`. All provider arguments are validated against the F11 registry (unknown id → exit 2). `status` with no positional args reports every provider with `enabled = true` in config (same expansion as `usage --all`). (Source: `docs/plan/annex-d-cli-reference.md` §2.2; Decisions D-2.)

2. **Wiring.** Self-registered per the F22 command-wiring contract: `pkg/whichmodel/auth_cmd.go` carries `func init() { register(NewAuthCmd) }` and `func NewAuthCmd() *cobra.Command` with the three subcommands attached inside that one file. The file is build-tagged `//go:build !nousage` (a `nousage` binary does not register `auth` at all, annex-d §4.6 L2); the logic file `pkg/whichmodel/auth.go` compiles under both tags via F21 stubs. (Source: F22 command-wiring contract; annex-d §4.6.)

3. **Status resolution.** For each queried provider, F25 calls the F12 credential resolver chain (F25 CONTRACTS §8.2). A credential that resolves is `ok`; a credential whose `ExpiresAt` is in the past is `expired` (not usable — exit-5 class, matching `Failure.Code = expired_credential`); no resolvable credential is `missing`. The reported `source` is the `usage.Source` of the resolver that produced the credential. (Source: annex-d §2.2; global CONTRACTS §1.6; Decisions D-3.)

4. **Redaction.** The token is never rendered under any flag. Instead, `status` renders a fingerprint: `sha256(secret)` hex digest, displayed as `first6…last4` (e.g. `a1b2c3…9f0e`). The fingerprint is shown whenever a credential exists. Account/login identity is shown only with the global `--show-identity` flag. Every output path is canary-tested (global SPEC §6.5, §6.7). (Source: annex-d §2.2; Decisions D-1.)

5. **Status text output.** One line per provider, columns separated by two spaces (Go `text/tabwriter`, `padding 2`): `<provider>  <status>  <source|->  <fingerprint|->  <expiry|->`. Expiry renders `(expires <RFC3339>)` for future expiry and `(expired <RFC3339>)` for past; missing providers render `-` for source, fingerprint, and expiry and a hint `run: which-model auth login <provider>`. With `--show-identity`, append `(account <login>)`. (Source: annex-d §2.2 example layout; Decisions D-7.)

6. **Status JSON output.** A single JSON document: root `{schema_version: "2.0", usage_enabled: true, providers: [...]}` per F25 CONTRACTS §6. Entries: `{provider, status, source|null, expires_at|null, fingerprint|null, account?}` (`account` present only with `--show-identity`; `expires_at`/`fingerprint`/`source` are `null` when `status == "missing"`). (Source: annex-d §2.2; global CONTRACTS §6; Decisions D-8.)

7. **Login: interactive only.** `auth login` refuses to start any flow when stdin is not a TTY or `WHICH_MODEL_NONINTERACTIVE=1` is set: exit 2 with message `auth login: [arguments] refusing unattended login; run from an interactive terminal`. Detection uses `golang.org/x/term.IsTerminal(int(os.Stdin.Fd()))`. (Source: annex-d §2.2 unattended-refusal rule.)

8. **Login: supported flows.** Only providers whose sole interactive flow is a device-code flow (Copilot) are supported: F25 calls F12's device-flow API and prints exactly two validated fields to stdout as the primary output — `Open <verification_uri> and enter code <code>.` — with progress (`waiting for confirmation...`) on stderr. The device code and any token never appear anywhere else. (Source: annex-d §2.2; research spec §2.3; Decisions D-4.)

9. **Login: deferred flows.** For every other provider — including all browser-cookie/web-handoff providers — `auth login` exits 2 with `[unsupported] login for <provider> is not supported until M5; sign in with the provider's own client, then run which-model auth status <provider>`. This is a hard refusal, not a hint-and-hope; the flow is never attempted. (Source: annex-d §2.2; Decisions D-4.)

10. **Logout: confirmation.** `auth logout <provider>` removes credential material `which-model` itself wrote (F12 `Remove`); provider-native stores (`~/.claude/credentials.json`, `~/.codex/auth.json`, keychain items the provider's own tools created, `git config`) are never touched. Confirmation: with `--yes`, no prompt; on a TTY without `--yes`, prompt `Remove which-model's cached credential for <provider>? [y/N]` (anything but `y`/`yes` → print `aborted` to stderr, exit 0, nothing removed); non-TTY without `--yes` → exit 2 `[arguments] refusing unattended logout without --yes`. If nothing removable: print `no which-model-managed credential for <provider>; nothing to remove` to stderr and exit 0 (idempotent). (Source: annex-d §2.2; Decisions D-5, D-6.)

11. **Logout: permissions.** If the credential material F12 is about to remove has broad permissions (`security.HasBroadPermissions`), F25 emits exactly one stderr warning `Warning: <path> permissions are broader than 0600; review them.` before removal — and never chmods, chowns, or otherwise remediates (global SPEC §6.6: permission warnings, never auto-remediation). (Source: research spec core.mjs:249-263; global SPEC §6.6.)

12. **Exit codes.** `auth status`: 0 when every queried provider is `ok`; 5 when at least one queried provider is `missing` or `expired` (no usable credential); 2 for argument errors (unknown provider, bad flags) and usage-disabled refusal. `auth login`: 0 on successful device-flow completion (or prompt handed off); 2 for unattended refusal, unsupported provider, unknown provider, invalid flags. `auth logout`: 0 on success/abort/nothing-to-remove; 2 for unknown provider, missing required args, unattended refusal without `--yes`; 1 on runtime failure (e.g. F12 removal error). (Source: annex-d §2.2; Decisions D-9.)

13. **Usage-disabled behaviour.** Under L0 (`--no-usage`) or L1 (`[usage] enabled = false`), every `auth` subcommand exits 2 with `which-model auth <sub>: [usage_disabled] usage is disabled by <source>`. F25 resolves this inline via F01 (`UnmarshalKey("usage.enabled")` + the global flag), same decision as F24 D-5. Under L2 the command is not registered (behaviour 2). (Source: annex-d §4.6.)

14. **stdout/stderr discipline.** stdout carries only the status report / login prompt / logout confirmation prompt; stderr carries progress, warnings, and the final failure line `which-model auth <sub>: [<code>] <message>` (F22 via F03 `output.WriteFailure`). On any nonzero exit stdout is empty EXCEPT: (a) the login prompt cases (the prompt is primary output even when the flow later fails), and (b) `status` exit 5, where the per-provider report remains the primary deliverable and is kept on stdout via F22's `ReportedError` marker (the `--json` error document is suppressed for it). Exit signalling uses the F22 exit contract (F25 CONTRACTS §8.1). (Source: annex-d §1.3; F22 `ReportedError`; Decisions D-10.)

## 3. Error behaviour

| Condition | Exit | stderr |
|---|---|---|
| Unknown provider id (any subcommand) | 2 | `[arguments] unknown provider "x"; valid providers: <ids>` |
| `status`: ≥1 provider `missing` or `expired` | 5 | (report stays on stdout via `ReportedError`; F22 renders the failure line `[login_required|expired_credential] provider(s) without usable credentials; run which-model auth status`) |
| `login` with non-TTY stdin or `WHICH_MODEL_NONINTERACTIVE=1` | 2 | `[arguments] refusing unattended login; run from an interactive terminal` |
| `login` for non-device-flow provider | 2 | `[unsupported] login for <provider> is not supported until M5; sign in with the provider's own client, then run which-model auth status <provider>` |
| `logout` non-TTY without `--yes` | 2 | `[arguments] refusing unattended logout without --yes` |
| `logout` prompt declined | 0 | `aborted` |
| `logout` nothing removable | 0 | `no which-model-managed credential for <provider>; nothing to remove` |
| Usage disabled (L0/L1) | 2 | `[usage_disabled] usage is disabled by <source>` |
| `logout` removal runtime error | 1 | `[runtime] <message>` |
| Broad credential permissions (logout) | 0 (warn only) | `Warning: <path> permissions are broader than 0600; review them.` |

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| D-1 | Fingerprint = `sha256(secret)` hex digest displayed as `first6…last4` (e.g. `a1b2c3…9f0e`) | The assignment's decided format; 10 of 64 hex chars reveal nothing usable while giving a stable identity check across invocations |
| D-2 | `auth status` with no providers reports every enabled provider (config `[providers.<id>].enabled`) | Mirrors `usage --all` expansion so agents can poll all configured providers with one call (quota-guard hook pattern) |
| D-3 | A credential with `ExpiresAt` in the past is `status: "expired"` and counts as not-usable (exit 5) | Matches canonical `expired_credential` → exit 5 mapping (global CONTRACTS §1.6); "expired" is more actionable than "missing" |
| D-4 | `login` supports only device-flow providers (Copilot via F12); every other provider — including cookie/browser-handoff providers — gets the explicit `not supported until M5` refusal (exit 2) | The assignment requires an explicit deferred message for cookie providers; a uniform capability rule is simpler and honest than per-provider partial flows |
| D-5 | Non-TTY `logout` without `--yes` is a hard exit 2 (never a hang, never silent removal) | Same rationale as the login unattended-refusal rule: agent contexts must fail fast, not stall |
| D-6 | `logout` with nothing removable exits 0 with a message | Idempotent logout is safer for scripts than a spurious error |
| D-7 | Status text columns via `text/tabwriter` (padding 2), order provider/status/source/fingerprint/expiry; missing → `-` placeholders plus a `run:` hint | Fixed, golden-testable layout; the `run:` hint carries the annex-d example's actionable guidance |
| D-8 | Status JSON wraps entries in `providers: [...]` under the F22 envelope | Global CONTRACTS §6 requires the envelope on every `--json` output; annex-d §2.2's bare array predates the envelope |
| D-9 | `status` exit 5 fires when ≥1 queried provider is not usable (not only "all") | annex-d §2.2: "one or more queried providers have no usable credential"; agents must notice a missing provider even when others are fine |
| D-10 | Logout's confirmation prompt goes to stdout (it is primary interactive output); refusal/failure lines go to stderr | annex-d §1.3 stdout/stderr discipline: primary output on stdout, diagnostics on stderr |

## 5. Out of scope

- `auth status --login` (inline login attempt) and `auth login --trust-configured-origin <origin>` (annex-d §2.2 flags not in this feature's contract; the origin-trust validation belongs with the F16/F17 provider work).
- Any actual device-flow HTTP or OAuth logic: F12 owns device flow; F25 only orchestrates interactivity and output.
- Browser-cookie extraction and browser-handoff login (deferred to M5 by Decision D-4).
- Credential file/keychain plumbing: F12 owns resolvers and removal; F25 consumes them.
- Provider-native credential stores: F25 never reads or writes them.
