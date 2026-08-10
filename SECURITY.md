# Security Policy

Security is a core product requirement for `which-model`, especially where provider credentials and usage data are involved.

## Supported versions

Before the first tagged release, security fixes are made on `main`. After releases begin, the latest release and `main` will receive security fixes; older releases may be asked to upgrade.

| Version | Supported |
|---|---|
| `main` / latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

Please do **not** disclose a suspected vulnerability in a public issue, discussion, pull request, log, or test fixture.

1. Use GitHub's **Report a vulnerability** action on the repository's Security tab to open a private vulnerability report.
2. If private vulnerability reporting is unavailable, open a minimal public issue asking a maintainer to establish a private contact channel. Do not include vulnerability details, exploit steps, credentials, personal information, or sensitive output in that issue.
3. Clearly identify a Code of Conduct report as such if you are using the same private channel for an enforcement matter.

Include, when safe and relevant:

- the affected command, package, version, or commit;
- a concise description of the impact;
- reproducible steps or a minimal proof of concept;
- whether credentials, identity data, network boundaries, or generated automation are involved;
- any mitigation you have already tested.

Never send real credentials. Replace sensitive values with unmistakable canary strings and remove personal data from screenshots or logs.

Maintainers will acknowledge the report as soon as practical, investigate it privately, coordinate remediation and disclosure with the reporter, and credit the reporter if requested and appropriate. Please allow time for a fix before public disclosure.

## Security-sensitive areas

Reports are especially valuable for issues involving:

- credential discovery, storage, or accidental disclosure;
- provider endpoint allow-lists, redirects, or configured-origin trust;
- identity disclosure without explicit opt-in;
- unexpected provider access while usage is disabled;
- sensitive constants or adapters present in a `nousage` build;
- unsafe hook or skill installation, command execution, or file ownership;
- generated workflow permissions or secret exposure;
- malformed remote responses that bypass size, status, or schema checks.

For non-sensitive defects and feature requests, use the public issue tracker instead.
