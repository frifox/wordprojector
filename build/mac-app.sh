#!/usr/bin/env bash
# Usage: build/mac-app.sh <binary> <output.app> <version>   (e.g. "dist/Word Projector.app")
set -euo pipefail
BIN="$1"; APP="$2"; VERSION="${3:-dev}"
NAME="WordProjector"        # executable inside the bundle
TITLE="Word Projector"      # name shown in Finder, the Dock and the menu bar
# CFBundle versions are numbers separated by dots: a release tag, the day "2026-10-22" (or "2026-10-22.2"), becomes
# 2026.10.22; anything else (a development build, "dev") gets 1.0
if [[ "$VERSION" =~ ^([0-9]{4})-([0-9]{2})-([0-9]{2}) ]]; then
  NUM="${BASH_REMATCH[1]}.$((10#${BASH_REMATCH[2]})).$((10#${BASH_REMATCH[3]}))"
else
  NUM="1.0"
fi
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/$NAME"
chmod +x "$APP/Contents/MacOS/$NAME"
cp build/icon.icns "$APP/Contents/Resources/icon.icns"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>$TITLE</string>
  <key>CFBundleDisplayName</key><string>$TITLE</string>
  <key>CFBundleExecutable</key><string>$NAME</string>
  <key>CFBundleIdentifier</key><string>org.wordprojector.app</string>
  <key>CFBundleVersion</key><string>$NUM</string>
  <key>CFBundleShortVersionString</key><string>$NUM</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleIconFile</key><string>icon.icns</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHumanReadableCopyright</key><string>GPL-3.0</string>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeExtensions</key><array><string>spsc</string></array>
      <key>CFBundleTypeName</key><string>Word Projector Schedule</string>
      <key>CFBundleTypeRole</key><string>Editor</string>
    </dict>
  </array>
</dict>
</plist>
PLIST
# a signature keeps Gatekeeper/Apple silicon from rejecting the copied binary: the Developer ID one when this Mac has the
# certificate (see mac-sign.sh), else an ad-hoc one
xattr -cr "$APP" 2>/dev/null || true
"$(dirname "$0")/mac-sign.sh" "$APP"
echo "built $APP"
