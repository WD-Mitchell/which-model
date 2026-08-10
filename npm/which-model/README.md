# @wdm-uk/which-model

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
optional dependency (`@wdm-uk/darwin-arm64`, `@wdm-uk/darwin-x64`,
`@wdm-uk/linux-arm64`, `@wdm-uk/linux-x64`, `@wdm-uk/windows-x64`); your
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
