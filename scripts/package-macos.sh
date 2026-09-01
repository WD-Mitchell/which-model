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

echo "==> Copying frontend dist for embedding…"
rm -rf "$ROOT/cmd/which-model-desktop/frontend"
mkdir -p "$ROOT/cmd/which-model-desktop/frontend"
cp -r "$ROOT/apps/desktop/dist" "$ROOT/cmd/which-model-desktop/frontend/dist"

echo "==> Creating .app bundle structure…"
rm -rf "$APP"
mkdir -p "$MACOS" "$RESOURCES"

echo "==> Building desktop binary…"
# Stamp the release identity, exactly as scripts/build-release.sh does for the
# CLI. Without it whichmodel.Version stays "dev" and the tray's "Check for
# updates" has nothing to compare against.
MODULE="github.com/WD-Mitchell/which-model"
VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILDDATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X ${MODULE}/pkg/whichmodel.Version=${VERSION} -X ${MODULE}/pkg/whichmodel.Commit=${COMMIT} -X ${MODULE}/pkg/whichmodel.BuildDate=${BUILDDATE}"
(cd "$ROOT" && go build -trimpath -ldflags "$LDFLAGS" -o "$MACOS/which-model-desktop" ./cmd/which-model-desktop)

# Derive the bundle version from the same source as the binary's ldflags:
# the git tag (v-prefixed stripped), falling back to the raw describe output
# (dev/dirty builds) so the plist never contradicts the binary it wraps.
if [[ "$VERSION" == v* ]]; then
  BUNDLE_VERSION="${VERSION#v}"
else
  BUNDLE_VERSION="$VERSION"
fi
echo "==> Bundle version: $BUNDLE_VERSION"

# Info.plist — LSUIElement=true keeps the app out of the Dock (menu-bar only).
# CFBundleVersion/ShortVersionString are interpolated from BUNDLE_VERSION
# (issue #43): the plist must match the binary's stamped version, never a
# hardcoded literal.
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
    <string>${BUNDLE_VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${BUNDLE_VERSION}</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
</dict>
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
