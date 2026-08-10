# which-model

[![CI](https://github.com/WD-Mitchell/which-model/actions/workflows/ci.yml/badge.svg)](https://github.com/WD-Mitchell/which-model/actions/workflows/ci.yml)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Choose the right AI model for the task you are doing—not just the model at the top of a benchmark.**

`which-model` is a local-first command-line tool that combines model quality, cost, speed, task fit, provider availability, and your remaining usage allowance. It returns a ranked, explainable recommendation with the provider, model ID, and reasoning effort to use.

> [!NOTE]
> The project is currently pre-release. Commands and configuration may evolve before the first stable release.

## Why which-model?

Model choice changes with the job. A fast, inexpensive model may be ideal for a small edit, while research, planning, review, or orchestration may need a different balance. Provider limits can change the answer again.

`which-model` brings those signals together so you can:

- pick models using task-focused profiles;
- account for live Claude, Codex, and GitHub Copilot allowance;
- compare quality, cost, speed, and provider preference;
- exclude unavailable or usage-gated candidates;
- explain and audit every recommendation;
- feed consistent JSON into scripts, agents, and CI workflows;
- run in score-only mode without reading provider credentials.

## Install

### With npm, pnpm, or bun

```bash
npm install -g @wdm-uk/which-model
# or: pnpm add -g @wdm-uk/which-model
# or: bun add -g @wdm-uk/which-model

which-model version
```

The npm package ships your platform's compiled binary via an optional
dependency (macOS arm64/x64, Linux arm64/x64, Windows x64); only the matching
binary is downloaded.

### With Go

Requires Go 1.25 or later:

```bash
go install github.com/WD-Mitchell/which-model/cmd/which-model@latest
```

Make sure your Go binary directory—usually `$(go env GOPATH)/bin`—is on `PATH`, then confirm the installation:

```bash
which-model version
```

### From source

```bash
git clone https://github.com/WD-Mitchell/which-model.git
cd which-model
mkdir -p bin
go build -o ./bin/which-model ./cmd/which-model
./bin/which-model --help
```

The source checkout includes the starter provider and benchmark files used in the examples below.

## Quick start

### 1. Refresh the model catalog

From the source checkout, refresh the catalog with an [Artificial Analysis](https://artificialanalysis.ai/) API key and the included starter catalog files:

```bash
export ARTIFICIAL_ANALYSIS_API="your-api-key"

which-model catalog refresh \
  --provider-config available-model-data-export/providers.toml \
  --benchmarks available-model-data-export/benchmarks.toml
```

You can also place `ARTIFICIAL_ANALYSIS_API=...` in a repository-root `.env` file. Never commit that file.

### 2. Build the provider-to-model routes

```bash
which-model routes refresh
which-model routes verify
```

Routes connect provider-native model IDs to catalog models and reasoning levels.

### 3. Pick a model

Start in score-only mode; this performs no provider credential or usage reads:

```bash
which-model pick \
  --profile balanced_implementation \
  --strategy score \
  --no-usage \
  --json
```

Other useful profiles include `simple_implementation`, `complex_implementation`, `research`, `planning`, `orchestration`, `review`, `ui_ux`, and `financial_work`.

Explain the latest recommendation:

```bash
which-model explain --last --json
```

## Add live provider allowance

Providers are disabled by default. Opt in only to the providers you want `which-model` to inspect:

```bash
which-model config set providers.claude.enabled true
which-model config set providers.codex.enabled true
which-model config set providers.copilot.enabled true
```

Check which credentials can be resolved, then request a usage report:

```bash
which-model auth status claude codex copilot
which-model usage --all --json
```

Once at least one provider is enabled, the default `usage.enabled = "auto"` setting allows `pick` to incorporate live or cached allowance data. Use `--no-usage` at any time to force a score-only run.

> [!IMPORTANT]
> Identity output is opt-in, redirects are rejected, responses are bounded, and credential material is never included in normal output or errors. A fresh installation reads no provider credentials until a provider is explicitly enabled.

## Common commands

| Command | What it does |
|---|---|
| `which-model catalog refresh` | Refresh model and benchmark data, then derive scores. |
| `which-model catalog list` | Browse scored catalog entries. |
| `which-model routes refresh` | Rebuild provider-to-model mappings. |
| `which-model pick --profile <name>` | Rank candidates and choose a model. |
| `which-model explain --last` | Show the evidence behind the latest pick. |
| `which-model usage --all` | Report allowance for enabled providers. |
| `which-model auth status` | Check credential availability without displaying secrets. |
| `which-model config show` | Print the resolved configuration. |
| `which-model schema pick` | Print the JSON Schema for automation. |

Run `which-model <command> --help` for all options.

## Selection strategies

- **`score`** — highest final score; the default.
- **`priority`** — prefer providers using configured priority.
- **`round-robin`** — rotate across candidates.
- **`least-used`** — prefer the provider with the most allowance remaining.
- **`weighted-random`** — make a reproducible weighted choice with `--seed`.
- **`cost-optimal`** — prefer lower cost above a quality floor.

Example:

```bash
which-model pick --profile research --strategy least-used --json
```

## Agent integration

The repository ships skills for model selection, provider usage, and usage-aware dispatch:

```bash
which-model skills install --repo .
```

For Claude-compatible project skills:

```bash
which-model skills install --repo . --target claude
```

Optional hooks can enforce the same selection flow at dispatch boundaries:

```bash
which-model hooks install --repo . --target claude
```

All primary result commands support machine-readable JSON, and `which-model schema <command>` exposes the corresponding schema.

## Configuration

Use a project file at `.which-model/config.toml`, a user configuration file, environment variables prefixed with `WHICH_MODEL_`, or command-line flags. Inspect the active values and file location with:

```bash
which-model config path
which-model config show
which-model config validate
```

A minimal project configuration that enables Claude usage is:

```toml
[usage]
enabled = "auto"

[providers.claude]
enabled = true
priority = 10
weight = 1.0
```

Command-line flags take precedence over environment and file configuration.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | Success |
| `1` | Runtime failure |
| `2` | Invalid arguments or configuration |
| `3` | No viable candidate remains |
| `4` | Every eligible provider is usage-gated |
| `5` | Provider authentication is required |

Contributions are welcome—see [CONTRIBUTING.md](CONTRIBUTING.md), the [Code of Conduct](CODE_OF_CONDUCT.md), and [SECURITY.md](SECURITY.md) before opening a pull request or reporting a vulnerability.
