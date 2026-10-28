#!/usr/bin/env bash
# Runs inside the build image started by build/dist.sh: builds the Linux program into dist/$NAME/ (NAME is linux-x64 or
# linux-arm64).
set -euo pipefail
mkdir -p "dist/$NAME"
CGO_ENABLED=1 go build -tags gtk3 -ldflags "-s -w -X main.version=$VERSION -X main.commit=${COMMIT:-}" -o "dist/$NAME/WordProjector" ./cmd/wordprojector
cp LICENSE build/icon.png build/linux/wordprojector.desktop build/linux/README-linux.txt "dist/$NAME/"
