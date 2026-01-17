package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cxt9/claude-go/internal/platform"
)

// Session represents a portable Claude Code session
type Session struct {
	ID          string            `json:"id"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUsedAt  time.Time         `json:"last_used_at"`
	HostMachine string            `json:"host_machine"`
	Platform    platform.Platform `json:"platform"`

	// Project information
	Project ProjectRef `json:"project"`

	// Session summary (for display in picker)
	Summary string `json:"summary"`

	// Permissions granted during this session
	Permissions []Permission `json:"permissions,omitempty"`
}

// ProjectRef stores project path information for cross-machine portability
type ProjectRef struct {
	OriginalPath string `json:"original_path"` // Path on original machine
	RelativePath string `json:"relative_path"` // Portable relative path
	RemappedPath string `json:"remapped_path"` // Path on current machine
}

// Permission represents a granted permission
type Permission struct {
	Tool      string    `json:"tool"`
	Pattern   string    `json:"pattern"`
	GrantedAt time.Time `json:"granted_at"`
}

// Manager handles session storage and retrieval
type Manager struct {
	sessionsDir string
}

// NewManager creates a new session manager
func NewManager(sessionsDir string) *Manager {
	return &Manager{
		sessionsDir: sessionsDir,
	}
}

// Create creates a new session
func (m *Manager) Create(projectPath string) (*Session, error) {
	id := generateSessionID()
	now := time.Now()

	hostname, _ := os.Hostname()
	plat, _ := platform.Current()

	session := &Session{
		ID:          id,
		CreatedAt:   now,
		LastUsedAt:  now,
		HostMachine: hostname,
		Platform:    plat,
		Project: ProjectRef{
			OriginalPath: projectPath,
			RelativePath: extractRelativePath(projectPath),
			RemappedPath: projectPath,
		},
		Summary: "New session",
	}

	if err := m.Save(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Load loads a session by ID
func (m *Manager) Load(id string) (*Session, error) {
	path := m.sessionPath(id)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &session, nil
}

// Save persists a session to disk
func (m *Manager) Save(session *Session) error {
	if err := os.MkdirAll(m.sessionsDir, 0700); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	session.LastUsedAt = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	path := m.sessionPath(session.ID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	return nil
}

// Delete removes a session
func (m *Manager) Delete(id string) error {
	path := m.sessionPath(id)
	return os.Remove(path)
}

// List returns all sessions sorted by last used time (most recent first)
func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		session, err := m.Load(id)
		if err != nil {
			continue // Skip corrupted sessions
		}
		sessions = append(sessions, session)
	}

	// Sort by last used time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUsedAt.After(sessions[j].LastUsedAt)
	})

	return sessions, nil
}

// Cleanup removes sessions older than the given duration
func (m *Manager) Cleanup(maxAge time.Duration) (int, error) {
	sessions, err := m.List()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, session := range sessions {
		if session.LastUsedAt.Before(cutoff) {
			if err := m.Delete(session.ID); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// RemapProjectPath updates the session's project path for the current machine
func (m *Manager) RemapProjectPath(session *Session, newPath string) error {
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		return fmt.Errorf("project path does not exist: %s", newPath)
	}

	hostname, _ := os.Hostname()
	plat, _ := platform.Current()

	session.Project.RemappedPath = newPath
	session.HostMachine = hostname
	session.Platform = plat

	return m.Save(session)
}

func (m *Manager) sessionPath(id string) string {
	return filepath.Join(m.sessionsDir, id+".json")
}

func generateSessionID() string {
	// Simple timestamp-based ID
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

func extractRelativePath(fullPath string) string {
	// Extract a portable relative path (e.g., last 2 components)
	parts := strings.Split(filepath.Clean(fullPath), string(filepath.Separator))
	if len(parts) >= 2 {
		return filepath.Join(parts[len(parts)-2], parts[len(parts)-1])
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return fullPath
}
