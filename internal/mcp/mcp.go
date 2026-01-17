package mcp

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cxt9/claude-go/internal/config"
	"github.com/cxt9/claude-go/internal/platform"
)

// ServerStatus represents the availability status of an MCP server
type ServerStatus struct {
	Name        string
	Portability string
	Available   bool
	Required    bool
	Error       string
}

// Manager handles MCP server resolution and availability checking
type Manager struct {
	usbRoot    string
	projectDir string
	platform   platform.Platform
	config     *config.MCPConfig
}

// NewManager creates a new MCP manager
func NewManager(usbRoot, projectDir string, cfg *config.MCPConfig) (*Manager, error) {
	plat, err := platform.Current()
	if err != nil {
		return nil, err
	}

	return &Manager{
		usbRoot:    usbRoot,
		projectDir: projectDir,
		platform:   plat,
		config:     cfg,
	}, nil
}

// CheckServers checks availability of all configured MCP servers
func (m *Manager) CheckServers() ([]ServerStatus, error) {
	var statuses []ServerStatus

	for name, server := range m.config.Servers {
		status := ServerStatus{
			Name:        name,
			Portability: server.Portability,
			Required:    server.Required,
		}

		switch server.Portability {
		case "remote":
			status.Available, status.Error = m.checkRemoteServer(server)
		case "bundled":
			status.Available, status.Error = m.checkLocalServer(server, true)
		case "usb-local":
			status.Available, status.Error = m.checkLocalServer(server, true)
		case "host-local":
			status.Available, status.Error = m.checkLocalServer(server, false)
		default:
			status.Available = false
			status.Error = fmt.Sprintf("unknown portability type: %s", server.Portability)
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// ResolveCommand resolves a server command with variable substitution
func (m *Manager) ResolveCommand(server config.MCPServer) (string, []string, error) {
	cmd := m.substituteVars(server.Command)

	// For bundled/usb-local, append platform-specific binary name
	if server.Portability == "bundled" || server.Portability == "usb-local" {
		cmd = m.resolvePlatformBinary(cmd)
	}

	// Resolve args
	args := make([]string, len(server.Args))
	for i, arg := range server.Args {
		args[i] = m.substituteVars(arg)
	}

	return cmd, args, nil
}

// ResolveEnv resolves environment variables for a server
func (m *Manager) ResolveEnv(server config.MCPServer) map[string]string {
	env := make(map[string]string)
	for k, v := range server.Env {
		env[k] = m.substituteVars(v)
	}
	return env
}

// GetAvailableServers returns only servers that are available
func (m *Manager) GetAvailableServers() (map[string]config.MCPServer, []ServerStatus, error) {
	statuses, err := m.CheckServers()
	if err != nil {
		return nil, nil, err
	}

	available := make(map[string]config.MCPServer)
	var unavailable []ServerStatus

	for _, status := range statuses {
		if status.Available {
			available[status.Name] = m.config.Servers[status.Name]
		} else {
			unavailable = append(unavailable, status)
		}
	}

	return available, unavailable, nil
}

// HasRequiredUnavailable checks if any required servers are unavailable
func (m *Manager) HasRequiredUnavailable() (bool, []string) {
	statuses, _ := m.CheckServers()

	var missing []string
	for _, status := range statuses {
		if status.Required && !status.Available {
			missing = append(missing, status.Name)
		}
	}

	return len(missing) > 0, missing
}

func (m *Manager) checkRemoteServer(server config.MCPServer) (bool, string) {
	if server.URL == "" {
		return false, "no URL configured"
	}

	// Quick HTTP HEAD check with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(server.URL)
	if err != nil {
		return false, fmt.Sprintf("unreachable: %v", err)
	}
	resp.Body.Close()

	// Accept any response (server is at least responding)
	return true, ""
}

func (m *Manager) checkLocalServer(server config.MCPServer, resolveVars bool) (bool, string) {
	if server.Command == "" {
		return false, "no command configured"
	}

	cmd := server.Command
	if resolveVars {
		cmd = m.substituteVars(cmd)
		cmd = m.resolvePlatformBinary(cmd)
	}

	// Check if command exists
	if filepath.IsAbs(cmd) {
		if _, err := os.Stat(cmd); os.IsNotExist(err) {
			return false, fmt.Sprintf("not found: %s", cmd)
		}
		return true, ""
	}

	// Check in PATH
	_, err := exec.LookPath(cmd)
	if err != nil {
		return false, fmt.Sprintf("not in PATH: %s", cmd)
	}

	return true, ""
}

func (m *Manager) substituteVars(s string) string {
	s = strings.ReplaceAll(s, "$USB_ROOT", m.usbRoot)
	s = strings.ReplaceAll(s, "${USB_ROOT}", m.usbRoot)
	s = strings.ReplaceAll(s, "$PROJECT_DIR", m.projectDir)
	s = strings.ReplaceAll(s, "${PROJECT_DIR}", m.projectDir)
	return s
}

func (m *Manager) resolvePlatformBinary(path string) string {
	// If path already contains platform, return as-is
	if strings.Contains(path, string(m.platform)) {
		return path
	}

	// Check if platform subdirectory exists
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	platformPath := filepath.Join(dir, string(m.platform), m.platform.BinaryName(base))
	if _, err := os.Stat(platformPath); err == nil {
		return platformPath
	}

	// Fall back to original with platform binary name
	return m.platform.BinaryName(path)
}

// GenerateClaudeConfig generates MCP configuration for Claude Code
func (m *Manager) GenerateClaudeConfig() (map[string]interface{}, error) {
	available, _, err := m.GetAvailableServers()
	if err != nil {
		return nil, err
	}

	mcpServers := make(map[string]interface{})

	for name, server := range available {
		serverConfig := make(map[string]interface{})

		switch server.Type {
		case "stdio":
			cmd, args, _ := m.ResolveCommand(server)
			serverConfig["command"] = cmd
			if len(args) > 0 {
				serverConfig["args"] = args
			}
			env := m.ResolveEnv(server)
			if len(env) > 0 {
				serverConfig["env"] = env
			}

		case "http", "websocket":
			serverConfig["url"] = server.URL
		}

		mcpServers[name] = serverConfig
	}

	return map[string]interface{}{
		"mcpServers": mcpServers,
	}, nil
}
