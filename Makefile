# YSM - Yandere SQL Manager
# "I'll never let your databases go~" <3
#
# Copyright (C) 2025 blubskye
# License: GNU AGPL v3.0

VERSION := 0.2.5
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY := ysm
MAIN := ./cmd/ysm

# Build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)"

# Installation directories
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
MANDIR ?= $(PREFIX)/share/man/man1

# Cross-compilation targets
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean install uninstall test lint deps cross release help

# Default target
all: build

# Build for current platform
build:
	@echo "Building YSM v$(VERSION)... <3"
	go build $(LDFLAGS) -o $(BINARY) $(MAIN)
	@echo "Build complete! YSM is ready to protect your databases~ <3"

# Build with debug symbols
build-debug:
	@echo "Building YSM (debug)..."
	go build -o $(BINARY) $(MAIN)

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY)
	rm -rf dist/
	@echo "Clean~ <3"

# Install to system
install: build
	@echo "Installing YSM to $(BINDIR)... YSM will always be with you~ <3"
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "Installing man page to $(MANDIR)..."
	install -d $(DESTDIR)$(MANDIR)
	install -m 644 ysm.1 $(DESTDIR)$(MANDIR)/ysm.1
	@echo ""
	@echo "Installation complete! YSM is now part of your system~ <3"
	@echo "Run 'ysm --help' or 'man ysm' to get started~"

# Uninstall from system
uninstall:
	@echo "Uninstalling YSM... *sniff* I'll miss you~ <3"
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -f $(DESTDIR)$(MANDIR)/ysm.1
	@echo "YSM has been removed... but I'll be waiting for you to come back~"

# Install to user directory (~/.local)
install-user: build
	@echo "Installing YSM to ~/.local... just for you~ <3"
	install -d $(HOME)/.local/bin
	install -m 755 $(BINARY) $(HOME)/.local/bin/$(BINARY)
	install -d $(HOME)/.local/share/man/man1
	install -m 644 ysm.1 $(HOME)/.local/share/man/man1/ysm.1
	@echo ""
	@echo "Installation complete! <3"
	@echo "Make sure ~/.local/bin is in your PATH~"
	@echo "Run 'ysm --help' or 'man ysm' to get started~"

# Uninstall from user directory
uninstall-user:
	@echo "Uninstalling YSM from ~/.local..."
	rm -f $(HOME)/.local/bin/$(BINARY)
	rm -f $(HOME)/.local/share/man/man1/ysm.1

# Run tests
test:
	@echo "Running tests... YSM wants to make sure everything is perfect~ <3"
	go test -v ./...

# Run linter
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Install dependencies
deps:
	@echo "Downloading dependencies... gathering everything YSM needs~ <3"
	go mod download
	go mod tidy

# Cross-compile for all platforms
cross: clean
	@echo "Building for all platforms... YSM wants to be everywhere~ <3"
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(MAIN) && echo "Built: $(BINARY)-linux-amd64"
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 $(MAIN) && echo "Built: $(BINARY)-linux-arm64"
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 $(MAIN) && echo "Built: $(BINARY)-darwin-amd64"
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(MAIN) && echo "Built: $(BINARY)-darwin-arm64"
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(MAIN) && echo "Built: $(BINARY)-windows-amd64.exe"
	@echo "Cross-compilation complete! <3"

# Create release archives
release: cross
	@echo "Creating release archives... packaging YSM with love~ <3"
	@mkdir -p dist/release
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		name="ysm-$(VERSION)-$$os-$$arch"; \
		mkdir -p "dist/release/$$name"; \
		cp "dist/$(BINARY)-$$os-$$arch$$ext" "dist/release/$$name/$(BINARY)$$ext"; \
		cp ysm.1 "dist/release/$$name/"; \
		cp README.md "dist/release/$$name/"; \
		cp LICENSE "dist/release/$$name/"; \
		cp install.sh "dist/release/$$name/"; \
		if [ "$$os" = "windows" ]; then \
			cd dist/release && zip -r "$$name.zip" "$$name" && cd ../..; \
		else \
			tar -czvf "dist/release/$$name.tar.gz" -C dist/release "$$name"; \
		fi; \
		rm -rf "dist/release/$$name"; \
		echo "Created: $$name"; \
	done
	@echo ""
	@echo "Release archives created in dist/release/ <3"
	@ls -la dist/release/

# Show help
help:
	@echo "YSM - Yandere SQL Manager"
	@echo "\"I'll never let your databases go~\" <3"
	@echo ""
	@echo "Available targets:"
	@echo "  make build        - Build YSM for current platform"
	@echo "  make build-debug  - Build with debug symbols"
	@echo "  make install      - Install to system (requires sudo)"
	@echo "  make uninstall    - Uninstall from system"
	@echo "  make install-user - Install to ~/.local (no sudo needed)"
	@echo "  make uninstall-user - Uninstall from ~/.local"
	@echo "  make test         - Run tests"
	@echo "  make lint         - Run linter"
	@echo "  make deps         - Download dependencies"
	@echo "  make cross        - Cross-compile for all platforms"
	@echo "  make release      - Create release archives"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make help         - Show this help~ <3"
	@echo ""
	@echo "Variables:"
	@echo "  PREFIX=$(PREFIX)"
	@echo "  BINDIR=$(BINDIR)"
	@echo "  MANDIR=$(MANDIR)"
