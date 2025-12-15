#!/bin/bash
# YSM - Yandere SQL Manager
# Installation Script
# "I'll never let your databases go~" <3
#
# Copyright (C) 2025 blubskye
# License: GNU AGPL v3.0

set -e

VERSION="0.2.2"
BINARY="ysm"

# Colors for pretty output <3
RED='\033[0;31m'
GREEN='\033[0;32m'
PINK='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${PINK}"
cat << 'EOF'
 __   __ ____  __  __
 \ \ / // ___||  \/  |
  \ V / \___ \| |\/| |
   | |   ___) | |  | |
   |_|  |____/|_|  |_|

 Yandere SQL Manager
 "I'll never let your databases go~" <3
EOF
echo -e "${NC}"

echo "YSM v${VERSION} Installation Script"
echo "=================================="
echo ""

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize architecture
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo "Detected: ${OS}/${ARCH}"
echo ""

# Check if running as root for system install
INSTALL_TYPE="user"
BINDIR="$HOME/.local/bin"
MANDIR="$HOME/.local/share/man/man1"

if [ "$EUID" -eq 0 ]; then
    INSTALL_TYPE="system"
    BINDIR="/usr/local/bin"
    MANDIR="/usr/local/share/man/man1"
fi

# Allow override via arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --system)
            if [ "$EUID" -ne 0 ]; then
                echo -e "${RED}Error: --system requires root privileges (use sudo)${NC}"
                exit 1
            fi
            INSTALL_TYPE="system"
            BINDIR="/usr/local/bin"
            MANDIR="/usr/local/share/man/man1"
            shift
            ;;
        --user)
            INSTALL_TYPE="user"
            BINDIR="$HOME/.local/bin"
            MANDIR="$HOME/.local/share/man/man1"
            shift
            ;;
        --prefix)
            BINDIR="$2/bin"
            MANDIR="$2/share/man/man1"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --system     Install system-wide (requires sudo)"
            echo "  --user       Install to ~/.local (default for non-root)"
            echo "  --prefix DIR Install to custom prefix"
            echo "  --help       Show this help~ <3"
            echo ""
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

echo "Installation type: ${INSTALL_TYPE}"
echo "Binary directory:  ${BINDIR}"
echo "Man page directory: ${MANDIR}"
echo ""

# Check if binary exists in current directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "${SCRIPT_DIR}/${BINARY}" ]; then
    BINARY_PATH="${SCRIPT_DIR}/${BINARY}"
elif [ -f "${SCRIPT_DIR}/${BINARY}.exe" ]; then
    BINARY_PATH="${SCRIPT_DIR}/${BINARY}.exe"
else
    echo -e "${RED}Error: Binary not found in ${SCRIPT_DIR}${NC}"
    echo "Please run this script from the release directory~"
    exit 1
fi

# Check for man page
MANPAGE_PATH=""
if [ -f "${SCRIPT_DIR}/ysm.1" ]; then
    MANPAGE_PATH="${SCRIPT_DIR}/ysm.1"
fi

# Create directories
echo -e "${PINK}Creating directories...${NC}"
mkdir -p "$BINDIR"
mkdir -p "$MANDIR"

# Install binary
echo -e "${PINK}Installing ${BINARY}...${NC}"
cp "$BINARY_PATH" "$BINDIR/$BINARY"
chmod 755 "$BINDIR/$BINARY"
echo -e "${GREEN}  Installed: $BINDIR/$BINARY${NC}"

# Install man page
if [ -n "$MANPAGE_PATH" ]; then
    echo -e "${PINK}Installing man page...${NC}"
    cp "$MANPAGE_PATH" "$MANDIR/ysm.1"
    chmod 644 "$MANDIR/ysm.1"
    echo -e "${GREEN}  Installed: $MANDIR/ysm.1${NC}"
fi

echo ""
echo -e "${GREEN}Installation complete! YSM is now part of your system~ <3${NC}"
echo ""

# Check if BINDIR is in PATH
if [[ ":$PATH:" != *":$BINDIR:"* ]]; then
    echo -e "${PINK}Note: $BINDIR is not in your PATH.${NC}"
    echo ""
    echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo ""
    echo "  export PATH=\"\$PATH:$BINDIR\""
    echo ""
fi

echo "Get started with:"
echo "  ysm --help     # Show help"
echo "  ysm            # Start TUI"
echo "  man ysm        # Read the manual~ <3"
echo ""
echo -e "${PINK}Your databases are safe with YSM... forever~ <3${NC}"
