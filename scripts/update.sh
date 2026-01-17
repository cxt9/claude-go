#!/bin/bash
# Claude Code Go - Update Script (macOS/Linux)

set -e

# Get the directory where this script is located (USB root)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Detect platform
case "$(uname -s)-$(uname -m)" in
    Darwin-arm64)
        PLATFORM="darwin-arm64"
        ;;
    Darwin-x86_64)
        PLATFORM="darwin-amd64"
        ;;
    Linux-x86_64)
        PLATFORM="linux-amd64"
        ;;
    *)
        echo "Unsupported platform: $(uname -s)-$(uname -m)"
        exit 1
        ;;
esac

# Configuration
REPO="cxt9/claude-go"
MANIFEST_URL="https://github.com/${REPO}/releases/latest/download/manifest.json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo ""
echo "╭─────────────────────────────────────────────╮"
echo "│       Claude Code Go - Update Check         │"
echo "╰─────────────────────────────────────────────╯"
echo ""

# Read current version
if [ -f "${SCRIPT_DIR}/.version" ]; then
    CURRENT_VERSION=$(cat "${SCRIPT_DIR}/.version" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
else
    CURRENT_VERSION="0.0.0"
fi

echo "Current version: ${CURRENT_VERSION}"
echo ""

# Handle offline update
if [ "$1" = "--offline" ] && [ -n "$2" ]; then
    echo "Applying offline update from $2..."

    if [ ! -f "$2" ]; then
        echo -e "${RED}Error: File not found: $2${NC}"
        exit 1
    fi

    # Create backup
    echo "Creating backup..."
    rm -rf "${SCRIPT_DIR}/.rollback"
    cp -r "${SCRIPT_DIR}/bin" "${SCRIPT_DIR}/.rollback"

    # Extract update
    echo "Extracting update..."
    unzip -o "$2" -d "${SCRIPT_DIR}" bin/* *.sh *.bat 2>/dev/null || true

    # Cleanup
    rm -rf "${SCRIPT_DIR}/.rollback"
    rm -rf "${SCRIPT_DIR}/cache"
    mkdir -p "${SCRIPT_DIR}/cache"

    echo -e "${GREEN}✓ Offline update applied${NC}"
    exit 0
fi

# Check for updates
echo "Checking for updates..."

# Fetch manifest
MANIFEST=$(curl -fsSL "$MANIFEST_URL" 2>/dev/null) || {
    echo -e "${RED}Error: Could not check for updates. No internet connection?${NC}"
    exit 1
}

LATEST_VERSION=$(echo "$MANIFEST" | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
CHANGELOG=$(echo "$MANIFEST" | grep -o '"changelog":\[[^]]*\]' | sed 's/"changelog":\[//;s/\]//;s/","/\n  • /g;s/"//g')

if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}Error: Could not parse version from manifest${NC}"
    exit 1
fi

# Compare versions
version_gt() {
    test "$(echo "$@" | tr " " "\n" | sort -V | head -n 1)" != "$1"
}

if version_gt "$LATEST_VERSION" "$CURRENT_VERSION"; then
    echo -e "${GREEN}✓ New version available: ${LATEST_VERSION}${NC}"
    echo ""
    echo "Changes:"
    echo "  • ${CHANGELOG}"
    echo ""

    # Get download URL and size
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${LATEST_VERSION}/claude-go-${LATEST_VERSION}-${PLATFORM}.zip"

    # Confirm update
    read -p "Proceed with update? [Y/n] " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        # Create backup
        echo "Backing up current version..."
        rm -rf "${SCRIPT_DIR}/.rollback"
        cp -r "${SCRIPT_DIR}/bin" "${SCRIPT_DIR}/.rollback"

        # Download update
        echo "Downloading update..."
        TEMP_FILE=$(mktemp)

        if curl -fL --progress-bar "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
            # Extract
            echo "Installing update..."
            unzip -o "$TEMP_FILE" -d "${SCRIPT_DIR}" bin/* *.sh *.bat 2>/dev/null || true

            # Update version file
            echo "{\"version\":\"${LATEST_VERSION}\",\"updated_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" > "${SCRIPT_DIR}/.version"

            # Cleanup
            rm -f "$TEMP_FILE"
            rm -rf "${SCRIPT_DIR}/.rollback"
            rm -rf "${SCRIPT_DIR}/cache"
            mkdir -p "${SCRIPT_DIR}/cache"

            echo ""
            echo -e "${GREEN}✓ Update complete! Now on version ${LATEST_VERSION}${NC}"
            echo "Your credentials, sessions, and settings were preserved."
        else
            echo -e "${RED}Download failed. Restoring backup...${NC}"
            rm -rf "${SCRIPT_DIR}/bin"
            mv "${SCRIPT_DIR}/.rollback" "${SCRIPT_DIR}/bin"
            rm -f "$TEMP_FILE"
            exit 1
        fi
    else
        echo "Update cancelled."
    fi
else
    echo -e "${GREEN}✓ Already up to date (${CURRENT_VERSION})${NC}"
fi
