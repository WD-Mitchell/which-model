# macOS release signing

Full and offline desktop release apps are Developer ID signed with hardened runtime and a secure timestamp, notarized by Apple, stapled, and assessed by Gatekeeper. Both arm64 and x64 must pass before publication. Local development packaging still uses ad-hoc signing.

## GitHub configuration

Create the `MacOS Publish` environment with these secrets:

| Secret | Value |
| --- | --- |
| `BUILD_CERTIFICATE_BASE64` | Base64-encoded Developer ID Application `.p12`, including its private key |
| `P12_PASSWORD` | Password used when exporting that `.p12` |
| `APPLE_ID` | Apple Developer account email |
| `APPLE_TEAM_ID` | Team identifier matching the certificate |
| `APPLE_APP_SPECIFIC_PASSWORD` | Apple app-specific password for notarization |

The desktop matrix in `npm-release.yml` imports the identity into a temporary keychain, stores the notary profile there, and removes both after use. The certificate export is deleted immediately after import. Signing credentials are scoped to that step. Keep all npm trusted publishers pointing to `Publish`, including the Darwin packages: the separate npm publishing job uses that environment.

Environment deployment policies must permit the exact candidate branch to test with `verify_only=true`. If only `main` and `v*` are permitted, an operator must explicitly add the candidate branch before dispatch. Do not create a release-shaped tag to work around this gate. Remove the temporary branch permission after validation.

## Local verification

Build the frontend, then package both products with the intended version:

```sh
pnpm install --frozen-lockfile
pnpm -r build
bash scripts/package-macos.sh --version 2.6.0-beta.1
bash scripts/package-macos.sh --version 2.6.0-beta.1 --offline
```

With a Developer ID Application identity and an existing Keychain notary profile, sign and submit both apps:

```sh
python3 scripts/sign-macos.py --arch arm64 --output-dir desktop-dist \
  --identity '<Developer ID certificate SHA-1>' --team-id '<TEAM_ID>' \
  --keychain-profile which-model-notary
```

Use `x64` on an Intel Mac. This command submits the built apps to Apple. A successful run prints confirmation for both products and writes one receipt and Apple log per app. It does not publish a GitHub release or npm package. A failed signing, notarization, stapling or Gatekeeper check stops the command. Each Apple submission has a 20-minute wait limit; a timeout requires investigation or a later retry.

Recipients can verify an extracted release app using:

```sh
codesign --verify --deep --strict which-model.app
codesign --display --verbose=4 which-model.app
xcrun stapler validate which-model.app
spctl --assess --type execute --verbose=4 which-model.app
```

Repeat for `which-model-offline.app`. Inspect the displayed Developer ID authority, team, runtime flag and timestamp. The release verifier authenticates CI receipts and their executable hashes using source-bound Sigstore attestations; these are separate from Apple’s native checks.

## Evidence and failures

The order is tests → package → Developer ID sign → Apple acceptance → staple and Gatekeeper → final archives → vulnerability scans and SBOMs → checksums and manifest → source-bound attestations → independent release verification. Publication depends on successful completion for both architectures.

The `.notarization.json` receipts describe the final signed executable and successful native checks. `.notarization.log.json` files contain Apple’s submission diagnostics. A rejected submission retains its log as a failed-job artifact; it cannot create a success receipt. Credentials and credential-tool output are excluded from logs.

A local arm64 test validates the installed identity and local notary profile. Only a GitHub candidate validates the stored P12 secrets, temporary-keychain import and both runner architectures. Release readiness requires that candidate from the exact release commit; earlier ad-hoc candidates are insufficient.
