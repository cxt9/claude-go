# Claude Code Go

**Portable Claude Code environment** - carry your authenticated Claude sessions on a USB device and use them on any computer.

## Features

- **Portable**: Run Claude Code from a USB drive on any machine (Windows, macOS, Linux)
- **Secure**: AES-256 encrypted credential vault with Argon2id key derivation
- **Session Persistence**: Resume conversations across different computers
- **Isolated**: Completely independent from any Claude Code installed on the host machine
- **Self-Updating**: Built-in update mechanism preserves your data
- **MCP Support**: Remote, bundled, and USB-local MCP servers

## Quick Start

### Installation

1. Download the latest release from [Releases](https://github.com/cxt9/claude-go/releases)
2. Extract to your USB drive
3. Run `./launch.sh` (macOS/Linux) or `launch.bat` (Windows)

### First-Time Setup

```
$ ./launch.sh

╭─────────────────────────────────────────────╮
│       Claude Code Go - First Run            │
╰─────────────────────────────────────────────╯

Step 1: Create a master password to protect your credentials
Master password (min 12 chars): ****************
Confirm password: ****************
✓ Vault created

Step 2: Link your Claude account
  [1] Claude.ai account (Pro/Max subscription)
  [2] API Key (Claude Console)
  [3] Amazon Bedrock
  [4] Google Vertex AI

> 1

Opening browser for Claude.ai login...
✓ Authentication successful!

You're all set! Claude Code Go is ready to use.
```

### Using on Another Computer

```
$ ./launch.sh

Unlock your portable vault
Master password: ****************
✓ Vault unlocked

Previous sessions:
  [1] 2h ago - myproject: "Implementing auth flow..."
  [2] 1d ago - api-work: "Fixed pagination bug..."
  [3] Start new session

> 1

Resuming session...
```

## Directory Structure

```
claude-go/
├── bin/                    # Platform-specific binaries
│   ├── darwin-arm64/
│   ├── darwin-amd64/
│   ├── linux-amd64/
│   └── windows-amd64/
├── vault/                  # Encrypted credentials (NEVER SHARE)
├── sessions/               # Your conversation history
├── config/                 # Settings and MCP configuration
├── mcp/                    # MCP servers
│   ├── bundled/           # Ships with Claude Code Go
│   └── user/              # Your installed servers
├── launch.sh / .bat        # Launcher scripts
└── update.sh / .bat        # Update scripts
```

## Security

### Encryption

- Credentials encrypted with **AES-256-GCM**
- Key derived using **Argon2id** (memory-hard, brute-force resistant)
- Each vault has unique random salt

### If Your USB Is Lost

1. Revoke access at [claude.ai/settings](https://claude.ai/settings)
2. Regenerate API keys in [Claude Console](https://console.anthropic.com)

### Recommendations

- Use a strong master password (16+ characters)
- Consider a hardware-encrypted USB drive (IronKey, Apricorn)
- Enable auto-lock in settings

## MCP Servers

Claude Code Go supports three types of MCP servers:

| Type | Description | Portability |
|------|-------------|-------------|
| **Remote** | HTTP/WebSocket servers | Works everywhere |
| **Bundled** | Ships with Claude Code Go | Works everywhere |
| **USB-local** | Installed to USB by user | Works everywhere |
| Host-local | Installed on host machine | Host-specific only |

Configure in `config/settings.json`:

```json
{
  "mcp": {
    "servers": {
      "github": {
        "portability": "remote",
        "type": "http",
        "url": "https://mcp.github.com/v1",
        "credential_ref": "mcp/github-token"
      },
      "sqlite": {
        "portability": "usb-local",
        "type": "stdio",
        "command": "$USB_ROOT/mcp/user/sqlite/server",
        "args": ["--db", "$PROJECT_DIR/data.db"]
      }
    }
  }
}
```

## Updates

Check for updates:

```bash
./update.sh
```

Offline update:

```bash
./update.sh --offline /path/to/claude-go-1.2.0.zip
```

## Building from Source

Requirements: Go 1.22+

```bash
# Clone the repository
git clone https://github.com/cxt9/claude-go.git
cd claude-go

# Build for all platforms
./scripts/build.sh 1.0.0

# Or build for current platform only
go build -o claude-go ./cmd/claude-go
```

## Contributing

Contributions are welcome! Please read the design log at `design-log/001-portable-claude-environment.md` before making significant changes.

## License

MIT License - see [LICENSE](LICENSE) for details.
