#!/bin/bash

# MHAI Interactive Installation Script
# This script installs MHAI by cloning the repository, building the Go binary,
# and moving it to /usr/local/bin.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper for confirmations
confirm() {
    local prompt="$1"
    local default="${2:-N}"
    # Use /dev/tty for input to allow curl | bash patterns
    echo -ne "${YELLOW}??${NC} $prompt [y/N] "
    read response < /dev/tty
    case "$response" in
        [yY][eE][sS]|[yY]) 
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

echo -e "${CYAN}--------------------------------------------------${NC}"
echo -e "${BLUE}        MHAI - Modular Hybrid AI CLI/TUI          ${NC}"
echo -e "${CYAN}--------------------------------------------------${NC}"
echo -e "Disclaimer: This software was 99% coded by AI. Use at own risk.\n"

# 1. Dependency Check
echo -e "${BLUE}==>${NC} Verifying system dependencies..."
missing_deps=()
for cmd in go git make; do
    if ! command -v "$cmd" &> /dev/null; then
        missing_deps+=("$cmd")
    fi
done

if [ ${#missing_deps[@]} -ne 0 ]; then
    echo -e "${RED}Error:${NC} The following dependencies are missing: ${RED}${missing_deps[*]}${NC}"
    echo -e "Please install them using your package manager (e.g., sudo apt install go git make) and try again."
    exit 1
fi
echo -e "${GREEN}Check passed!${NC} (go, git, make found)\n"

# 2. Source Selection
INSTALL_TMP="/tmp/mhai-install-$(date +%s)"
repo_url="https://github.com/mholtzhausen/term-ai.git"

if [ -f "go.mod" ] && grep -q "github.com/mhai-org/mhai" go.mod; then
    echo -e "${BLUE}==>${NC} Detected MHAI source directory at: ${CYAN}$(pwd)${NC}"
    if confirm "Build and install from current directory?"; then
        REPO_DIR="."
    else
        echo -e "Installation cancelled."
        exit 0
    fi
else
    echo -e "${BLUE}==>${NC} No local source detected. MHAI will be cloned to a temporary directory."
    if confirm "Clone repository from ${CYAN}$repo_url${NC}?"; then
        echo -e "${BLUE}==>${NC} Cloning..."
        git clone "$repo_url" "$INSTALL_TMP"
        REPO_DIR="$INSTALL_TMP"
    else
        echo -e "Installation cancelled."
        exit 0
    fi
fi

cd "$REPO_DIR"

# 3. Build Step
echo -e "\n${BLUE}==>${NC} Next: Building the binary using 'make build'..."
if confirm "Proceed with build?"; then
    make build
else
    echo -e "Installation cancelled."
    exit 0
fi

# 4. Binary Installation
target="/usr/local/bin/ai"
echo -e "\n${BLUE}==>${NC} Final Step: Installing binary to ${CYAN}$target${NC}"

if [ -f "$target" ]; then
    echo -e "${YELLOW}Warning:${NC} $target already exists."
    if ! confirm "Overwrite existing binary?"; then
        echo -e "Installation cancelled. You can find the built binary at ${CYAN}$(pwd)/build/bin/ai${NC}"
        exit 0
    fi
fi

if [ -w "/usr/local/bin" ]; then
    cp build/bin/ai "$target"
else
    echo -e "${BLUE}==>${NC} Elevated privileges required for /usr/local/bin..."
    sudo cp build/bin/ai "$target"
fi

echo -e "\n${GREEN}==>${NC} ${GREEN}Installation successful!${NC}"
echo -e "${CYAN}--------------------------------------------------${NC}"
echo -e "You can now run '${GREEN}ai${NC}' from your terminal."
echo -e "Use '${CYAN}ai config set-provider${NC}' to get started."
echo -e "${CYAN}--------------------------------------------------${NC}"

# Cleanup
if [ "$REPO_DIR" == "$INSTALL_TMP" ]; then
    rm -rf "$INSTALL_TMP"
fi
