package launcher

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cxt9/claude-go/internal/auth"
	"github.com/cxt9/claude-go/internal/config"
	"github.com/cxt9/claude-go/internal/mcp"
	"github.com/cxt9/claude-go/internal/platform"
	"github.com/cxt9/claude-go/internal/session"
	"github.com/cxt9/claude-go/internal/vault"
	"golang.org/x/term"
)

const (
	minPasswordLength = 12
	banner            = `
╭─────────────────────────────────────────────╮
│           Claude Code Go                    │
│         Portable Claude Environment         │
╰─────────────────────────────────────────────╯
`
)

// App holds the application state
type App struct {
	usbRoot        string
	platform       platform.Platform
	config         *config.Config
	vault          *vault.Vault
	auth           *auth.Authenticator
	sessionManager *session.Manager
	mcpManager     *mcp.Manager
}

// Run is the main entry point
func Run(args []string) error {
	fmt.Print(banner)

	// Detect USB root (directory containing this binary)
	usbRoot, err := detectUSBRoot()
	if err != nil {
		return fmt.Errorf("failed to detect USB root: %w", err)
	}

	plat, err := platform.Current()
	if err != nil {
		return fmt.Errorf("unsupported platform: %w", err)
	}

	app := &App{
		usbRoot:  usbRoot,
		platform: plat,
	}

	// Load or create configuration
	configPath := filepath.Join(usbRoot, "config", "settings.json")
	app.config, err = config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize session manager
	sessionsDir := filepath.Join(usbRoot, "sessions")
	app.sessionManager = session.NewManager(sessionsDir)

	// Check if vault exists
	vaultPath := filepath.Join(usbRoot, "vault", "credentials.vault")
	if !vault.Exists(vaultPath) {
		return app.runFirstTimeSetup(vaultPath)
	}

	return app.runNormalLaunch(vaultPath)
}

func (app *App) runFirstTimeSetup(vaultPath string) error {
	fmt.Println("\nWelcome! Let's set up your portable Claude environment.\n")

	// Step 1: Create master password
	fmt.Println("Step 1: Create a master password to protect your credentials")
	fmt.Println("        This password encrypts everything stored on this USB.\n")

	password, err := app.promptPassword("Master password (min 12 chars): ", true)
	if err != nil {
		return err
	}

	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	confirm, err := app.promptPassword("Confirm password: ", false)
	if err != nil {
		return err
	}

	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}

	// Create vault
	v, err := vault.Create(vaultPath, password)
	if err != nil {
		return fmt.Errorf("failed to create vault: %w", err)
	}
	app.vault = v
	app.auth = auth.NewAuthenticator(v)

	fmt.Println("✓ Vault created\n")

	// Step 2: Authentication
	fmt.Println("Step 2: Link your Claude account\n")
	fmt.Println("How would you like to authenticate?")
	fmt.Println("  [1] Claude.ai account (Pro/Max subscription)")
	fmt.Println("  [2] API Key (Claude Console)")
	fmt.Println("  [3] Amazon Bedrock")
	fmt.Println("  [4] Google Vertex AI")
	fmt.Print("\n> ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1":
		if err := app.setupOAuth(); err != nil {
			return err
		}
	case "2":
		if err := app.setupAPIKey(auth.ProviderConsole); err != nil {
			return err
		}
	case "3":
		if err := app.setupAPIKey(auth.ProviderBedrock); err != nil {
			return err
		}
	case "4":
		if err := app.setupAPIKey(auth.ProviderVertex); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}

	// Save configuration
	configPath := filepath.Join(app.usbRoot, "config", "settings.json")
	if err := app.config.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Setup complete! Claude Code Go is ready to use.\n")

	return app.startSession("")
}

func (app *App) runNormalLaunch(vaultPath string) error {
	// Open vault (locked)
	v, err := vault.Open(vaultPath)
	if err != nil {
		return fmt.Errorf("failed to open vault: %w", err)
	}
	app.vault = v

	// Prompt for password
	fmt.Print("Unlock your portable vault\n")
	password, err := app.promptPassword("Master password: ", false)
	if err != nil {
		return err
	}

	if err := v.Unlock(password); err != nil {
		if err == vault.ErrWrongPassword {
			return fmt.Errorf("incorrect password")
		}
		return fmt.Errorf("failed to unlock vault: %w", err)
	}
	fmt.Println("✓ Vault unlocked\n")

	app.auth = auth.NewAuthenticator(v)

	// Show session picker
	return app.showSessionPicker()
}

