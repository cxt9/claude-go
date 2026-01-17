package platform

import (
	"fmt"
	"runtime"
)

// Platform represents a supported OS/architecture combination
type Platform string

const (
	DarwinARM64 Platform = "darwin-arm64"
	DarwinAMD64 Platform = "darwin-amd64"
	LinuxAMD64  Platform = "linux-amd64"
	WindowsAMD64 Platform = "windows-amd64"
)

// AllPlatforms lists all supported platforms for cross-compilation
var AllPlatforms = []Platform{
	DarwinARM64,
	DarwinAMD64,
	LinuxAMD64,
	WindowsAMD64,
}

// Current detects the current platform
func Current() (Platform, error) {
	key := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	switch key {
	case "darwin-arm64":
		return DarwinARM64, nil
	case "darwin-amd64":
		return DarwinAMD64, nil
	case "linux-amd64":
		return LinuxAMD64, nil
	case "windows-amd64":
		return WindowsAMD64, nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", key)
	}
}

// BinaryName returns the appropriate binary name for this platform
func (p Platform) BinaryName(base string) string {
	if p == WindowsAMD64 {
		return base + ".exe"
	}
	return base
}

// String returns the platform identifier
func (p Platform) String() string {
	return string(p)
}

// GOOS returns the Go OS value for this platform
func (p Platform) GOOS() string {
	switch p {
	case DarwinARM64, DarwinAMD64:
		return "darwin"
	case LinuxAMD64:
		return "linux"
	case WindowsAMD64:
		return "windows"
	default:
		return ""
	}
}

// GOARCH returns the Go architecture value for this platform
func (p Platform) GOARCH() string {
	switch p {
	case DarwinARM64:
		return "arm64"
	case DarwinAMD64, LinuxAMD64, WindowsAMD64:
		return "amd64"
	default:
		return ""
	}
}
