#!/usr/bin/env bash
# Usage: build/mac-sign.sh <Word Projector.app> <output.zip>
# Signs the app, has Apple notarize it, staples the ticket on, and zips it. Without the certificate (the secrets below
# are not set) the app is only signed ad hoc: it runs, but users have to allow it in the system settings.
#
# Environment:
#   MACOS_CERT_P12_BASE64   the "Developer ID Application" certificate with its key (.p12), base64
#   MACOS_CERT_PASSWORD     the password of that .p12
#   NOTARY_KEY_P8_BASE64    the App Store Connect API key (.p8), base64
#   NOTARY_KEY_ID           its Key ID
#   NOTARY_ISSUER_ID        the Issuer ID of the account
# See docs/macos-signing.md.
set -euo pipefail
APP="$1"; ZIP="$2"
ZIP="$(cd "$(dirname "$ZIP")" && pwd)/$(basename "$ZIP")"

if [ -z "${MACOS_CERT_P12_BASE64:-}" ]; then
  echo "mac-sign: no certificate set: signing ad hoc (not notarized)"
  codesign --force --deep --sign - "$APP" || true
  ditto -c -k --keepParent "$APP" "$ZIP"
  exit 0
fi
for v in MACOS_CERT_PASSWORD NOTARY_KEY_P8_BASE64 NOTARY_KEY_ID NOTARY_ISSUER_ID; do
  [ -n "${!v:-}" ] || { echo "mac-sign: $v is not set"; exit 1; }
done

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

# a keychain of its own, holding the certificate only while this runs
security create-keychain -p "$KCPASS" "$KEYCHAIN"
security set-keychain-settings -lut 3600 "$KEYCHAIN"
security unlock-keychain -p "$KCPASS" "$KEYCHAIN"
echo "$MACOS_CERT_P12_BASE64" | base64 --decode > "$WORK/cert.p12"
security import "$WORK/cert.p12" -k "$KEYCHAIN" -P "$MACOS_CERT_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security >/dev/null
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KCPASS" "$KEYCHAIN" >/dev/null
# shellcheck disable=SC2086
security list-keychains -d user -s "$KEYCHAIN" $ORIGINAL_KEYCHAINS
IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN" | grep "Developer ID Application" | head -1 | sed -E 's/^[^"]*"(.*)".*$/\1/' || true)"
[ -n "$IDENTITY" ] || { echo "mac-sign: no \"Developer ID Application\" certificate in the .p12"; exit 1; }
echo "mac-sign: signing as $IDENTITY"

# hardened runtime and a secure timestamp: Apple's notarization wants both
codesign --force --deep --options runtime --timestamp --sign "$IDENTITY" "$APP"
codesign --verify --strict --verbose=2 "$APP"

# Apple scans the app (a few minutes) and answers Accepted or Invalid
echo "$NOTARY_KEY_P8_BASE64" | base64 --decode > "$WORK/key.p8"
ditto -c -k --keepParent "$APP" "$WORK/notarize.zip"
RESULT="$(xcrun notarytool submit "$WORK/notarize.zip" --key "$WORK/key.p8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" --wait --output-format json)"
echo "$RESULT"
STATUS="$(echo "$RESULT" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')"
if [ "$STATUS" != "Accepted" ]; then
  ID="$(echo "$RESULT" | /usr/bin/python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))')"
  [ -n "$ID" ] && xcrun notarytool log "$ID" --key "$WORK/key.p8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" || true
  echo "mac-sign: notarization status is \"$STATUS\""
  exit 1
fi

# the ticket goes into the app, so it is accepted without a network too
xcrun stapler staple "$APP"
spctl --assess --type execute --verbose=2 "$APP"
ditto -c -k --keepParent "$APP" "$ZIP"
echo "mac-sign: signed and notarized: $ZIP"