func (app *App) showSessionPicker() error {
	sessions, err := app.sessionManager.List()
	if err != nil {
		return err
	}

	if len(sessions) > 0 {
		fmt.Println("Previous sessions:")
		for i, s := range sessions {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(sessions)-10)
				break
			}
			age := formatAge(time.Since(s.LastUsedAt))
			projectName := filepath.Base(s.Project.OriginalPath)
			fmt.Printf("  [%d] %s - %s: \"%s\"\n", i+1, age, projectName, truncate(s.Summary, 40))
		}
		fmt.Printf("  [%d] Start new session\n", len(sessions)+1)
		fmt.Print("\n> ")

		var choice string
		fmt.Scanln(&choice)

		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 1 && idx <= len(sessions) {
			// Resume existing session
			return app.resumeSession(sessions[idx-1])
		}
	}

	// Start new session
	return app.promptNewSession()
}

func (app *App) promptNewSession() error {
	fmt.Print("Enter project directory on this machine: ")

	reader := bufio.NewReader(os.Stdin)
	projectPath, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	projectPath = strings.TrimSpace(projectPath)

	// Expand ~ to home directory
	if strings.HasPrefix(projectPath, "~") {
		home, _ := os.UserHomeDir()
		projectPath = filepath.Join(home, projectPath[1:])
	}

	// Validate path exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", projectPath)
	}

	return app.startSession(projectPath)
}

func (app *App) resumeSession(s *session.Session) error {
	fmt.Printf("\nResuming session...\n")

	// Check if original project path exists on this machine
	if _, err := os.Stat(s.Project.OriginalPath); err == nil {
		s.Project.RemappedPath = s.Project.OriginalPath
	} else {
		// Prompt for new path
		fmt.Printf("Original path not found: %s\n", s.Project.OriginalPath)
		fmt.Printf("Enter project directory on this machine: ")

		reader := bufio.NewReader(os.Stdin)
		newPath, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		newPath = strings.TrimSpace(newPath)

		if err := app.sessionManager.RemapProjectPath(s, newPath); err != nil {
			return err
		}

		fmt.Printf("Project path remapped: %s -> %s\n", s.Project.OriginalPath, newPath)
	}

	return app.startSession(s.Project.RemappedPath)
}

func (app *App) startSession(projectPath string) error {
	// Create or update session
	var s *session.Session
	var err error

	if projectPath != "" {
		s, err = app.sessionManager.Create(projectPath)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Initialize MCP manager
	app.mcpManager, err = mcp.NewManager(app.usbRoot, projectPath, &app.config.MCP)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP: %w", err)
	}

	// Check MCP servers
	fmt.Println("\nChecking MCP servers...")
	available, unavailable, err := app.mcpManager.GetAvailableServers()
	if err != nil {
		return fmt.Errorf("failed to check MCP servers: %w", err)
	}

	for name := range available {
		fmt.Printf("  ✓ %s\n", name)
	}
	for _, status := range unavailable {
		fmt.Printf("  ⚠ %s (%s) - %s\n", status.Name, status.Portability, status.Error)
	}

	// Check for required unavailable servers
	hasRequired, missing := app.mcpManager.HasRequiredUnavailable()
	if hasRequired {
		return fmt.Errorf("required MCP servers unavailable: %v", missing)
	}

	// Setup environment and launch Claude Code
	return app.launchClaudeCode(projectPath, s)
}

