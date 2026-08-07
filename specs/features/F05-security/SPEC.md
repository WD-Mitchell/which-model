---
kind: feature-spec
feature: F05-security
version: "1.0"
project: which-model
---

# F05 — security: SPEC

## Purpose

`internal/security` ports the security core of the Node prototype (`usage-allowance-checks/lib/core.mjs`) verbatim: opaque-token shape validation, bounded credential-file reads with permission inspection, exact-HTTPS and trusted-origin URL validation, bounded response reading, and the canary test harness that proves no credential material ever reaches an error string. It is Layer 0 (it MAY import `internal/config` per `specs/global/CONTRACTS.md` §8, and in practice imports stdlib only) and backs F06 (csvstore reads), F11/F12 (usage types, credentials), and F04's security posture.

Source: `docs/plan/research/usage-allowance-checks-spec.md` §1, §9; `usage-allowance-checks/lib/core.mjs` (the port source); `specs/global/CONTRACTS.md` §1.6 (Failure codes), §7 (constants); `specs/global/SPEC.md` §6 (security invariants 1–6).

## Behaviour

1. **Opaque-token validation.** `ValidateOpaqueToken(token)` accepts exactly the tokens that pass `assertOpaque`'s shape class: non-empty, at most 8192 bytes, containing no whitespace (Go `unicode.IsSpace`), no control characters (Go `unicode.IsControl`, which includes DEL `\u007f`), and no C0 control characters. Any violation returns `Error{Code:"unsafe_credential", Message:"The credential is missing or unsafe."}` — the fixed message never echoes the token (invariant #5). (Pinned bounds: 1..8192; see D1.)
2. **Bounded file reads.** `ReadBoundedFile(path, maxBytes)` stats, checks regularity, bounds by size (stat size and, after reading, actual byte length — two checks), and returns `(data, mode, nil)`. Missing file and non-regular path → `credential_file` "The credential file was not found."; size violations → `credential_file` "The credential file has an invalid size."; any other I/O failure → `credential_file` "The credential file could not be read safely." (underlying error details swallowed — no leakage). The returned `mode` is for the caller's permission **warning**; nothing in this feature ever chmod's or mutates the file (invariant #6). (`readBoundedFile`, core.mjs:37-60.)
3. **Broad-permission detection.** `HasBroadPermissions(mode)` is true when any group or other rwx bit is set — `mode.Perm() & 0o077 != 0` — matching `S_IRWXG|S_IRWXO` verbatim (core.mjs:72-74). Callers warn on true; they never auto-remediate.
4. **Exact-HTTPS allow-list.** `ValidateExactHTTPS(rawURL, allowed)` returns the Go-serialized URL when the input parses, is `https`, has no userinfo and no fragment, has a non-empty host, and its Go-serialized form is an exact member of `allowed` — no prefix, origin, or substring matching (invariant #1; `validateExactHttpsUrl`, core.mjs:76-91). Parse failure → `endpoint_refused` "The provider endpoint is not a valid URL."; any other rejection → `endpoint_refused` "The provider endpoint was refused.". Allow-list entries must be written in Go-normalized form (no WHATWG-style trailing slash injection).
5. **Trusted-origin fallback.** `ValidateTrustedBaseURL(rawURL, trustedOrigin)` requires: base `https`, no userinfo, no query, no fragment; trust argument `https`, no userinfo, no query, no fragment, and a bare origin (path `/` or empty — Go parses a bare origin with an empty path); and `base` origin exactly equal to `trusted` origin (protocol+host+port). Any violation → `untrusted_origin` "The configured Codex fallback origin was not explicitly trusted." On success it returns the fallback target: base origin + base path (with trailing slash) + the fixed suffix `api/codex/usage`, matching the Node original byte-for-byte (`validateTrustedBaseUrl`, core.mjs:93-116). Per-invocation trust is the caller's gate (F16) — this function only validates shape and origin equality (invariant #8).
6. **Bounded response reads.** `ReadResponseBounded(resp, maxBytes)` fails with `response_too_large` "The provider response exceeded the safe size limit." when the `Content-Length` header exceeds maxBytes (pre-check) or when the streamed/read body exceeds maxBytes (limited reader) — invariant #3, checked twice. It does not close the body; non-oversize read errors propagate unwrapped (they cannot carry credential material).
7. **Canary harness.** `WithCanary(canary, fn)` runs `fn` and returns its error unchanged when the canary string does not appear in the error text; when it does appear, it returns the sentinel `ErrCanaryLeak` (fixed text — the leaked error is NOT re-echoed, so even the harness output cannot carry the canary). Every error-producing path in this feature is tested under a canary (T7) proving invariant #5.

## Error behaviour

- All domain errors are `*security.Error{Code, Message}` with `Code` from the stable set in `specs/global/CONTRACTS.md` §1.6: `unsafe_credential`, `credential_file`, `endpoint_refused`, `untrusted_origin`, `response_too_large`.
- `Error.Error()` renders `"<code>: <message>"` with the fixed, sanitized messages above; messages are constructed from constants only and never interpolate input, paths, or underlying error text.
- F05 adds **no new `Failure.Code` values**, owns **no flags** and **no config keys**.

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Max token length | 8192 bytes, min 1; measured by byte length (`len(token)`) | pinned API surface; bounds memory, keeps the single-line/no-control-char invariant, and comfortably covers every real bearer token; byte length keeps the invariant about memory, not glyphs |
| D2 | Control-character predicate | reject any rune where `unicode.IsSpace(r) \|\| unicode.IsControl(r)` (IsControl includes `\u007f`) | superset of the Node regex `[\s\u0000-\u001f\u007f]` (also rejects C1 controls `\u0080-\u009f`); same accept class, fail-closed, deterministic in Go |
| D3 | Permission-bit threshold | `mode.Perm() & 0o077 != 0` — any group or other rwx bit triggers `HasBroadPermissions` | verbatim `S_IRWXG \| S_IRWXO` from core.mjs:72-74; `0o644`-default files correctly warn |
| D4 | Error-sanitization strategy | fixed constant messages per code; input, paths, and underlying error text are never interpolated; verified by the canary sweep (F05-T7) | invariant #5 (no credential material in errors/logs); the Node prototype swallows underlying details identically (`readBoundedFile` catch-all, core.mjs:51-60) |
| D5 | Missing vs oversized are distinct errors | same code `credential_file`, distinct fixed messages ("not found." / "invalid size.") | pinned contract ("distinct errors for missing vs oversized"); code stays stable for machine consumers, message distinguishes for humans |
| D6 | ReadBoundedFile takes an explicit `maxBytes` | no default; callers pass `MaxCredentialBytes` (or their own bound, e.g. F06's `MaxCsvBytes`) | pinned signature; the bound is a call-site decision, never hidden |
| D7 | URL comparison on Go serialization | `(*url.URL).String()`, exact membership | `net/url` is the canonical Go form; same rule as F04 (`specs/features/F04-http/SPEC.md` D4); WHATWG's automatic trailing slash is NOT emulated |
| D8 | Trusted-origin bareness | trust path must be empty or `/`; origin = `scheme://host` (port kept) | Go parses a bare origin with empty `Path`; WHATWG reports `/`; both accepted, anything else rejected |
| D9 | Fallback suffix kept verbatim | target = base origin + base path (+ `/`) + `api/codex/usage`; the defensive origin guard is omitted (Go string construction cannot change origin) | byte-for-byte port of core.mjs:111-116; `api/codex/usage` is a path, not a host constant, so the M4 `nousage` "no provider endpoint constants" invariant (`specs/DEPENDENCY-GRAPH.md` §4) is unaffected — host literals live in provider packages (F16), compiled out under `-tags nousage` |
| D10 | ReadResponseBounded does not close the body; non-oversize read errors pass through unwrapped | no `resp.Body.Close()`, raw error return | close ownership stays with the caller (httpkit/F14); read errors cannot carry secret material, so no sanitization is needed |
| D11 | Canary leak detection returns a sentinel | `ErrCanaryLeak` with fixed text; the offending error is not echoed | the harness's own output must never contain the canary either; `errors.Is` makes detection unambiguous |

## Milestone / dependencies

Milestone M1. `depends_on`: — (none, Wave W1, `specs/DEPENDENCY-GRAPH.md` §2–§3). `blocks`: F06 (csvstore), F11 (usage-types), F12 (credentials).

## Out of scope

- HTTP request execution, redirects, retries, User-Agent → F04-http (F04 mirrors the allow-list predicate natively because its graph row forbids a dependency on F05; both share the exact-match semantics and test vectors).
- Status mapping (`unauthorized`/`rate_limited`/`provider_status`), JSON parsing, `response_json` → F14-usage-fetch.
- `assertIdentifier` (the 1..512 identifier variant) → not part of the pinned F05 surface; F12-credentials owns identifier-shaped fields.
- Credential JSON parsing (`readCredentialJson`) → F12-credentials.
- The permission-warning TEXT and its stderr emission → F12 (via `output.WriteWarning`); F05 only detects.
- The `--trust-configured-origin` flag and Codex fallback orchestration → F16-provider-codex.
