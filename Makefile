APP      := Word Projector
BIN      := wordprojector
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse HEAD 2>/dev/null)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
GOFLAGS  :=

# Linux: the window uses GTK 3 and WebKitGTK 4.1
ifeq ($(shell uname -s),Linux)
GOFLAGS  := -tags gtk3
endif

# macOS: build the C parts for the oldest macOS the program supports, so the linker does not warn that they target the newer one of the SDK
ifeq ($(shell uname -s),Darwin)
export CGO_CFLAGS  += -mmacosx-version-min=11.0
export CGO_LDFLAGS += -mmacosx-version-min=11.0
endif

# Where the programs of this computer are built: dist/<system>-<architecture>, the same folders as build/dist.sh makes
# (macos-arm64, macos-x64, windows-x64, windows-arm64, linux-x64, linux-arm64). Nothing is built anywhere else.
HOSTOS   := $(shell go env GOOS)
HOSTARCH := $(shell go env GOARCH)
OUT      := dist/$(if $(filter darwin,$(HOSTOS)),macos,$(HOSTOS))-$(if $(filter amd64,$(HOSTARCH)),x64,$(HOSTARCH))
EXT      := $(if $(filter windows,$(HOSTOS)),.exe)

.PHONY: default frontend all macos windows linux linux-x64 linux-arm64 macos-app install dev clean test
default: macos

# build UI
frontend:
	cd frontend && ([ -d node_modules ] || npm ci) && npm run build

# build all OS
all: frontend
	./build/dist.sh $(VERSION)

# build specific OS
macos windows linux linux-x64 linux-arm64: frontend
	./build/dist.sh $(VERSION) $@

# bundle MacOS app into "dist/macos-<architecture>/Word Projector.app"
macos-app: build
	./build/mac-app.sh $(OUT)/$(BIN) "$(OUT)/$(APP).app" $(VERSION)

# Build the app and install it as "/Applications/Word Projector.app", replacing an earlier copy (macOS)
install: macos-app
	-osascript -e 'tell application "$(APP)" to quit' >/dev/null 2>&1; sleep 1; pkill -x WordProjector 2>/dev/null; true
	rm -rf "/Applications/$(APP).app"
	cp -R "$(OUT)/$(APP).app" "/Applications/$(APP).app"
	rm -rf "$(OUT)/$(APP).app"
	@echo "installed /Applications/$(APP).app"

# headless backend on :8777, Vite dev server with hot reload on :5173
dev: frontend
	go build -tags headless -ldflags "$(LDFLAGS)" -o $(OUT)/$(BIN)-headless$(EXT) ./cmd/wordprojector
	./$(OUT)/$(BIN)-headless$(EXT) --listen 127.0.0.1:8777 & cd frontend && npm run dev

# clean /dist folder (except macos-signing, which holds macOS signing keys and certificates)
clean:
	rm -rf frontend/dist internal/server/dist/assets
	@if [ -d dist ]; then find dist -mindepth 1 -maxdepth 1 ! -name macos-signing -exec rm -rf {} +; fi

test:
	go test $(GOFLAGS) ./...
	cd frontend && npx tsc -b