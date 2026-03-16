#!/bin/bash

# term-ai Installation Script
# Detects OS/arch, fetches the latest release from GitHub, and installs the binary.

set -e

# ── Config ────────────────────────────────────────────────────────────────────
GITHUB_REPO="mholtzhausen/term-ai"
BINARY_NAME="ai"
INSTALL_DIR="/usr/local/bin"
# ─────────────────────────────────────────────────────────────────────────────

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

confirm() {
    echo -ne "${YELLOW}??${NC} $1 [y/N] "
    read response < /dev/tty
    case "$response" in
        [yY][eE][sS]|[yY]) return 0 ;;
        *) return 1 ;;
    esac
}

die() {
    echo -e "${RED}Error:${NC} $1" >&2
    exit 1
}

echo -e "${CYAN}--------------------------------------------------${NC}"
echo -e "${BLUE}        term-ai - Modular Hybrid AI CLI/TUI       ${NC}"
echo -e "${CYAN}--------------------------------------------------${NC}"
echo ""

# ── 1. Detect OS ──────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
    Linux*)  OS_KEY="linux" ;;
    Darwin*) OS_KEY="darwin" ;;
    *)       die "Unsupported operating system: $OS" ;;
esac

# ── 2. Detect Architecture ────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)  ARCH_KEY="amd64" ;;
    arm64|aarch64) ARCH_KEY="arm64" ;;
    *)             die "Unsupported architecture: $ARCH" ;;
esac

ASSET_NAME="${BINARY_NAME}-${OS_KEY}-${ARCH_KEY}"
echo -e "${BLUE}==>${NC} Detected environment: ${CYAN}${OS_KEY}/${ARCH_KEY}${NC}"

# ── Asset existence check ─────────────────────────────────────────────────────
ASSET_API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${LATEST_TAG}"
ASSET_EXISTS=$(${API_GET} "$ASSET_API_URL" | grep '"name": *"${ASSET_NAME}"')
if [ -z "$ASSET_EXISTS" ]; then
    echo -e "${RED}Error:${NC} Release asset '${ASSET_NAME}' not found for tag '${LATEST_TAG}'."
    echo -e "Please check https://github.com/${GITHUB_REPO}/releases for available assets or build from source."
    exit 1
fi

# ── 3. Check for curl or wget ─────────────────────────────────────────────────
if command -v curl &>/dev/null; then
    DOWNLOAD="curl -fsSL"
    API_GET="curl -fsSL"
elif command -v wget &>/dev/null; then
    DOWNLOAD="wget -qO-"
    API_GET="wget -qO-"
else
    die "Neither curl nor wget found. Please install one and retry."
fi

# ── 4. Fetch latest release tag from GitHub API ───────────────────────────────
echo -e "${BLUE}==>${NC} Fetching latest release info from GitHub..."
API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
LATEST_TAG=$(${API_GET} "$API_URL" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    die "Could not determine latest release. Check your network or visit https://github.com/${GITHUB_REPO}/releases"
fi

echo -e "${BLUE}==>${NC} Latest release: ${CYAN}${LATEST_TAG}${NC}"

DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_TAG}/${ASSET_NAME}"
echo -e "${BLUE}==>${NC} Asset URL: ${CYAN}${DOWNLOAD_URL}${NC}"

# ── 5. Confirm and download ───────────────────────────────────────────────────
echo ""
if ! confirm "Download and install ${CYAN}term-ai ${LATEST_TAG}${NC} to ${CYAN}${INSTALL_DIR}/${BINARY_NAME}${NC}?"; then
    echo "Installation cancelled."
    exit 0
fi

TMP_FILE="$(mktemp /tmp/term-ai-XXXXXX)"
trap 'rm -f "$TMP_FILE"' EXIT

echo -e "${BLUE}==>${NC} Downloading..."
if command -v curl &>/dev/null; then
    curl -fSL --progress-bar "$DOWNLOAD_URL" -o "$TMP_FILE" || die "Download failed. Make sure the release has a '${ASSET_NAME}' asset."
else
    wget -q --show-progress "$DOWNLOAD_URL" -O "$TMP_FILE" || die "Download failed. Make sure the release has a '${ASSET_NAME}' asset."
fi

chmod +x "$TMP_FILE"

# ── 6. Install ────────────────────────────────────────────────────────────────
TARGET="${INSTALL_DIR}/${BINARY_NAME}"

if [ -f "$TARGET" ]; then
    EXISTING_VER=$("$TARGET" --version 2>/dev/null | awk '{print $NF}' || echo "unknown")
    echo -e "${YELLOW}Warning:${NC} ${TARGET} already exists (version: ${EXISTING_VER})."
    if ! confirm "Overwrite with ${LATEST_TAG}?"; then
        echo "Installation cancelled."
        exit 0
    fi
fi

if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$TARGET"
else
    echo -e "${BLUE}==>${NC} Elevated privileges required to write to ${INSTALL_DIR}..."
    sudo mv "$TMP_FILE" "$TARGET"
fi

echo ""
echo -e "${GREEN}==>${NC} ${GREEN}Installation successful!${NC}"
echo -e "${CYAN}--------------------------------------------------${NC}"
echo -e "  Installed : ${GREEN}${TARGET}${NC}"
echo -e "  Version   : ${CYAN}${LATEST_TAG}${NC}"
echo -e ""
echo -e "Run '${GREEN}ai${NC}' to get started."
echo -e "Use '${CYAN}ai config set-provider${NC}' to configure your first provider."
echo -e "${CYAN}--------------------------------------------------${NC}"

