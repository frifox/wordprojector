#!/usr/bin/env bash
# Usage: build/dist.sh <version> [macos] [windows] [linux] [linux-x64] [linux-arm64]   (nothing named: all of them)
# Builds the programs into dist/, one folder per build: macos-arm64, macos-x64, windows-x64, windows-arm64, linux-x64, linux-arm64.
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

# one Linux build: name, Docker platform, and the name of the Go download for that architecture
linux_build() {
  local name=$1 platform=$2 gotar=$3
  echo "== $name (Docker, $platform)"
  rm -rf "dist/$name"
  # the build image is made once per Go version and architecture (the download of the packages is most of the time); the
  # Go build cache lives in a volume, so the next builds only compile what changed
  gover="$(go env GOVERSION)"
  image="wordprojector-$name-build:$gover"
  docker image inspect "$image" >/dev/null 2>&1 || docker build --platform "$platform" --build-arg GOVER="$gover" --build-arg GOTAR="$gotar" -t "$image" -f build/linux.Dockerfile build
  docker run --rm --platform "$platform" -v "$PWD":/src -w /src -v "wordprojector-$name-gocache:/root/.cache" -e NAME="$name" -e VERSION="$VERSION" "$image" bash build/linux-dist.sh
}

# Linux: x64 and arm64 (also a Raspberry Pi with a 64-bit system), each in a container of its own architecture. "linux" builds
# both; linux-x64 or linux-arm64 only that one. (There is no 32-bit ARM build: Wails does not compile for it.)
linux() {
  if ! docker info >/dev/null 2>&1; then
    echo "== linux skipped: Docker is not running (Linux needs GTK and WebKitGTK, so it is built in a container)"
    return 1
  fi
  local all=0
  case "$WANT" in *" linux "*) all=1;; esac
  if [ $all = 1 ] || [[ "$WANT" == *" linux-x64 "* ]]; then linux_build linux-x64 linux/amd64 linux-amd64; fi
  if [ $all = 1 ] || [[ "$WANT" == *" linux-arm64 "* ]]; then linux_build linux-arm64 linux/arm64 linux-arm64; fi
}

status=0
for os in macos windows linux; do
  case "$WANT" in *" $os "*|*" $os-"*) $os || status=1;; esac
done
echo
ls -1d dist/*-*/ 2>/dev/null
exit $status
