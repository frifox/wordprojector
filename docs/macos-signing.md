# Signing and notarizing the macOS app

A release is signed with an Apple **Developer ID Application** certificate and **notarized** by Apple, so macOS opens it
without the user allowing it in System Settings. The release workflow does this by itself once the five secrets below are
set; without them it still builds, but only signs ad hoc (users must right-click → Open the first time).

The scripts: [build/mac-signing-setup.sh](../build/mac-signing-setup.sh) (one-time helpers),
[build/mac-sign.sh](../build/mac-sign.sh) (signs, notarizes, staples, zips: used by the workflow and by `make macos`).

## One-time setup

1. **Apple Developer Program** (individual, $99 a year). Wait for the approval e-mail.
2. **Certificate request.** Already made on this Mac when you read this: the private key is in
   `~/.wordprojector-signing/developer-id.key` and the request in `~/.wordprojector-signing/WordProjector.csr` (a copy is in
   `dist/signing/WordProjector.csr`). **Back up that folder**: the certificate is useless without the key. To make them again
   (only if the folder is gone): `build/mac-signing-setup.sh csr`.
3. **Certificate.** developer.apple.com → Account → Certificates, IDs & Profiles → Certificates → **+** →
   **Developer ID Application** (choose the *G2 Sub-CA*) → upload `WordProjector.csr` → download the `.cer`.
4. **The .p12 for CI.** `build/mac-signing-setup.sh p12 ~/Downloads/developerID_application.cer` checks that the certificate
   belongs to the key, makes `~/.wordprojector-signing/WordProjector-DeveloperID.p12` with a random password and prints
   what to put in the secrets.
5. **Notarization key.** appstoreconnect.apple.com → Users and Access → Integrations → App Store Connect API → Team Keys →
   **+** (access: *Developer*). Download `AuthKey_XXXXXXXXXX.p8` (it can be downloaded once), note the **Key ID** and the
   **Issuer ID** shown above the list.
6. **GitHub secrets.** Repository → Settings → Secrets and variables → Actions → New repository secret:

   | Secret | Value |
   |---|---|
   | `MACOS_CERT_P12_BASE64` | content of `~/.wordprojector-signing/MACOS_CERT_P12_BASE64.txt` |
   | `MACOS_CERT_PASSWORD` | content of `~/.wordprojector-signing/MACOS_CERT_PASSWORD.txt` |
   | `NOTARY_KEY_P8_BASE64` | `base64 < AuthKey_XXXXXXXXXX.p8 \| tr -d '\n'` |
   | `NOTARY_KEY_ID` | the Key ID |
   | `NOTARY_ISSUER_ID` | the Issuer ID |

   (`pbcopy < file` puts a file's content on the clipboard.)
7. **After the first signed release works**, remove the "not notarized, right-click → Open" sentences from the README and from
   the release notes in `.github/workflows/release.yml`.

## Checking a build

```bash
spctl -a -vvv -t exec "Word Projector.app"   # accepted, source=Notarized Developer ID
codesign -dv --verbose=4 "Word Projector.app" # Authority=Developer ID Application: ..., flags=0x10000(runtime)
xcrun stapler validate "Word Projector.app"
```

If Apple rejects the app, the log of `mac-sign.sh` ends with Apple's list of problems (`notarytool log`). The usual ones are an
unsigned or unhardened executable inside the app, or a missing secure timestamp: the script signs with `--options runtime
--timestamp`. The app is a plain Go program with the system web view, so it needs no entitlements.

## Building a signed app on this Mac

Set the same five values as environment variables (the base64 ones from the files above) and run `make macos`: the folders in
`dist/` then hold signed, notarized apps and zips. Without them `make macos` signs ad hoc, as before.

## Never

Never commit the key, the `.p12`, the `.p8` or their passwords. They live in `~/.wordprojector-signing/` and in the GitHub
secrets only.
