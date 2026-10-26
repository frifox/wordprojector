#!/usr/bin/env bash
# One-time helpers for signing the macOS app (see docs/macos-signing.md).
#   build/mac-signing-setup.sh csr                  makes a private key and the certificate request to upload to Apple
#   build/mac-signing-setup.sh p12 <Developer ID .cer>   makes the .p12 from the certificate Apple made, and prints the secrets
# The private key stays in ~/.wordprojector-signing (never in the repository). Keep a backup of that folder: without the key
# the certificate is useless, and Apple cannot give it back.
set -euo pipefail
DIR="$HOME/.wordprojector-signing"
cd "$(dirname "$0")/.."
REPO_OUT="dist/signing"

case "${1:-}" in
csr)
  mkdir -p "$DIR" && chmod 700 "$DIR"
  [ ! -e "$DIR/developer-id.key" ] || { echo "$DIR/developer-id.key exists already: not replacing it (the certificate must match it)"; exit 1; }
  NAME="$(git config user.name || echo "Word Projector")"
  umask 077
  /usr/bin/openssl genrsa -out "$DIR/developer-id.key" 2048 2>/dev/null
  /usr/bin/openssl req -new -key "$DIR/developer-id.key" -out "$DIR/WordProjector.csr" -subj "/CN=$NAME"
  mkdir -p "$REPO_OUT" && cp "$DIR/WordProjector.csr" "$REPO_OUT/WordProjector.csr"
  echo "private key:          $DIR/developer-id.key   (keep it, back it up, never share it)"
  echo "certificate request:  $DIR/WordProjector.csr  (copy: $REPO_OUT/WordProjector.csr)"
  echo
  echo "Next: developer.apple.com > Certificates > + > Developer ID Application > G2 Sub-CA, upload the .csr, download the .cer,"
  echo "then run: build/mac-signing-setup.sh p12 <the .cer>"
  ;;
p12)
  CER="${2:-}"; [ -f "$CER" ] || { echo "usage: $0 p12 <Developer ID Application .cer>"; exit 1; }
  [ -f "$DIR/developer-id.key" ] || { echo "$DIR/developer-id.key is missing: the certificate was made for another key"; exit 1; }
  umask 077
  # Apple gives a DER file; the key in the folder has to be the one the certificate was made for
  /usr/bin/openssl x509 -inform DER -in "$CER" -out "$DIR/developer-id.pem" 2>/dev/null || cp "$CER" "$DIR/developer-id.pem"
  A="$(/usr/bin/openssl x509 -in "$DIR/developer-id.pem" -noout -modulus | shasum)"
  B="$(/usr/bin/openssl rsa -in "$DIR/developer-id.key" -noout -modulus 2>/dev/null | shasum)"
  [ "$A" = "$B" ] || { echo "this certificate was not made from the request of $DIR/developer-id.key"; exit 1; }
  PASS="$(/usr/bin/openssl rand -base64 24)"
  /usr/bin/openssl pkcs12 -export -inkey "$DIR/developer-id.key" -in "$DIR/developer-id.pem" -name "Developer ID Application" -out "$DIR/WordProjector-DeveloperID.p12" -passout "pass:$PASS"
  base64 < "$DIR/WordProjector-DeveloperID.p12" | tr -d '\n' > "$DIR/MACOS_CERT_P12_BASE64.txt"
  printf '%s' "$PASS" > "$DIR/MACOS_CERT_PASSWORD.txt"
  echo "made $DIR/WordProjector-DeveloperID.p12"
  echo
  echo "GitHub secrets (repository > Settings > Secrets and variables > Actions > New repository secret):"
  echo "  MACOS_CERT_P12_BASE64   = the content of $DIR/MACOS_CERT_P12_BASE64.txt   (pbcopy < that file)"
  echo "  MACOS_CERT_PASSWORD     = the content of $DIR/MACOS_CERT_PASSWORD.txt"
  echo "  NOTARY_KEY_P8_BASE64    = base64 of the AuthKey_XXXX.p8 from App Store Connect   (base64 < AuthKey_XXXX.p8 | tr -d '\\n' | pbcopy)"
  echo "  NOTARY_KEY_ID           = the Key ID of that key"
  echo "  NOTARY_ISSUER_ID        = the Issuer ID shown above the keys list"
  ;;
*)
  sed -n 2,6p "$0" | sed 's/^# \{0,1\}//'
  exit 1
  ;;
esac
