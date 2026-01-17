#!/bin/bash
# Claude Code Go - Setup Script
# Run this after cloning to build the binaries

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo ""
echo "╭─────────────────────────────────────────────╮"
echo "│       Claude Code Go - Setup                │"
echo "╰─────────────────────────────────────────────╯"
echo ""

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed."
    echo ""
    echo "Please install Go first:"
    echo "  macOS:   brew install go"
    echo "  Linux:   sudo apt install golang  (or your package manager)"
    echo "  Windows: Download from https://go.dev/dl/"
    echo ""
    exit 1
fi

echo "Go version: $(go version)"
echo ""

# Detect current platform
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
    MINGW*|MSYS*|CYGWIN*)
        PLATFORM="windows-amd64"
        ;;
    *)
        echo "Unsupported platform: $(uname -s)-$(uname -m)"
        exit 1
        ;;
esac

echo "Detected platform: ${PLATFORM}"
echo ""

# Ask if user wants to build for all platforms or just current
echo "Build options:"
echo "  [1] Build for current platform only (faster)"
echo "  [2] Build for all platforms (portable to any machine)"
echo ""
read -p "Choose [1/2]: " BUILD_CHOICE

if [ "$BUILD_CHOICE" = "2" ]; then
    PLATFORMS=("darwin-arm64" "darwin-amd64" "linux-amd64" "windows-amd64")
else
    PLATFORMS=("$PLATFORM")
fi

# Build
echo ""
echo "Building..."

for PLAT in "${PLATFORMS[@]}"; do
    GOOS="${PLAT%-*}"
    GOARCH="${PLAT#*-}"

    BIN_NAME="claude-go"
    if [ "$GOOS" = "windows" ]; then
        BIN_NAME="claude-go.exe"
    fi

    echo "  Building for ${PLAT}..."
    mkdir -p "bin/${PLAT}"

    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-s -w" -o "bin/${PLAT}/${BIN_NAME}" ./cmd/claude-go
done

# Create directories
echo ""
echo "Creating directories..."
mkdir -p config vault sessions cache mcp/bundled mcp/user

# Create default config if not exists
if [ ! -f "config/settings.json" ]; then
    cat > config/settings.json << 'EOF'
{
  "version": "1.0",
  "vault": {
    "auto_lock_minutes": 15,
    "require_password_on_resume": true
  },
  "sessions": {
    "cleanup_period_days": 30,
    "max_sessions": 100,
    "auto_save_seconds": 30
  },
  "environment": {
    "paranoid_mode": false,
    "cleanup_on_exit": true,
    "default_model": "claude-sonnet-4-20250514"
  },
  "updates": {
    "auto_check": true,
    "channel": "stable"
  },
  "mcp": {
    "servers": {}
  }
}
EOF
fi

# Create version file
echo '{"version":"dev","built_at":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"}' > .version

# Make scripts executable
chmod +x scripts/*.sh launch.sh 2>/dev/null || true

# Copy launch scripts to root if not there
if [ ! -f "launch.sh" ]; then
    cp scripts/launch.sh .
fi
if [ ! -f "launch.bat" ]; then
    cp scripts/launch.bat .
fi
if [ ! -f "update.sh" ]; then
    cp scripts/update.sh .
fi
if [ ! -f "update.bat" ]; then
    cp scripts/update.bat .
fi

echo ""
echo "╭─────────────────────────────────────────────╮"
echo "│            Setup Complete!                  │"
echo "╰─────────────────────────────────────────────╯"
echo ""
echo "To start Claude Code Go, run:"
echo ""
echo "  ./launch.sh"
echo ""
echo "Your USB drive is now portable to any machine"
if [ "$BUILD_CHOICE" = "2" ]; then
    echo "running macOS, Linux, or Windows."
else
    echo "running ${PLATFORM}."
    echo ""
    echo "To make it work on other platforms, run setup.sh"
    echo "again and choose option 2."
fi
echo ""
