# term-ai Makefile

BINARY_NAME=ai
BUILD_DIR=build/bin
INSTALL_PATH=/usr/local/bin/$(BINARY_NAME)

.PHONY: all build clean install uninstall help

all: build

## build: Build the binary to build/bin
build:
	@echo "Building term-ai..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Build binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
build-all:
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	@echo "All cross-platform builds complete."

## clean: Remove build directory
clean:
	@echo "Cleaning up..."
	@rm -rf build
	@rm -f $(BINARY_NAME)

## install: Install the binary by creating a symlink in /usr/local/bin (requires sudo)
install: build
	@echo "Installing term-ai to $(INSTALL_PATH)..."
	@sudo ln -sf $(PWD)/$(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)
	@echo "Installation complete. You can now run '$(BINARY_NAME)' from anywhere."

## uninstall: Remove the symlink from /usr/local/bin (requires sudo)
uninstall:
	@echo "Uninstalling term-ai..."
	@sudo rm -f $(INSTALL_PATH)
	@echo "Uninstalled."

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' Makefile | sed -e 's/## //'
