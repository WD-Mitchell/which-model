#!/usr/bin/env bash
# End-to-end smoke test for the npm packaging.
#
# Packs the launcher and the platform package for the HOST platform, installs
# them into a scratch project with a real package manager, and runs the
# installed `which-model` binary.
#
# Usage: scripts/smoke-npm.sh [npm|pnpm|bun]   (default: npm)
#
# Requires: go, node, and the chosen package manager on PATH.
set -euo pipefail

MANAGER="${1:-npm}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM_DIR="$(node -p '`${process.platform}-${process.arch}`')"
BIN_NAME="which-model"
[ "$(node -p 'process.platform')" = "win32" ] && BIN_NAME="which-model.exe"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "==> building host binary"
CGO_ENABLED=0 go build -trimpath -o "$ROOT/npm/wdm-uk-${PLATFORM_DIR}/${BIN_NAME}" ./cmd/which-model
chmod +x "$ROOT/npm/wdm-uk-${PLATFORM_DIR}/${BIN_NAME}"
cleanup_binary() { rm -f "$ROOT/npm/wdm-uk-${PLATFORM_DIR}/${BIN_NAME}"; }
trap 'cleanup_binary; rm -rf "$WORK"' EXIT

echo "==> stamping versions"
SAVED_VERSION="$(node -p "require('$ROOT/npm/which-model/package.json').version")"
node "$ROOT/npm/scripts/sync-version.js" 0.0.0-smoke
trap 'node "$ROOT/npm/scripts/sync-version.js" "$SAVED_VERSION" >/dev/null 2>&1; cleanup_binary; rm -rf "$WORK"' EXIT

echo "==> packing"
(cd "$ROOT/npm/wdm-uk-${PLATFORM_DIR}" && npm pack --pack-destination "$WORK" >/dev/null)
(cd "$ROOT/npm/which-model" && npm pack --pack-destination "$WORK" >/dev/null)

echo "==> installing with $MANAGER"
cd "$WORK"
mkdir project && cd project
case "$MANAGER" in
  npm)
    npm init -y >/dev/null
    npm install "$WORK"/wdm-uk-"${PLATFORM_DIR}"-*.tgz "$WORK"/wdm-uk-which-model-*.tgz >/dev/null
    ;;
  pnpm)
    pnpm init >/dev/null 2>&1 || true
    # pnpm >= 10 exits 1 with ERR_PNPM_IGNORED_BUILDS when a dependency's
    # lifecycle scripts are not pre-approved; the install itself succeeds.
    set +e
    pnpm_out="$(pnpm add "$WORK"/wdm-uk-"${PLATFORM_DIR}"-*.tgz "$WORK"/wdm-uk-which-model-*.tgz 2>&1)"
    pnpm_status=$?
    set -e
    if [ "$pnpm_status" -ne 0 ] && ! echo "$pnpm_out" | grep -q "ERR_PNPM_IGNORED_BUILDS"; then
      echo "$pnpm_out" >&2
      echo "pnpm add failed (exit $pnpm_status)" >&2
      exit 1
    fi
    ;;
  bun)
    bun init -y >/dev/null
    bun add "$WORK"/wdm-uk-"${PLATFORM_DIR}"-*.tgz "$WORK"/wdm-uk-which-model-*.tgz >/dev/null
    ;;
  *) echo "unknown package manager: $MANAGER" >&2; exit 2 ;;
esac

echo "==> running installed which-model"
WHICH_MODEL_SKIP_DOWNLOAD=1 ./node_modules/.bin/which-model version

echo "==> smoke test passed ($MANAGER)"
