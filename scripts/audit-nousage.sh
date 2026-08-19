#!/usr/bin/env bash
# audit-nousage.sh — nousage build-matrix audit
# (specs/features/F21-usage-toggle/SPEC.md §2.2 step 11, §2.3, §5 R5;
#  docs/plan/annex-a-provider-matrix.md §1a.3, annex-d §4.7, annex-b §0).
#
# Verifies, in order:
#   1. the default build compiles,
#   2. the compiled-out (-tags nousage) binary builds,
#   3. the nousage binary contains no provider endpoint strings,
#   4. the usage-free packages pass their full suites under -tags nousage.
set -euo pipefail

# Step 1 — default build (CLI + libraries; desktop app needs GTK on Linux).
go build ./cmd/which-model ./internal/... ./pkg/...

# Step 2 — compiled-out build.
BIN="$(mktemp -d)/which-model-nousage"
go build -tags nousage -o "$BIN" ./cmd/which-model

# Step 3 — strings scan: zero matches is the pass condition.
if strings "$BIN" | grep -qE 'chatgpt.com/backend-api|api.anthropic.com|copilot_internal'; then
	echo "provider endpoint strings leaked into nousage build" >&2
	exit 1
fi

# Step 4 — compiled-out tests: catalog must be usage-free and fully green
# under nousage (annex-b §0), and the usage/pick stubs must hold.
go test -tags nousage ./internal/catalog/...
go test -tags nousage ./internal/usage/... ./internal/pick/...

# Step 5 — positive marker for CI logs.
echo "audit-nousage: OK (default build, nousage build, strings scan, catalog/usage/pick tests)"
