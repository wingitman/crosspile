BINARY      := crosspile
INSTALL_DIR := $(HOME)/.local/bin
BUILD_DIR   := bin

.PHONY: all build install uninstall test clean run

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) .
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"
	@echo "Config: $$(go env GOOS 2>/dev/null | grep -q windows && echo %APPDATA%\\delbysoft\\crossfile.toml || echo ~/.config/delbysoft/crossfile.toml)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

test:
	go test ./... -timeout 30s

run:
	go run .

clean:
	rm -rf $(BUILD_DIR)
