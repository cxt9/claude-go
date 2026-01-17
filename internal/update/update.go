package update

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cxt9/claude-go/internal/platform"
)

const (
	manifestURL = "https://github.com/cxt9/claude-go/releases/latest/download/manifest.json"
	downloadURL = "https://github.com/cxt9/claude-go/releases/download/%s/claude-go-%s-%s.zip"
)

// Manifest represents the version manifest from GitHub
type Manifest struct {
	Version     string              `json:"version"`
	ReleaseDate string              `json:"release_date"`
	Changelog   []string            `json:"changelog"`
	Downloads   map[string]Download `json:"downloads"`
	MinVersion  string              `json:"min_version"`
}

// Download represents download information for a platform
type Download struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Updater handles self-updates
type Updater struct {
	USBRoot        string
	CurrentVersion string
	Platform       platform.Platform
}

// NewUpdater creates a new updater
func NewUpdater(usbRoot string) (*Updater, error) {
	plat, err := platform.Current()
	if err != nil {
		return nil, err
	}

	version := readVersionFile(usbRoot)

	return &Updater{
		USBRoot:        usbRoot,
		CurrentVersion: version,
		Platform:       plat,
	}, nil
}

// CheckForUpdate checks if a newer version is available
func (u *Updater) CheckForUpdate() (*Manifest, bool, error) {
	resp, err := http.Get(manifestURL)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("manifest not found: %s", resp.Status)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, false, fmt.Errorf("invalid manifest: %w", err)
	}

	hasUpdate := compareVersions(manifest.Version, u.CurrentVersion) > 0

	return &manifest, hasUpdate, nil
}

// PerformUpdate downloads and installs an update
func (u *Updater) PerformUpdate(manifest *Manifest, progressFn func(downloaded, total int64)) error {
	download, ok := manifest.Downloads[string(u.Platform)]
	if !ok {
		return fmt.Errorf("no download available for platform: %s", u.Platform)
	}

	// Create rollback backup
	if err := u.createRollback(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Download update
	tmpFile, err := u.downloadUpdate(download, progressFn)
	if err != nil {
		u.rollback()
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpFile)

	// Verify checksum
	if err := u.verifyChecksum(tmpFile, download.SHA256); err != nil {
		u.rollback()
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// Extract update
	if err := u.extractUpdate(tmpFile); err != nil {
		u.rollback()
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Update version file
	if err := u.writeVersionFile(manifest.Version); err != nil {
		// Non-fatal
		fmt.Printf("Warning: failed to update version file: %v\n", err)
	}

	// Cleanup
	u.cleanupRollback()
	u.clearCache()

	return nil
}

// PerformOfflineUpdate installs from a local zip file
func (u *Updater) PerformOfflineUpdate(zipPath string) error {
	// Create rollback backup
	if err := u.createRollback(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Extract update
	if err := u.extractUpdate(zipPath); err != nil {
		u.rollback()
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Cleanup
	u.cleanupRollback()
	u.clearCache()

	return nil
}

func (u *Updater) createRollback() error {
	binDir := filepath.Join(u.USBRoot, "bin")
	rollbackDir := filepath.Join(u.USBRoot, ".rollback")

	// Remove old rollback if exists
	os.RemoveAll(rollbackDir)

	// Copy bin to rollback
	return copyDir(binDir, rollbackDir)
}

func (u *Updater) rollback() error {
	rollbackDir := filepath.Join(u.USBRoot, ".rollback")
	binDir := filepath.Join(u.USBRoot, "bin")

	if _, err := os.Stat(rollbackDir); os.IsNotExist(err) {
		return fmt.Errorf("no rollback available")
	}

	os.RemoveAll(binDir)
	return os.Rename(rollbackDir, binDir)
}

func (u *Updater) cleanupRollback() {
	rollbackDir := filepath.Join(u.USBRoot, ".rollback")
	os.RemoveAll(rollbackDir)
}

func (u *Updater) clearCache() {
	cacheDir := filepath.Join(u.USBRoot, "cache")
	os.RemoveAll(cacheDir)
	os.MkdirAll(cacheDir, 0700)
}

func (u *Updater) downloadUpdate(download Download, progressFn func(downloaded, total int64)) (string, error) {
	resp, err := http.Get(download.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "claude-go-update-*.zip")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Download with progress
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			tmpFile.Write(buf[:n])
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, download.Size)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return tmpFile.Name(), nil
}

func (u *Updater) verifyChecksum(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func (u *Updater) extractUpdate(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Only extract bin/ and scripts
		if !strings.HasPrefix(f.Name, "bin/") &&
			!strings.HasSuffix(f.Name, ".sh") &&
			!strings.HasSuffix(f.Name, ".bat") {
			continue
		}

		destPath := filepath.Join(u.USBRoot, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, f.Mode())
			continue
		}

		if err := extractFile(f, destPath); err != nil {
			return err
		}
	}

	return nil
}

func (u *Updater) writeVersionFile(version string) error {
	versionFile := filepath.Join(u.USBRoot, ".version")
	data := fmt.Sprintf(`{"version":"%s","updated_at":"%s"}`, version, time.Now().Format(time.RFC3339))
	return os.WriteFile(versionFile, []byte(data), 0644)
}

func readVersionFile(usbRoot string) string {
	versionFile := filepath.Join(usbRoot, ".version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return "0.0.0"
	}

	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return "0.0.0"
	}

	return v.Version
}

func extractFile(f *zip.File, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Simple version comparison (assumes semver format x.y.z)
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}

		if numA > numB {
			return 1
		}
		if numA < numB {
			return -1
		}
	}

	return 0
}
