#!/bin/bash
set -e

# Claude Code Go - Cross-platform build script

VERSION="${1:-dev}"
OUTPUT_DIR="${2:-dist}"

echo "Building Claude Code Go v${VERSION}..."

# Platforms to build for
PLATFORMS=(
    "darwin/arm64"
    "darwin/amd64"
    "linux/amd64"
    "windows/amd64"
)

# Create output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    PLATFORM_DIR="${GOOS}-${GOARCH}"
    BIN_NAME="claude-go"

    if [ "$GOOS" = "windows" ]; then
        BIN_NAME="claude-go.exe"
    fi

    echo "Building for ${PLATFORM_DIR}..."

    OUTPUT_PATH="${OUTPUT_DIR}/bin/${PLATFORM_DIR}/${BIN_NAME}"
    mkdir -p "$(dirname "$OUTPUT_PATH")"

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o "$OUTPUT_PATH" \
        ./cmd/claude-go
done

# Copy launcher scripts
echo "Copying launcher scripts..."
cp scripts/launch.sh "$OUTPUT_DIR/"
cp scripts/launch.bat "$OUTPUT_DIR/"
cp scripts/update.sh "$OUTPUT_DIR/"
cp scripts/update.bat "$OUTPUT_DIR/"
chmod +x "$OUTPUT_DIR"/*.sh

# Create directory structure
mkdir -p "$OUTPUT_DIR/config"
mkdir -p "$OUTPUT_DIR/vault"
mkdir -p "$OUTPUT_DIR/sessions"
mkdir -p "$OUTPUT_DIR/cache"
mkdir -p "$OUTPUT_DIR/mcp/bundled"
mkdir -p "$OUTPUT_DIR/mcp/user"

# Copy default config
cat > "$OUTPUT_DIR/config/settings.json" << 'EOF'
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

# Create version file
cat > "$OUTPUT_DIR/.version" << EOF
{"version":"${VERSION}","built_at":"$(date -u +%Y-%m-%dT%H:%M:%SZ)"}
EOF

# Create README
cat > "$OUTPUT_DIR/README.md" << 'EOF'
# Claude Code Go

Portable Claude Code environment for use on any machine.

## Quick Start

1. Run `./launch.sh` (macOS/Linux) or `launch.bat` (Windows)
2. Create your master password
3. Authenticate with Claude
4. Start coding!

## Files

- `launch.sh/bat` - Start Claude Code Go
- `update.sh/bat` - Check for and install updates
- `vault/` - Encrypted credentials (never share!)
- `sessions/` - Your conversation history
- `config/` - Settings and MCP configuration

## Security

Your credentials are encrypted with AES-256. Never share your master password.

If you lose your USB device, revoke access at https://claude.ai/settings
EOF

echo ""
echo "Build complete! Output in ${OUTPUT_DIR}/"
echo ""
echo "To create a release zip:"
echo "  cd ${OUTPUT_DIR} && zip -r ../claude-go-${VERSION}.zip ."
