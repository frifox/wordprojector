#!/usr/bin/env bash
# Usage: build/mac-sign.sh <Word Projector.app> [--notarize]
# Signs the app on this Mac with the Developer ID certificate (hardened runtime, secure timestamp). With --notarize Apple also
# scans it (a few minutes) and the ticket is stapled on, so macOS opens it without asking. Without the certificate the app is
# signed ad hoc: it runs on this Mac, but others have to allow it in System Settings. (The GitHub release has its own copy of
# this in .github/workflows/release.yml, using repository secrets.)
#
# What it uses (all outside the repository), from ~/.wordprojector-signing/ or the file named by WP_SIGNING_ENV:
#   WordProjector-DeveloperID.p12   the certificate with its key; MACOS_CERT_PASSWORD.txt next to it holds its password
#   signing.env                     shell settings: NOTARY_KEY_P8 (path of the AuthKey_XXXX.p8 of App Store Connect),
#                                   NOTARY_KEY_ID, NOTARY_ISSUER_ID; MACOS_CERT_P12 and MACOS_CERT_PASSWORD may be set there too
# NO_NOTARIZE=1 skips the notarization even with --notarize (a quick signed build).
set -euo pipefail
APP="${1:?usage: mac-sign.sh <app> [--notarize]}"
NOTARIZE=0; [ "${2:-}" = "--notarize" ] && [ "${NO_NOTARIZE:-0}" != 1 ] && NOTARIZE=1

DIR="$HOME/.wordprojector-signing"
# shellcheck disable=SC1090
[ -f "${WP_SIGNING_ENV:-$DIR/signing.env}" ] && . "${WP_SIGNING_ENV:-$DIR/signing.env}"
P12="${MACOS_CERT_P12:-$DIR/WordProjector-DeveloperID.p12}"
if [ -z "${MACOS_CERT_PASSWORD:-}" ] && [ -f "$(dirname "$P12")/MACOS_CERT_PASSWORD.txt" ]; then
  MACOS_CERT_PASSWORD="$(cat "$(dirname "$P12")/MACOS_CERT_PASSWORD.txt")"
fi

if [ ! -f "$P12" ] || [ -z "${MACOS_CERT_PASSWORD:-}" ]; then
  echo "mac-sign: no certificate ($P12): signing ad hoc, other Macs will ask before opening it"
  codesign --force --deep --sign - "$APP" >/dev/null 2>&1 || echo "warning: could not sign $APP"
  exit 0
fi

WORK="$(mktemp -d)"
KEYCHAIN="$WORK/signing.keychain-db"
KCPASS="$(uuidgen)"
ORIGINAL_KEYCHAINS="$(security list-keychains -d user | tr -d '"' | tr '\n' ' ')"
cleanup() {
  # shellcheck disable=SC2086
  security list-keychains -d user -s $ORIGINAL_KEYCHAINS >/dev/null 2>&1 || true
  security delete-keychain "$KEYCHAIN" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

# a keychain of its own, holding the certificate only while this runs (nothing is added to the login keychain)
security create-keychain -p "$KCPASS" "$KEYCHAIN"
security set-keychain-settings -lut 3600 "$KEYCHAIN"
security unlock-keychain -p "$KCPASS" "$KEYCHAIN"
security import "$P12" -k "$KEYCHAIN" -P "$MACOS_CERT_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security >/dev/null
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KCPASS" "$KEYCHAIN" >/dev/null
# shellcheck disable=SC2086
security list-keychains -d user -s "$KEYCHAIN" $ORIGINAL_KEYCHAINS
IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN" | grep "Developer ID Application" | head -1 | sed -E 's/^[^"]*"(.*)".*$/\1/' || true)"
[ -n "$IDENTITY" ] || { echo "mac-sign: no valid \"Developer ID Application\" certificate in $P12"; exit 1; }
echo "mac-sign: signing as $IDENTITY"
xattr -cr "$APP" 2>/dev/null || true
codesign --force --deep --options runtime --timestamp --sign "$IDENTITY" "$APP"
codesign --verify --strict --verbose=2 "$APP"

if [ $NOTARIZE = 0 ]; then
  echo "mac-sign: signed, not notarized"
  exit 0
fi
for v in NOTARY_KEY_P8 NOTARY_KEY_ID NOTARY_ISSUER_ID; do
  [ -n "${!v:-}" ] || { echo "mac-sign: $v is not set (see $DIR/signing.env): signed, not notarized"; exit 0; }
done
[ -f "$NOTARY_KEY_P8" ] || { echo "mac-sign: $NOTARY_KEY_P8 does not exist: signed, not notarized"; exit 0; }

# Apple answers Accepted or Invalid
ditto -c -k --keepParent "$APP" "$WORK/notarize.zip"
echo "mac-sign: notarizing (a few minutes)"
RESULT="$(xcrun notarytool submit "$WORK/notarize.zip" --key "$NOTARY_KEY_P8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" --wait --output-format json)"
echo "$RESULT"
STATUS="$(echo "$RESULT" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')"
if [ "$STATUS" != "Accepted" ]; then
  ID="$(echo "$RESULT" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))')"
  [ -n "$ID" ] && xcrun notarytool log "$ID" --key "$NOTARY_KEY_P8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" || true
  echo "mac-sign: notarization status is \"$STATUS\""
  exit 1
fi
xcrun stapler staple "$APP"
spctl --assess --type execute --verbose=2 "$APP"
echo "mac-sign: signed and notarized: $APP"
