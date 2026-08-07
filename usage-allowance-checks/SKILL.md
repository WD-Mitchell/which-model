---
name: usage-allowance-checks
description: Use when a user explicitly asks to inspect their current Claude, Codex, or GitHub Copilot usage allowance. Trigger when an interactive, read-only allowance report is needed without enabling automatic polling, spawn gating, or provider-consent enforcement.
---

# Usage allowance checks

Run one provider script explicitly from the hub root. These utilities read the
current user's machine-local authentication, make one bounded provider request,
and print only normalized allowance and reset fields. They do not change
credentials, persist device tokens, poll in the background, or authorize agent
spawns.

## Commands

```bash
node .agents/skills/usage-allowance-checks/scripts/claude-usage.mjs
node .agents/skills/usage-allowance-checks/scripts/codex-usage.mjs
node .agents/skills/usage-allowance-checks/scripts/copilot-usage.mjs
```

The three files above are the only user-facing entry scripts. Shared code under
`lib/` exists so filesystem, HTTP, and clock behavior can be mocked without
touching the real home directory or network.

## Provider behavior

### Claude

The Claude entry reads `~/.claude/credentials.json` and calls the fixed HTTPS
endpoint `https://api.anthropic.com/api/oauth/usage` with OAuth Bearer auth and
the `oauth-2025-04-20` beta header. It warns when the credential file is broader
than mode `0600`; it never changes permissions. Re-authenticate with Claude Code
when credentials are absent or expired.

### Codex

The Codex entry reads the access token and ChatGPT account identifier from
`~/.codex/auth.json`, then calls the fixed endpoint
`https://chatgpt.com/backend-api/wham/usage`. A configured provider base URL is
considered only when the primary endpoint returns a fixed unsupported status.
The token is sent to that configured origin only after an exact, explicit HTTPS
origin opt-in:

```bash
node .agents/skills/usage-allowance-checks/scripts/codex-usage.mjs \
  --trust-configured-origin https://trusted.example
```

The opt-in must match the configured URL's origin exactly. Credentials, query
strings, fragments, HTTP origins, redirects, and mismatches fail closed.

### GitHub Copilot

The Copilot entry checks only the named global/system Git key
`github.copilot.oauthToken`, then `gh auth token --hostname github.com`. It never
reads repository/local Git configuration or dumps config files. Every token is
validated with `GET https://api.github.com/user` before the private canonical
usage endpoint `https://api.github.com/copilot_internal/user` is called.

If no usable token exists, device login is opt-in:

```bash
node .agents/skills/usage-allowance-checks/scripts/copilot-usage.mjs --login
```

Only GitHub's validated verification URI and user code are shown. The device
code and resulting token remain in memory, polling respects GitHub's interval,
expiry, pending, slow-down, denial, and cancellation outcomes, and identity is
not printed unless `--show-identity` is also supplied.

## Security and compatibility

- Tokens must be opaque single-line values; whitespace or control characters
  are rejected. Tokens, fragments, hashes, auth headers, device codes, account
  identifiers, credential bodies, raw responses, and exception configs are
  never printed.
- Requests use Node's standard `fetch`, fixed HTTPS endpoints, manual redirect
  refusal, timeouts, bounded response bodies, and strict status/JSON handling.
- Provider-private endpoints, the Copilot OAuth client ID, and required editor
  headers are drift-prone. An unexpected shape or status returns a fixed,
  sanitized unsupported error; update this skill through review rather than
  weakening validation at runtime.
- Do not record live command output in tracked files. Use mocked tests for
  routine validation; live runs require the user's credentials and, for device
  login, their interaction.

## Checklist

- [ ] Run only the provider the user requested; never schedule or auto-wire it.
- [ ] For Codex fallback, confirm and explicitly trust the exact configured
  HTTPS origin.
- [ ] Use `--login` only with the user present; use `--show-identity` only when
  identity display was requested.
- [ ] Keep output sanitized and ephemeral; never paste credential or raw
  provider material into evidence.
- [ ] Treat private-endpoint drift as unsupported, not as permission to follow a
  redirect or broaden the accepted origin/response shape.
