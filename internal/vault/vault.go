package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	// File format magic number: "CCGO" (Claude Code Go)
	magicNumber uint32 = 0x4343474F

	// Current vault format version
	vaultVersion uint16 = 1

	// Argon2id parameters (OWASP recommended)
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32 // 256 bits for AES-256

	// Salt and nonce sizes
	saltSize  = 32
	nonceSize = 12 // GCM standard nonce size
)

var (
	ErrVaultLocked     = errors.New("vault is locked")
	ErrWrongPassword   = errors.New("incorrect password")
	ErrInvalidVault    = errors.New("invalid vault file")
	ErrVaultNotFound   = errors.New("vault not found")
	ErrEntryNotFound   = errors.New("credential entry not found")
	ErrVaultCorrupted  = errors.New("vault file corrupted")
)

// CredentialType identifies the type of stored credential
type CredentialType string

const (
	CredentialOAuth  CredentialType = "oauth"
	CredentialAPIKey CredentialType = "apikey"
	CredentialAWS    CredentialType = "aws"
	CredentialGCP    CredentialType = "gcp"
	CredentialMCP    CredentialType = "mcp"
)

// Entry represents a single credential stored in the vault
type Entry struct {
	ID        string            `json:"id"`
	Type      CredentialType    `json:"type"`
	Provider  string            `json:"provider"`  // claudeai, console, bedrock, vertex
	Data      json.RawMessage   `json:"data"`      // Type-specific credential data
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// OAuthData stores OAuth token information
type OAuthData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

// APIKeyData stores API key information
type APIKeyData struct {
	APIKey string `json:"api_key"`
}

// vaultData is the decrypted contents of the vault
type vaultData struct {
	Version   int                `json:"version"`
	Entries   map[string]*Entry  `json:"entries"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// Vault manages encrypted credential storage
type Vault struct {
	path     string
	salt     []byte
	key      []byte
	gcm      cipher.AEAD
	data     *vaultData
	mu       sync.RWMutex
	unlocked bool
}

// Create initializes a new vault with the given password
func Create(path string, password string) (*Vault, error) {
	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key from password
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	now := time.Now()
	v := &Vault{
		path:     path,
		salt:     salt,
		key:      key,
		gcm:      gcm,
		unlocked: true,
		data: &vaultData{
			Version:   1,
			Entries:   make(map[string]*Entry),
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}

	// Save initial vault
	if err := v.save(); err != nil {
		return nil, fmt.Errorf("failed to save vault: %w", err)
	}

	return v, nil
}

// Open loads an existing vault (but doesn't unlock it)
func Open(path string) (*Vault, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrVaultNotFound
	}

	return &Vault{
		path:     path,
		unlocked: false,
	}, nil
}

// Unlock decrypts the vault with the given password
func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Read vault file
	data, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Errorf("failed to read vault: %w", err)
	}

	// Parse header
	if len(data) < 6 { // magic(4) + version(2) minimum
		return ErrInvalidVault
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != magicNumber {
		return ErrInvalidVault
	}

	version := binary.BigEndian.Uint16(data[4:6])
	if version != vaultVersion {
		return fmt.Errorf("unsupported vault version: %d", version)
	}

	offset := 6

	// Read salt
	if len(data) < offset+saltSize {
		return ErrVaultCorrupted
	}
	v.salt = make([]byte, saltSize)
	copy(v.salt, data[offset:offset+saltSize])
	offset += saltSize

	// Derive key
	v.key = argon2.IDKey([]byte(password), v.salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Create cipher
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	v.gcm, err = cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Read nonce
	if len(data) < offset+nonceSize {
		return ErrVaultCorrupted
	}
	nonce := data[offset : offset+nonceSize]
	offset += nonceSize

	// Decrypt payload
	ciphertext := data[offset:]
	plaintext, err := v.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ErrWrongPassword
	}

	// Parse decrypted data
	v.data = &vaultData{}
	if err := json.Unmarshal(plaintext, v.data); err != nil {
		return ErrVaultCorrupted
	}

	v.unlocked = true
	return nil
}

// Lock clears sensitive data from memory
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Zero out sensitive data
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
	v.gcm = nil
	v.data = nil
	v.unlocked = false
}

// IsUnlocked returns whether the vault is currently unlocked
func (v *Vault) IsUnlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.unlocked
}

// save writes the encrypted vault to disk
func (v *Vault) save() error {
	if !v.unlocked {
		return ErrVaultLocked
	}

	// Serialize data
	plaintext, err := json.Marshal(v.data)
	if err != nil {
		return fmt.Errorf("failed to serialize vault: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := v.gcm.Seal(nil, nonce, plaintext, nil)

	// Build file: magic + version + salt + nonce + ciphertext
	fileSize := 4 + 2 + saltSize + nonceSize + len(ciphertext)
	file := make([]byte, fileSize)

	offset := 0

	// Magic number
	binary.BigEndian.PutUint32(file[offset:], magicNumber)
	offset += 4

	// Version
	binary.BigEndian.PutUint16(file[offset:], vaultVersion)
	offset += 2

	// Salt
	copy(file[offset:], v.salt)
	offset += saltSize

	// Nonce
	copy(file[offset:], nonce)
	offset += nonceSize

	// Ciphertext
	copy(file[offset:], ciphertext)

	// Write atomically (write to temp, then rename)
	tmpPath := v.path + ".tmp"
	if err := os.WriteFile(tmpPath, file, 0600); err != nil {
		return fmt.Errorf("failed to write vault: %w", err)
	}

	if err := os.Rename(tmpPath, v.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize vault: %w", err)
	}

	return nil
}

// SetEntry adds or updates a credential entry
func (v *Vault) SetEntry(entry *Entry) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	v.data.Entries[entry.ID] = entry
	v.data.UpdatedAt = now

	return v.save()
}

// GetEntry retrieves a credential entry by ID
func (v *Vault) GetEntry(id string) (*Entry, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked {
		return nil, ErrVaultLocked
	}

	entry, ok := v.data.Entries[id]
	if !ok {
		return nil, ErrEntryNotFound
	}

	return entry, nil
}

// DeleteEntry removes a credential entry
func (v *Vault) DeleteEntry(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}

	if _, ok := v.data.Entries[id]; !ok {
		return ErrEntryNotFound
	}

	delete(v.data.Entries, id)
	v.data.UpdatedAt = time.Now()

	return v.save()
}

// ListEntries returns all entry IDs and their types
func (v *Vault) ListEntries() ([]Entry, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked {
		return nil, ErrVaultLocked
	}

	entries := make([]Entry, 0, len(v.data.Entries))
	for _, entry := range v.data.Entries {
		// Return a copy without the sensitive data field
		entries = append(entries, Entry{
			ID:        entry.ID,
			Type:      entry.Type,
			Provider:  entry.Provider,
			CreatedAt: entry.CreatedAt,
			UpdatedAt: entry.UpdatedAt,
			ExpiresAt: entry.ExpiresAt,
			Metadata:  entry.Metadata,
		})
	}

	return entries, nil
}

// Exists checks if a vault file exists at the given path
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