func (app *App) launchClaudeCode(projectPath string, s *session.Session) error {
	fmt.Println("\nStarting Claude Code Go...")
	fmt.Printf("Portable Mode • Project: %s\n\n", projectPath)

	// Setup environment variables for isolation
	env := app.buildEnvironment(projectPath)

	// Get the credential for Claude
	providers, err := app.auth.ListProviders()
	if err != nil || len(providers) == 0 {
		return fmt.Errorf("no authentication configured")
	}

	credential, err := app.auth.GetCredential(providers[0])
	if err != nil {
		return fmt.Errorf("failed to get credential: %w", err)
	}

	// Add credential to environment
	env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", credential))

	// Generate MCP config
	mcpConfig, err := app.mcpManager.GenerateClaudeConfig()
	if err != nil {
		return fmt.Errorf("failed to generate MCP config: %w", err)
	}

	// Write MCP config to temp file
	// (In practice, Claude Code would read this from the portable config)
	_ = mcpConfig

	// Find claude binary (would be bundled on USB)
	claudeBinary := app.findClaudeBinary()

	// Launch Claude Code
	cmd := exec.Command(claudeBinary)
	cmd.Dir = projectPath
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (app *App) buildEnvironment(projectPath string) []string {
	// Start with minimal environment
	env := []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("USER=%s", os.Getenv("USER")),
		fmt.Sprintf("PATH=%s", app.buildPath()),
		fmt.Sprintf("TERM=%s", os.Getenv("TERM")),

		// Claude Code Go specific
		fmt.Sprintf("CLAUDE_CONFIG_DIR=%s", filepath.Join(app.usbRoot, "config")),
		fmt.Sprintf("CLAUDE_DATA_DIR=%s", filepath.Join(app.usbRoot, "sessions")),
		fmt.Sprintf("CLAUDE_CACHE_DIR=%s", filepath.Join(app.usbRoot, "cache")),
		fmt.Sprintf("CLAUDE_CODE_GO=1"),
		fmt.Sprintf("CLAUDE_CODE_GO_USB_ROOT=%s", app.usbRoot),
	}

	return env
}

func (app *App) buildPath() string {
	// Prioritize USB-bundled binaries
	usbBinPath := filepath.Join(app.usbRoot, "bin", string(app.platform))
	nodePath := filepath.Join(usbBinPath, "node", "bin")

	return fmt.Sprintf("%s:%s:%s", usbBinPath, nodePath, os.Getenv("PATH"))
}

func (app *App) findClaudeBinary() string {
	// Look for claude in USB bin directory first
	usbClaude := filepath.Join(app.usbRoot, "bin", string(app.platform), "claude")
	if _, err := os.Stat(usbClaude); err == nil {
		return usbClaude
	}

	// Fall back to PATH
	claudePath, err := exec.LookPath("claude")
	if err == nil {
		return claudePath
	}

	// Default
	return "claude"
}

func (app *App) setupOAuth() error {
	fmt.Println("\nOpening browser for Claude.ai login...")

	ctx := context.Background()

	// Start callback server
	codeChan, err := auth.StartCallbackServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	// Get authorization URL
	authURL, state, err := app.auth.StartOAuthFlow(ctx)
	if err != nil {
		return err
	}

	// Open browser
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Please open this URL in your browser:\n%s\n", authURL)
	}

	// Wait for callback
	select {
	case code := <-codeChan:
		if err := app.auth.CompleteOAuthFlow(ctx, code, state); err != nil {
			return err
		}
		fmt.Println("✓ Authentication successful!")

	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authentication timed out")
	}

	return nil
}

func (app *App) setupAPIKey(provider auth.Provider) error {
	fmt.Print("\nEnter your API key: ")

	apiKey, err := app.promptPassword("", false)
	if err != nil {
		return err
	}

	if err := app.auth.SetAPIKey(provider, apiKey); err != nil {
		return err
	}

	fmt.Println("✓ API key stored!")
	return nil
}

func (app *App) promptPassword(prompt string, showRequirements bool) (string, error) {
	if prompt != "" {
		fmt.Print(prompt)
	}

	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println()

	return string(password), nil
}

func detectUSBRoot() (string, error) {
	// Get the directory containing the executable
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	// Go up from bin/<platform>/ to USB root
	binDir := filepath.Dir(exe)
	platformDir := filepath.Dir(binDir)
	usbRoot := filepath.Dir(platformDir)

	// Verify it looks like a USB root
	if _, err := os.Stat(filepath.Join(usbRoot, "config")); os.IsNotExist(err) {
		// Maybe we're running from a different location, use current directory
		cwd, _ := os.Getwd()
		return cwd, nil
	}

	return usbRoot, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch plat, _ := platform.Current(); plat {
	case platform.DarwinARM64, platform.DarwinAMD64:
		cmd = exec.Command("open", url)
	case platform.LinuxAMD64:
		cmd = exec.Command("xdg-open", url)
	case platform.WindowsAMD64:
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform for browser open")
	}

	return cmd.Start()
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
