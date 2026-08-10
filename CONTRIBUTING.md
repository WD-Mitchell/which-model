# Contributing to which-model

Thank you for helping improve `which-model`. Contributions may include bug fixes, product improvements, provider support, tests, documentation, and catalog data corrections.

## Before you start

1. Search existing issues and pull requests to avoid duplicate work.
2. Open an issue for a new feature or behavior change before writing code. Describe the user problem and expected outcome.
3. For a security vulnerability, do not open a public issue; follow [SECURITY.md](SECURITY.md).
4. Read [AGENTS.md](AGENTS.md) and the relevant feature documents under `specs/features/` before changing implementation behavior.

This repository is **spec-as-source**: specifications define the product contract. If implementation and specification disagree, update the specification through review before changing behavior.

## Development setup

Requirements:

- Go 1.25 or later
- Git
- Python 3 for specification validation

```bash
git clone https://github.com/WD-Mitchell/which-model.git
cd which-model
go mod download
go test ./...
```

Build and inspect the CLI:

```bash
mkdir -p bin
go build -o ./bin/which-model ./cmd/which-model
./bin/which-model --help
```

Do not place real provider credentials in source files, fixtures, logs, issues, or pull requests.

## Making a change

1. Create a focused branch from `main`.
2. Identify the feature task and its declared file list in `specs/features/F<NN>-<slug>/TASKS.md`.
3. Write the required test first and confirm that it fails for the intended reason.
4. Implement the smallest complete change that satisfies the specification.
5. Format Go files with `gofmt`.
6. Run the exact acceptance command named by the task.
7. Update user-facing documentation when commands, output, configuration, or behavior changes.

Keep changes focused. Do not combine unrelated refactors with a feature or bug fix.

## Required checks

Run the full project checks before opening a pull request:

```bash
go test ./... -count=1
go test -tags nousage ./... -count=1
go build ./cmd/which-model
go build -tags nousage ./cmd/which-model
bash scripts/audit-nousage.sh
python3 specs/verify_sdd.py
```

The `nousage` build is a product guarantee: provider adapters and endpoint constants must not be present in that binary.

## Tests

Tests should protect observable behavior rather than implementation details. Follow existing table-driven Go test conventions and use fixtures or golden files where the command contract requires them.

Credential-handling paths must include canary coverage proving that secrets never appear in errors or output. Tests must not access real home-directory configuration, keychains, credentials, or network services.

## Commit and pull request style

Use a concise Conventional Commit subject when practical:

```text
feat(pick): add task profile selector
fix(routes): preserve user-declared mappings
docs: clarify score-only setup
```

A pull request should:

- explain the user problem and the chosen behavior;
- identify the relevant feature task or issue;
- stay within the task's declared files;
- include tests for new or corrected behavior;
- include the commands run and their results;
- pass both default and `nousage` CI variants;
- contain no credential material or unrelated generated files.

Small, reviewable pull requests are preferred. Maintainers may ask for a specification change, narrower scope, additional behavioral coverage, or a clean rebase before merge.

## Documentation contributions

Documentation-only fixes are welcome. Keep product documentation concise and user-focused, preserve command and configuration names exactly, and verify every example against the current CLI.

By participating, you agree to follow the repository's [Code of Conduct](CODE_OF_CONDUCT.md).
