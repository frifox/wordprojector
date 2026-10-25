#!/usr/bin/env bash
# Runs inside the build image started by build/dist.sh: builds the Linux program into dist/linux-x64/.
set -euo pipefail
mkdir -p dist/linux-x64
CGO_ENABLED=1 go build -tags gtk3 -ldflags "-s -w -X main.version=$VERSION" -o dist/linux-x64/WordProjector ./cmd/wordprojector
cp LICENSE build/icon.png build/linux/wordprojector.desktop build/linux/README-linux.txt dist/linux-x64/
