#!/usr/bin/env bash
# Build and package the which-model desktop app as a macOS .app bundle.
#
# Usage: scripts/package-macos.sh
#
# Produces bin/which-model.app/ — drag to /Applications to install.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$ROOT/bin/which-model.app"
CONTENTS="$APP/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"

echo "==> Resolving build identity…"
# Stamp the bare release tag (vX.Y.Z or the tag's numeric X.Y.Z): the version
# string feeds both "Check for updates" (which compares against a tag) and the
# Settings sidebar. Commit/built metadata stays in `which-model version`.
# Queried BEFORE the rm -rf below, which deletes a tracked placeholder file
# and would otherwise brand every build "-dirty".
MODULE="github.com/WD-Mitchell/which-model"
VERSION="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null || echo dev)"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILDDATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "==> Copying frontend dist for embedding…"
rm -rf "$ROOT/cmd/which-model-desktop/frontend"
mkdir -p "$ROOT/cmd/which-model-desktop/frontend"
cp -r "$ROOT/apps/desktop/dist" "$ROOT/cmd/which-model-desktop/frontend/dist"

echo "==> Creating .app bundle structure…"
rm -rf "$APP"
mkdir -p "$MACOS" "$RESOURCES"

echo "==> Building desktop binary…"
LDFLAGS="-X ${MODULE}/pkg/whichmodel.Version=${VERSION} -X ${MODULE}/pkg/whichmodel.Commit=${COMMIT} -X ${MODULE}/pkg/whichmodel.BuildDate=${BUILDDATE}"
go build -trimpath -ldflags "$LDFLAGS" -o "$MACOS/which-model-desktop" ./cmd/which-model-desktop

# Derive the bundle version from the same source as the binary's ldflags.
# CFBundleShortVersionString must be a numeric X.Y.Z marketing version and
# CFBundleVersion a numeric build number — raw `git describe` output
# (v2.0.0-beta.1-11-ge9d5773, dev, ...-dirty) is NOT valid for either, so
# extract the release tag's X.Y.Z and fail packaging when none exists
# (issue #43 review: never copy describe output into the plist).
MARKETING_VERSION="$(printf '%s' "$VERSION" | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*$/\1/p')"
if [ -z "$MARKETING_VERSION" ]; then
  echo "error: no numeric X.Y.Z release version derivable from '$VERSION'." >&2
  echo "       tag the commit (e.g. v1.2.3) before packaging." >&2
  exit 1
fi
BUILD_NUMBER="$(git -C "$ROOT" rev-list --count "$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null)..HEAD" 2>/dev/null || echo 1)"
case "$BUILD_NUMBER" in
  ''|*[!0-9]*) BUILD_NUMBER=1 ;;
esac
echo "==> Bundle version: $MARKETING_VERSION (build $BUILD_NUMBER)"

# Info.plist — LSUIElement=true keeps the app out of the Dock (menu-bar only).
# CFBundleShortVersionString carries the marketing version, CFBundleVersion
# the numeric build counter; both derive from the same tag source as the
# binary's ldflags (issue #43), never a hardcoded literal.
cat > "$CONTENTS/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>which-model-desktop</string>
    <key>CFBundleIdentifier</key>
    <string>com.wdmitchell.which-model</string>
    <key>CFBundleName</key>
    <string>which-model</string>
    <key>CFBundleDisplayName</key>
    <string>which-model</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleVersion</key>
    <string>${BUILD_NUMBER}</string>
    <key>CFBundleShortVersionString</key>
    <string>${MARKETING_VERSION}</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
</dict>
</plist>
PLIST

# Copy tray icon as a resource.
if [ -f "$ROOT/cmd/which-model-desktop/assets/tray-icon.svg" ]; then
  cp "$ROOT/cmd/which-model-desktop/assets/tray-icon.svg" "$RESOURCES/"
fi

# Clean up the embedded frontend copy from the source tree.
rm -rf "$ROOT/cmd/which-model-desktop/frontend"

echo "==> Packaged: $APP"

# Auto-install to ~/Applications so the app appears in Spotlight and Launchpad.
INSTALL_DIR="$HOME/Applications"
mkdir -p "$INSTALL_DIR"
rm -rf "$INSTALL_DIR/which-model.app"
cp -R "$APP" "$INSTALL_DIR/which-model.app"
# Trigger Spotlight re-index for the new .app.
mdimport "$INSTALL_DIR/which-model.app" 2>/dev/null || true

echo "==> Installed: $INSTALL_DIR/which-model.app"
echo "    Search 'which-model' in Spotlight or open from ~/Applications."
