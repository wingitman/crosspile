BINARY       := crosspile
INSTALL_DIR  ?= $(HOME)/.local/bin
BUILD_DIR    := bin
RELEASES_DIR := releases

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
ORIGIN := $(shell git remote get-url origin 2>/dev/null || echo https://github.com/wingitman/crosspile.git)
LDFLAGS := -s -w -X 'main.version=$(COMMIT)' -X 'main.origin=$(ORIGIN)' -X 'main.repoDir=$(shell pwd)'

.PHONY: all build build-all install uninstall test clean run

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

build-all:
	@mkdir -p $(RELEASES_DIR)/linux/amd64 $(RELEASES_DIR)/linux/arm64 $(RELEASES_DIR)/darwin/amd64 $(RELEASES_DIR)/darwin/arm64 $(RELEASES_DIR)/windows
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/linux/amd64/$(BINARY) .
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/linux/arm64/$(BINARY) .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/darwin/amd64/$(BINARY) .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/darwin/arm64/$(BINARY) .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(RELEASES_DIR)/windows/$(BINARY).exe .
	@echo "Pre-built binaries written to $(RELEASES_DIR)/"

install:
	@mkdir -p $(INSTALL_DIR)
	@SOURCE_ROOT="$$(pwd)"; \
	REMOTE_URL="$$(git -C "$$SOURCE_ROOT" remote get-url origin 2>/dev/null || echo "$(ORIGIN)")"; \
	if [ -d "$$SOURCE_ROOT/.git" ]; then \
		echo "==> Checking git remote for updates..."; \
		REMOTE_SHA=""; \
		if GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never git -C "$$SOURCE_ROOT" fetch origin --quiet 2>/dev/null; then \
			REMOTE_SHA="$$(git -C "$$SOURCE_ROOT" rev-parse '@{u}' 2>/dev/null || git -C "$$SOURCE_ROOT" rev-parse origin/HEAD 2>/dev/null || git -C "$$SOURCE_ROOT" rev-parse origin/main 2>/dev/null || git -C "$$SOURCE_ROOT" rev-parse origin/master 2>/dev/null)"; \
		fi; \
		if [ -z "$$REMOTE_SHA" ]; then \
			REMOTE_SHA="$$(GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never git ls-remote "$$REMOTE_URL" HEAD 2>/dev/null | cut -f1)"; \
		fi; \
		LOCAL_SHA="$$(git -C "$$SOURCE_ROOT" rev-parse HEAD 2>/dev/null || true)"; \
		if [ -n "$$REMOTE_SHA" ] && [ -n "$$LOCAL_SHA" ] && ! echo "$$REMOTE_SHA" | grep -q "^$$LOCAL_SHA"; then \
			echo "    Local : $$(echo $$LOCAL_SHA | cut -c1-7)"; \
			echo "    Remote: $$(echo $$REMOTE_SHA | cut -c1-7)"; \
			printf "    Pull latest changes before installing? [Y/n] "; \
			read pull_choice; \
			if [ -z "$$pull_choice" ] || echo "$$pull_choice" | grep -qi '^y'; then \
				GCM_INTERACTIVE=never git -C "$$SOURCE_ROOT" pull || { echo "ERROR: git pull failed. Aborting install."; exit 1; }; \
			fi; \
		else \
			echo "    Already up to date or remote unavailable."; \
		fi; \
	fi; \
	INSTALL_COMMIT="$$(git -C "$$SOURCE_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"; \
	INSTALL_ORIGIN="$$(git -C "$$SOURCE_ROOT" remote get-url origin 2>/dev/null || echo "$(ORIGIN)")"; \
	INSTALL_LDFLAGS="-s -w -X 'main.version=$$INSTALL_COMMIT' -X 'main.origin=$$INSTALL_ORIGIN' -X 'main.repoDir=$$SOURCE_ROOT'"; \
	if command -v go >/dev/null 2>&1; then \
		echo "==> Building from source..."; \
		BUILD_OUT="$$(mktemp "$${TMPDIR:-/tmp}/crosspile.XXXXXX")"; \
		trap 'rm -f "$$BUILD_OUT"' EXIT; \
		go -C "$$SOURCE_ROOT" build -ldflags="$$INSTALL_LDFLAGS" -o "$$BUILD_OUT" . || exit 1; \
		cp "$$BUILD_OUT" "$(INSTALL_DIR)/$(BINARY)"; \
	else \
		echo "==> Go not found - installing pre-built binary..."; \
		OS="$$(uname -s | tr '[:upper:]' '[:lower:]')"; ARCH="$$(uname -m)"; \
		case "$$ARCH" in x86_64|amd64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; *) echo "ERROR: unsupported architecture: $$ARCH"; exit 1 ;; esac; \
		if [ "$$OS" = "darwin" ]; then RELEASE_BIN="$$SOURCE_ROOT/$(RELEASES_DIR)/darwin/$$ARCH/$(BINARY)"; elif [ "$$OS" = "linux" ]; then RELEASE_BIN="$$SOURCE_ROOT/$(RELEASES_DIR)/linux/$$ARCH/$(BINARY)"; else echo "ERROR: unsupported OS: $$OS"; exit 1; fi; \
		if [ ! -f "$$RELEASE_BIN" ]; then echo "ERROR: missing pre-built binary: $$RELEASE_BIN"; exit 1; fi; \
		cp "$$RELEASE_BIN" "$(INSTALL_DIR)/$(BINARY)"; \
		chmod +x "$(INSTALL_DIR)/$(BINARY)"; \
	fi; \
	PLATFORM="$$(uname -s | tr '[:upper:]' '[:lower:]')/$$(uname -m)"; \
	"$(INSTALL_DIR)/$(BINARY)" --set-repo-dir "$$SOURCE_ROOT" "$$INSTALL_ORIGIN" "$$INSTALL_COMMIT" "git" "$$PLATFORM" || echo "Warning: could not write install metadata. Auto-updates may not work."
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"
	@echo "Run: $(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

test:
	go test ./... -timeout 30s

run:
	go run .

clean:
	rm -rf $(BUILD_DIR) $(RELEASES_DIR)
