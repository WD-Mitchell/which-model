#!/usr/bin/env bash
# Build a version-stamped bundle. Installation is opt-in, never a CI side effect.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
VERSION=""
PRODUCT=full
INSTALL=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:?version required}"; shift 2 ;;
    --offline) PRODUCT=offline; shift ;;
    --install) INSTALL=true; shift ;;
    *) echo "Usage: package-macos.sh [--version X.Y.Z] [--offline] [--install]" >&2; exit 2 ;;
  esac
done
if [ "$(go env GOOS)" != darwin ]; then echo 'macOS packaging requires a macOS toolchain' >&2; exit 1; fi
if [ -z "$VERSION" ]; then VERSION="$(git describe --tags --exact-match 2>/dev/null || echo 0.0.0-dev)"; fi
VERSION="${VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then echo 'Invalid desktop version' >&2; exit 2; fi
MODULE=github.com/WD-Mitchell/which-model
COMMIT="$(git rev-parse HEAD)"
BUILDDATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
APPNAME=which-model
BINARY=which-model-desktop
BUNDLE_ID=com.wdmitchell.which-model
TAGS=""
if [ "$PRODUCT" = offline ]; then APPNAME=which-model-offline; BINARY=which-model-score-only-desktop; BUNDLE_ID=com.wdmitchell.which-model.offline; TAGS=nousage; fi
APP="$ROOT/bin/$APPNAME.app"
EMBED="$ROOT/cmd/$BINARY/frontend"
BACKUP="$(mktemp -d)"
if [ -d "$EMBED" ]; then cp -R "$EMBED" "$BACKUP/frontend"; fi
cleanup() {
  rm -rf "$EMBED"
  if [ -d "$BACKUP/frontend" ]; then cp -R "$BACKUP/frontend" "$EMBED"; fi
  rm -rf "$BACKUP"
}
trap cleanup EXIT
mkdir -p "$EMBED/dist"
if [ "$PRODUCT" = offline ]; then
  python3 scripts/copy-offline-assets.py apps/desktop/dist "$EMBED/dist"
else
  cp -R apps/desktop/dist/. "$EMBED/dist/"
fi
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
LDFLAGS="-X ${MODULE}/pkg/whichmodel.Version=${VERSION} -X ${MODULE}/pkg/whichmodel.Commit=${COMMIT} -X ${MODULE}/pkg/whichmodel.BuildDate=${BUILDDATE} -X ${MODULE}/pkg/scoreonly.Version=${VERSION} -X ${MODULE}/pkg/scoreonly.Commit=${COMMIT}"
go build -trimpath -tags "$TAGS" -ldflags "$LDFLAGS" -o "$APP/Contents/MacOS/$BINARY" "./cmd/$BINARY"
python3 - "$APP" "$APPNAME" "$BINARY" "$BUNDLE_ID" "$VERSION" "$COMMIT" "$PRODUCT" <<'PY'
import json,plistlib,sys
from pathlib import Path
app,name,binary,identifier,version,commit,product=sys.argv[1:]
p=Path(app)/'Contents'
info={'CFBundleExecutable':binary,'CFBundleIdentifier':identifier,'CFBundleName':name,'CFBundleDisplayName':name,'CFBundleIconFile':'which-model.icns','CFBundlePackageType':'APPL','CFBundleVersion':version.split('-')[0],'CFBundleShortVersionString':version.split('-')[0],'LSMinimumSystemVersion':'13.0','LSUIElement':product=='full','NSHighResolutionCapable':True,'NSSupportsAutomaticGraphicsSwitching':True}
(p/'Info.plist').write_bytes(plistlib.dumps(info))
(p/'Resources'/'build-identity.json').write_text(json.dumps({'version':version,'source_commit':commit,'product':product},sort_keys=True)+'\n')
PY
cp icons/which-model.icns "$APP/Contents/Resources/which-model.icns"
# Ad-hoc integrity signing is not Developer ID signing or Apple notarization.
codesign --force --deep --sign - "$APP"
codesign --verify --deep --strict "$APP"
plutil -lint "$APP/Contents/Info.plist"
echo "Packaged $APP ($VERSION, $COMMIT)"
if [ "$INSTALL" = true ]; then
  mkdir -p "$HOME/Applications"
  ditto "$APP" "$HOME/Applications/$APPNAME.app"
  mdimport "$HOME/Applications/$APPNAME.app" || true
fi
