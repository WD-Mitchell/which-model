# @wdm-uk/which-model

This product is **pre-release**. A numeric version or npm distribution tag selects
an artifact; it does not establish stable readiness or company approval. Review
the [release readiness record](https://github.com/WD-Mitchell/which-model/blob/main/docs/releases/readiness.md)
and pin the version approved for your deployment.

Install the [which-model](https://github.com/WD-Mitchell/which-model) CLI:

```bash
npm install -g @wdm-uk/which-model
which-model version
```

Also works with pnpm, bun, and yarn.

`which-model` chooses the right AI model for the task — combining model
quality, cost, speed, task fit, provider availability, and remaining usage
allowance into a ranked, explainable recommendation.

## How it works

This package is a launcher. The real binary ships in a platform-specific
optional dependency (`@wdm-uk/which-model-darwin-arm64`,
`@wdm-uk/which-model-darwin-x64`, `@wdm-uk/which-model-linux-arm64`,
`@wdm-uk/which-model-linux-x64`, `@wdm-uk/which-model-windows-x64`); your
package manager installs only the one matching your platform. If that fails,
a postinstall fallback downloads the binary from the GitHub release.

Supported: macOS (arm64/x64), Linux (arm64/x64), Windows (x64). Requires
Node.js >= 18 for the launcher.

## Uninstall

```bash
npm uninstall -g @wdm-uk/which-model
```

## License

MIT — see the [repository](https://github.com/WD-Mitchell/which-model).

### Verified fallback downloads

If the optional platform package is unavailable, fallback installation requires
a trusted GitHub CLI 2.97.0+ and signed release evidence. Verification failures
leave no new executable; install the matching platform package or correct the
verifier/evidence issue. See the [release verification guide](https://github.com/WD-Mitchell/which-model/blob/main/docs/security/release-verification.md)
for source-identity checks and offline/mirror handling.
