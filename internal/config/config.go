package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config represents the portable Claude Code Go configuration
type Config struct {
	Version string `json:"version"`

	// Vault settings
	Vault VaultConfig `json:"vault"`

	// Session settings
	Sessions SessionConfig `json:"sessions"`

	// Environment settings
	Environment EnvironmentConfig `json:"environment"`

	// Update settings
	Updates UpdateConfig `json:"updates"`

	// MCP server configuration
	MCP MCPConfig `json:"mcp"`
}

// VaultConfig contains vault-related settings
type VaultConfig struct {
	AutoLockMinutes         int  `json:"auto_lock_minutes"`
	RequirePasswordOnResume bool `json:"require_password_on_resume"`
}

// SessionConfig contains session-related settings
type SessionConfig struct {
	CleanupPeriodDays int `json:"cleanup_period_days"`
	MaxSessions       int `json:"max_sessions"`
	AutoSaveSeconds   int `json:"auto_save_seconds"`
}

// EnvironmentConfig contains runtime environment settings
type EnvironmentConfig struct {
	ParanoidMode  bool   `json:"paranoid_mode"`
	CleanupOnExit bool   `json:"cleanup_on_exit"`
	DefaultModel  string `json:"default_model"`
}

// UpdateConfig contains update-related settings
type UpdateConfig struct {
	AutoCheck     bool       `json:"auto_check"`
	Channel       string     `json:"channel"` // stable, beta, nightly
	PinnedVersion string     `json:"pinned_version,omitempty"`
	LastCheck     *time.Time `json:"last_check,omitempty"`
}

// MCPConfig contains MCP server configuration
type MCPConfig struct {
	Servers map[string]MCPServer `json:"servers"`
}

// MCPServer represents a single MCP server configuration
type MCPServer struct {
	Portability   string            `json:"portability"` // remote, bundled, usb-local, host-local
	Type          string            `json:"type"`        // stdio, http, websocket
	URL           string            `json:"url,omitempty"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	Required      bool              `json:"required"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Version: "1.0",
		Vault: VaultConfig{
			AutoLockMinutes:         15,
			RequirePasswordOnResume: true,
		},
		Sessions: SessionConfig{
			CleanupPeriodDays: 30,
			MaxSessions:       100,
			AutoSaveSeconds:   30,
		},
		Environment: EnvironmentConfig{
			ParanoidMode:  false,
			CleanupOnExit: true,
			DefaultModel:  "claude-sonnet-4-20250514",
		},
		Updates: UpdateConfig{
			AutoCheck: true,
			Channel:   "stable",
		},
		MCP: MCPConfig{
			Servers: map[string]MCPServer{
				"filesystem": {
					Portability: "bundled",
					Type:        "stdio",
					Command:     "$USB_ROOT/mcp/bundled/filesystem/server",
					Args:        []string{"--root", "$PROJECT_DIR"},
					Required:    false,
				},
			},
		},
	}
}

// Load reads configuration from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to the given path
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
