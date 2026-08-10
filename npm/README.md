# @wdm-uk/which-model (npm)

`which-model` as an npm package — install with npm, pnpm, bun, or yarn and run
`which-model` from your PATH:

```bash
npm install -g @wdm-uk/which-model
# or: pnpm add -g @wdm-uk/which-model
# or: bun add -g @wdm-uk/which-model
```

The package itself contains only a small Node launcher. The real Go binary is
delivered through a platform-specific optional dependency
(`@wdm-uk/which-model-darwin-arm64`, `@wdm-uk/which-model-darwin-x64`,
`@wdm-uk/which-model-linux-arm64`, `@wdm-uk/which-model-linux-x64`,
`@wdm-uk/which-model-windows-x64`), so only the binary for your platform is
downloaded. If the optional dependency cannot be installed, a postinstall
script falls back to downloading the matching asset from the GitHub release
(set `WHICH_MODEL_SKIP_DOWNLOAD=1` to disable the fallback).

## Supported platforms

| Platform | Package |
|---|---|
| macOS arm64 (Apple Silicon) | `@wdm-uk/which-model-darwin-arm64` |
| macOS x64 (Intel) | `@wdm-uk/which-model-darwin-x64` |
| Linux arm64 | `@wdm-uk/which-model-linux-arm64` |
| Linux x64 | `@wdm-uk/which-model-linux-x64` |
| Windows x64 | `@wdm-uk/which-model-windows-x64` |

## Developing the npm packages

Layout:

- `which-model/` — launcher package (`@wdm-uk/which-model`): `bin.js`
  launcher, `install.js` release-asset fallback, manifest.
- `which-model-<os>-<arch>/` — binary packages; the release workflow drops the
  compiled `which-model` executable in before publishing.
- `scripts/sync-version.js <version>` — stamps one version into all six
  manifests.
- `scripts/publish.js [--dry-run]` — publishes platform packages, then the
  launcher. Authenticates via npm Trusted Publishing (OIDC); no stored token.

The release pipeline (`.github/workflows/npm-release.yml`) builds all five
binaries from the tag, uploads them as GitHub release assets with
`checksums.txt`, and publishes the npm packages. Binaries are never committed.
