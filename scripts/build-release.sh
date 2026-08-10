#!/usr/bin/env bash
# Build release binaries for the npm platform packages and the GitHub release.
#
# Usage: scripts/build-release.sh <version>
# Example: scripts/build-release.sh 0.1.0
#
# Produces dist/ containing five binaries named after the GitHub release
# asset convention (which-model-<os>-<arch>[.exe]) plus checksums.txt.
set -euo pipefail

VERSION="${1:?usage: build-release.sh <version>}"
MODULE="github.com/WD-Mitchell/which-model"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILDDATE="$(date -u +%Y-%m-%d)"
LDFLAGS="-s -w -X ${MODULE}/pkg/whichmodel.Version=${VERSION} -X ${MODULE}/pkg/whichmodel.Commit=${COMMIT} -X ${MODULE}/pkg/whichmodel.BuildDate=${BUILDDATE}"
DIST="dist"

rm -rf "$DIST"
mkdir -p "$DIST"

build() {
  local goos="$1" goarch="$2" out="$3"
  echo "building ${goos}/${goarch} -> ${DIST}/${out}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
    -ldflags "$LDFLAGS" -o "${DIST}/${out}" ./cmd/which-model
}

build darwin  arm64 which-model-darwin-arm64
build darwin  amd64 which-model-darwin-x64
build linux   arm64 which-model-linux-arm64
build linux   amd64 which-model-linux-x64
build windows amd64 which-model-windows-x64.exe

(
  cd "$DIST"
  : > checksums.txt
  for f in which-model-*; do
    shasum -a 256 "$f" >> checksums.txt
  done
)
echo "wrote ${DIST}/checksums.txt"
