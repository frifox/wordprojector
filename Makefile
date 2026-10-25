APP      := Word Projector
BIN      := wordprojector
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  :=
# Linux: the window uses GTK 3 and WebKitGTK 4.1
ifeq ($(shell uname -s),Linux)
GOFLAGS  := -tags gtk3
endif

# macOS: build the C parts for the oldest macOS the program supports, so the linker does not warn that they target the
# newer one of the SDK
ifeq ($(shell uname -s),Darwin)
export CGO_CFLAGS  += -mmacosx-version-min=11.0
export CGO_LDFLAGS += -mmacosx-version-min=11.0
endif

.PHONY: all frontend build headless run dev test clean mac-app install dist macos windows linux

all: build

frontend:
	cd frontend && ([ -d node_modules ] || npm ci) && npm run build

build: frontend
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/wordprojector

headless:
	go build -tags headless -ldflags "$(LDFLAGS)" -o bin/$(BIN)-headless ./cmd/wordprojector

# Run the desktop app from source (requires a built frontend).
run: build
	./bin/$(BIN)

# Development: Go server on :8777 plus Vite dev server with hot reload on :5173.
dev:
	go build -tags headless -o bin/$(BIN)-headless ./cmd/wordprojector
	./bin/$(BIN)-headless --listen 127.0.0.1:8777 & cd frontend && npm run dev

test:
	go test $(GOFLAGS) ./...
	cd frontend && npx tsc -b

clean:
	rm -rf bin frontend/dist internal/server/dist/assets

# Bundle bin/wordprojector into "dist/Word Projector.app" (macOS)
mac-app: build
	./build/mac-app.sh bin/$(BIN) "dist/$(APP).app" $(VERSION)

# Build the app and install it as "/Applications/Word Projector.app", replacing an earlier copy (macOS)
install: mac-app
	-osascript -e 'tell application "$(APP)" to quit' >/dev/null 2>&1; sleep 1; pkill -x WordProjector 2>/dev/null; true
	rm -rf "/Applications/$(APP).app"
	cp -R "dist/$(APP).app" "/Applications/$(APP).app"
	rm -rf "dist/$(APP).app"
	@echo "installed /Applications/$(APP).app"

# The program of every system in dist/, a folder for each: macos-arm64, macos-x64, windows-x64, windows-arm64, linux-x64
# (Linux is built in Docker, which has to be running). The version is the git tag, or VERSION=... to set it.
dist: frontend
	./build/dist.sh $(VERSION)

# One system at a time (the same folders in dist/)
macos windows linux: frontend
	./build/dist.sh $(VERSION) $@
