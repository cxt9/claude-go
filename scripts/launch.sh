#!/bin/bash
# Claude Code Go - Portable Launcher (macOS/Linux)

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

# Path to the launcher binary
LAUNCHER="${SCRIPT_DIR}/bin/${PLATFORM}/claude-go"

# Check if binary exists
if [ ! -f "$LAUNCHER" ]; then
    echo "Error: Launcher binary not found at ${LAUNCHER}"
    echo "Please ensure Claude Code Go is properly installed."
    exit 1
fi

# Make sure it's executable
chmod +x "$LAUNCHER"

# Run the launcher
exec "$LAUNCHER" "$@"
