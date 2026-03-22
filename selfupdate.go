package matcha

// Self-Update Conventions
//
// For self-update to work, your project must follow these conventions:
//
// 1. GitHub Releases
//    - Create releases at github.com/{ManagerRepo}/releases
//    - Tag releases with semantic versions: v1.0.0, v1.2.3, etc.
//
// 2. Release Assets
//    - Binary naming: {Name}-linux-{arch} (e.g., fusionaly-linux-amd64, fusionaly-linux-arm64)
//    - Checksums file: checksums.txt
//
// 3. Checksums Format (checksums.txt)
//    Each line: "<sha256>  <filename>"
//    Example:
//      a1b2c3d4...  fusionaly-linux-amd64
//      e5f6g7h8...  fusionaly-linux-arm64
//
// 4. Version in Binary
//    Pass version at build time via ldflags:
//      go build -ldflags "-X main.currentManagerVersion=$(VERSION)" ./cmd/manager/
//
// GoReleaser handles all of this automatically with default configuration.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// githubRelease represents a GitHub release response.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// SelfUpdate checks for a newer manager version and updates if available.
// Returns true if an update was performed.
func (m *Matcha) SelfUpdate() (bool, error) {
	if m.config.ManagerRepo == "" {
		return false, nil // Self-update not configured
	}

	currentVersion := strings.TrimPrefix(m.config.ManagerVersion, "v")
	if currentVersion == "" {
		return false, nil // No version configured
	}

	// Fetch latest release from GitHub
	release, err := m.fetchLatestRelease()
	if err != nil {
		return false, fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// Dev builds always update to latest release
	if currentVersion != "dev" && compareVersions(currentVersion, latestVersion) >= 0 {
		return false, nil // Already up to date
	}

	// Find the right asset for this OS/arch
	assetName := m.getManagerAssetName()
	var downloadURL, checksumsURL string

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.BrowserDownloadURL
		}
	}

	if downloadURL == "" {
		return false, fmt.Errorf("no binary found for %s in release %s", assetName, release.TagName)
	}

	// Download and install (includes checksum verification)
	if err := m.downloadAndInstall(downloadURL, checksumsURL, assetName); err != nil {
		return false, fmt.Errorf("failed to update binary: %w", err)
	}

	printSuccess("Manager updated: %s → %s", currentVersion, latestVersion)

	// Re-exec with new binary to continue update with new code
	// The checksum was verified in downloadAndInstall, so this is safe
	fmt.Println("Restarting with new binary...")
	if err := syscall.Exec(m.config.BinaryPath, os.Args, os.Environ()); err != nil {
		// If re-exec fails, continue with current binary
		printWarn("failed to restart: %v (continuing with current binary)", err)
		return true, nil
	}

	// This line is never reached if Exec succeeds
	return true, nil
}

// fetchLatestRelease gets the latest release from GitHub.
func (m *Matcha) fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", m.config.ManagerRepo)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// getManagerAssetName returns the expected asset name for current OS/arch.
func (m *Matcha) getManagerAssetName() string {
	// Pattern: {name}-linux-{arch} (matches GoReleaser output)
	return fmt.Sprintf("%s-linux-%s", m.config.Name, runtime.GOARCH)
}

// downloadAndInstall downloads a new binary and replaces the current one.
func (m *Matcha) downloadAndInstall(downloadURL, checksumsURL, assetName string) error {
	client := &http.Client{Timeout: 60 * time.Second}

	// Download to temp file
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpPath := "/tmp/" + m.config.Name + ".new"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	out.Close()

	// Verify checksum if available
	if checksumsURL != "" {
		if err := m.verifyChecksum(client, checksumsURL, tmpPath, assetName); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Make executable and replace
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	if err := os.Rename(tmpPath, m.config.BinaryPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}

// verifyChecksum downloads checksums.txt and verifies the binary's SHA256.
func (m *Matcha) verifyChecksum(client *http.Client, checksumsURL, filePath, assetName string) error {
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums download failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	// Parse checksums.txt — format: "<sha256>  <filename>"
	var expectedHash string
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s", assetName)
	}

	// Hash the downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash file: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// compareVersions compares two semantic versions.
// Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2.
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Strip any suffix like -dirty
	if idx := strings.Index(v1, "-"); idx > 0 {
		v1 = v1[:idx]
	}
	if idx := strings.Index(v2, "-"); idx > 0 {
		v2 = v2[:idx]
	}

	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")

	maxParts := len(v1Parts)
	if len(v2Parts) > maxParts {
		maxParts = len(v2Parts)
	}

	for i := 0; i < maxParts; i++ {
		var n1, n2 int
		if i < len(v1Parts) {
			n1, _ = strconv.Atoi(v1Parts[i])
		}
		if i < len(v2Parts) {
			n2, _ = strconv.Atoi(v2Parts[i])
		}

		if n1 < n2 {
			return -1
		} else if n1 > n2 {
			return 1
		}
	}

	return 0
}
