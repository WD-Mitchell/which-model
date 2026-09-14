# Desktop company workflows

The full desktop app exposes company administration in **Settings → Security & privacy**. Policy is read-only: installation, source, executable and CodexBar approvals still come from protected administrator files.

The page shows enrollment, policy identity/digest, permitted providers and credential sources, retention, integrations and execution restrictions. Expand an approved executable or CodexBar entry to inspect its protected paths, arguments and hashes. **Verify approved files** checks the CodexBar image/configuration without starting a process. It is a point-in-time result; normal usage verifies them again.

Personal users can enable **Use native OS credential storage**. Managed installations select native storage through policy and cannot change it in Settings. Existing General settings continue to preserve the native preference; the company keychain switch is marked administrator-controlled.

For an owned legacy credential, select its provider and choose **Migrate credential**. Replacement and source removal are separate explicit choices, off by default. The OS copy must be verified before optional removal. Read both the secure-store and legacy-copy outcomes: a verified OS copy can coexist with a retained/recovery source. Partial failures remain visible. Migration never deletes provider-owned credentials or grants an otherwise prohibited source/provider. Company catalog-key migration uses the Artificial Analysis entry.

**Clean up expired records** applies company retention and minimization. **Delete selected records** requires one or more categories and confirmation; it deletes owned records regardless of age. A previous project's absolute directory can be supplied for its two legacy audit files. The app does not scan other projects. Category reports distinguish retained, removed, scrubbed and failed records/files. Usage purge clears current in-memory evidence; enabled polling or new launches can create new records afterward. Automatic maintenance runs every minute while the app is running, and its latest result is visible here. A stopped app still needs scheduled CLI cleanup to process expiry.

In **Harnesses**, company launch previews show the protected command template instead of ignored user configuration. A missing approval is shown as unavailable. The preview is not proof of file verification or process success. Native launch/copy actions still verify approval and retain advisory quota/audit reporting.

## Separate offline desktop

`which-model-offline.app` is a separate executable. It offers only built-in profiles, embedded-catalog ranking and build/capability inspection. Ranking uses the same `pkg/scoreonly` engine and inputs as the restricted CLI. There is no full desktop service, provider discovery, credential access, company policy loading, catalog refresh, harness launch, integration installation or update check. Installing it does not enroll or configure the full app.

This GUI has a native Wails/webview host. The webview's own OS storage and local asset/binding transport are additional to the engine; the GUI is **not** a claim that the whole native framework satisfies the restricted CLI's no-I/O process audit. Its asset closure contains only the offline entry, uses a same-origin content policy, and its three bound operations are profiles, ranking and capabilities. The original restricted CLI and its stricter import/symbol/syscall checks are unchanged.

## macOS downloads and updates

Release assets include `which-model-desktop-darwin-arm64.zip` and `which-model-offline-desktop-darwin-arm64.zip` for Apple Silicon, with corresponding `x64` archives for Intel Macs. They are built on native macOS runners, stamped with the requested version and exact source commit, and included in the release's checksum, SBOM, vulnerability-review and signed provenance verification. The inventory covers Go dependencies and the archived app, including its embedded frontend; it is not an inventory of OS-provided WebKit or system frameworks.

Verify the release evidence using [the release verification instructions](security/release-verification.md), then extract the selected archive and copy the app to Applications. These preview bundles have ad-hoc code signatures for local integrity. They are not Apple Developer ID signed or notarized; company distribution must follow the organization's macOS software approval process. Do not treat GitHub attestation as Apple notarization.

The full desktop's **Check for updates…** opens a newer version's release page. It does not replace an installed bundle or bypass company binary/digest approvals. The offline app has no update network path; update it by selecting and verifying a new archive. Windows/Linux CLI support does not imply desktop bundle support on those platforms.

For a local build, run `pnpm -r build`, then `bash scripts/package-macos.sh` (add `--offline` for the separate app). Installation is now opt-in with `--install`; packaging alone writes only `bin/`. `--version X.Y.Z[-suffix]` explicitly stamps a candidate/release. Without it, an exact Git tag is used or the build is labeled `0.0.0-dev` rather than inheriting an unrelated older tag.
