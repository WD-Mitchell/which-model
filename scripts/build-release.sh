#!/usr/bin/env bash
# Build release binaries for the npm platform packages and the GitHub release.
#
# Usage: scripts/build-release.sh <version>
# Example: scripts/build-release.sh 0.1.0
#
# Produces full and restricted binaries, capabilities, checksums and SBOMs.
set -euo pipefail

VERSION="${1:?usage: build-release.sh <version>}"
MODULE="github.com/WD-Mitchell/which-model"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
SOURCE_COMMIT="$(git rev-parse HEAD)"
BUILDDATE="$(date -u +%Y-%m-%d)"
LDFLAGS="-s -w -X ${MODULE}/pkg/whichmodel.Version=${VERSION} -X ${MODULE}/pkg/whichmodel.Commit=${COMMIT} -X ${MODULE}/pkg/whichmodel.BuildDate=${BUILDDATE}"
SCORE_LDFLAGS="-s -w -X ${MODULE}/pkg/scoreonly.Version=${VERSION} -X ${MODULE}/pkg/scoreonly.Commit=${SOURCE_COMMIT}"
DIST="dist"

rm -rf "$DIST"
mkdir -p "$DIST"

build() {
  local goos="$1" goarch="$2" out="$3"
  echo "building ${goos}/${goarch} -> ${DIST}/${out}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
    -ldflags "$LDFLAGS" -o "${DIST}/${out}" ./cmd/which-model
  local restricted="${out/which-model-/which-model-score-only-}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -buildvcs=false -tags nousage \
    -ldflags "$SCORE_LDFLAGS" -o "${DIST}/${restricted}" ./cmd/which-model-score-only
}

build darwin  arm64 which-model-darwin-arm64
build darwin  amd64 which-model-darwin-x64
build linux   arm64 which-model-linux-arm64
build linux   amd64 which-model-linux-x64
build windows amd64 which-model-windows-x64.exe

CGO_ENABLED=0 go run -trimpath -buildvcs=false -tags nousage -ldflags "$SCORE_LDFLAGS" \
  ./cmd/which-model-score-only capabilities --json > "$DIST/which-model-score-only-capabilities.json"

(
  cd "$DIST"
  : > checksums.txt
  for f in which-model-*; do
    shasum -a 256 "$f" >> checksums.txt
  done
)
echo "wrote ${DIST}/checksums.txt"
SOURCE_REF="${GITHUB_REF:-$(git symbolic-ref HEAD)}"
python3 scripts/release_metadata.py "$DIST" "$VERSION" "$SOURCE_COMMIT" "$SOURCE_REF"
