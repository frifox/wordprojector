#!/usr/bin/env bash
# Usage: build/dist.sh <version> [macos] [windows] [linux]      (no system named: all of them)
# Builds the programs into dist/, one folder per build: macos-arm64, macos-x64, windows-x64, windows-arm64, linux-x64.
# macOS and Windows are built here; Linux in a Docker container (it has to be running).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-dev}"
shift || true
WANT=" ${*:-macos windows linux} "
LD="-s -w -X main.version=$VERSION"
mkdir -p dist

macos() {
  for arch in arm64 amd64; do
    name=macos-$([ "$arch" = amd64 ] && echo x64 || echo arm64)
    echo "== $name"
    rm -rf dist/$name && mkdir -p dist/$name
    CGO_ENABLED=1 GOOS=darwin GOARCH=$arch CGO_CFLAGS="-mmacosx-version-min=11.0" CGO_LDFLAGS="-mmacosx-version-min=11.0" \
      go build -ldflags "$LD" -o dist/$name/wordprojector ./cmd/wordprojector
    ./build/mac-app.sh dist/$name/wordprojector "dist/$name/Word Projector.app" "$VERSION"
    codesign --force --deep --sign - "dist/$name/Word Projector.app" || true
    rm dist/$name/wordprojector
  done
}

windows() {
  for arch in amd64 arm64; do
    name=windows-$([ "$arch" = amd64 ] && echo x64 || echo arm64)
    echo "== $name"
    rm -rf dist/$name && mkdir -p dist/$name
    CGO_ENABLED=0 GOOS=windows GOARCH=$arch go build -ldflags "$LD -H windowsgui" -o dist/$name/WordProjector.exe ./cmd/wordprojector
    cp LICENSE dist/$name/LICENSE.txt
  done
}

linux() {
  if ! docker info >/dev/null 2>&1; then
    echo "== linux-x64 skipped: Docker is not running (Linux needs GTK and WebKitGTK, so it is built in a container)"
    return 1
  fi
  echo "== linux-x64 (Docker)"
  rm -rf dist/linux-x64
  # the build image is made once per Go version (the download of the packages is most of the time); the Go build cache
  # lives in a volume, so the next builds only compile what changed
  gover="$(go env GOVERSION)"
  image="wordprojector-linux-build:$gover"
  docker image inspect "$image" >/dev/null 2>&1 || docker build --platform linux/amd64 --build-arg GOVER="$gover" -t "$image" -f build/linux.Dockerfile build
  docker run --rm --platform linux/amd64 -v "$PWD":/src -w /src -v wordprojector-linux-gocache:/root/.cache -e VERSION="$VERSION" "$image" bash build/linux-dist.sh
}

status=0
for os in macos windows linux; do
  case "$WANT" in *" $os "*) $os || status=1;; esac
done
echo
ls -1d dist/*-*/ 2>/dev/null
exit $status
