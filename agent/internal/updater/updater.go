package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Config holds updater configuration.
type Config struct {
	CurrentVersion string
	ChinaMirror    bool
	AutoUpdate     bool
	Repo           string
}

// FetchLatestVersion detects the latest release tag from GitHub or mirrors.
func FetchLatestVersion(ctx context.Context, repo string, chinaMirror bool) (string, error) {
	if repo == "" {
		repo = "guimc233/JustPing"
	}

	endpoints := GetLatestCandidates(repo, chinaMirror)
	tagRegex := regexp.MustCompile(`releases/tag/(v?[0-9]+(?:\.[0-9]+)*(?:-[0-9A-Za-z._-]+)?)`)

	for _, ep := range endpoints {
		tag, err := queryEndpointForTag(ctx, ep, tagRegex)
		if err == nil && tag != "" {
			return tag, nil
		}
	}

	// Fallback to GitHub API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	tag, err := queryAPIForTag(ctx, apiURL)
	if err == nil && tag != "" {
		return tag, nil
	}

	return "", fmt.Errorf("failed to detect latest release from any endpoint")
}

func queryEndpointForTag(ctx context.Context, url string, tagRegex *regexp.Regexp) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "JustPing-Agent-Updater")

	client := &http.Client{
		Timeout: 7 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			matches := tagRegex.FindStringSubmatch(loc)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	if resp.StatusCode == http.StatusOK {
		buf := make([]byte, 65536)
		n, _ := io.ReadFull(resp.Body, buf)
		body := string(buf[:n])
		matches := tagRegex.FindStringSubmatch(body)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("tag not found from %s (status %d)", url, resp.StatusCode)
}

func queryAPIForTag(ctx context.Context, apiURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "JustPing-Agent-Updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.TagName == "" {
		return "", fmt.Errorf("empty tag_name in API response")
	}
	return data.TagName, nil
}

// DownloadAndVerify downloads the release checksums and binary, then validates SHA256.
func DownloadAndVerify(ctx context.Context, repo, version string, chinaMirror bool) ([]byte, error) {
	binName, err := GetBinaryName()
	if err != nil {
		return nil, err
	}

	candidates := GetDownloadCandidates(repo, version, chinaMirror)
	client := &http.Client{Timeout: 60 * time.Second}

	var lastErr error
	for _, base := range candidates {
		checksumURL := fmt.Sprintf("%s/SHA256SUMS.txt", base)
		binaryURL := fmt.Sprintf("%s/%s", base, binName)

		sumsData, err := fetchURL(ctx, client, checksumURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to fetch SHA256SUMS.txt from %s: %w", base, err)
			continue
		}

		expectedHash := parseChecksum(string(sumsData), binName)
		if expectedHash == "" {
			lastErr = fmt.Errorf("checksum for %s not found in SHA256SUMS.txt from %s", binName, base)
			continue
		}

		binData, err := fetchURL(ctx, client, binaryURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to fetch binary from %s: %w", binaryURL, err)
			continue
		}

		if err := validateBinaryHeader(binData); err != nil {
			lastErr = fmt.Errorf("invalid binary format from %s: %w", binaryURL, err)
			continue
		}

		actualHash := sha256Hex(binData)
		if !strings.EqualFold(actualHash, expectedHash) {
			lastErr = fmt.Errorf("checksum mismatch from %s (expected %s, got %s)", binaryURL, expectedHash, actualHash)
			continue
		}

		return binData, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to download and verify binary from any source")
}

func fetchURL(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "JustPing-Agent-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func parseChecksum(content, binaryName string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			name := strings.TrimPrefix(fields[1], "*")
			if name == binaryName {
				return fields[0]
			}
		}
	}
	return ""
}

func validateBinaryHeader(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("binary too small (%d bytes)", len(data))
	}
	if runtime.GOOS == "linux" {
		if data[0] != 0x7f || data[1] != 'E' || data[2] != 'L' || data[3] != 'F' {
			return fmt.Errorf("not a valid ELF binary")
		}
	} else if runtime.GOOS == "windows" {
		if data[0] != 'M' || data[1] != 'Z' {
			return fmt.Errorf("not a valid PE executable")
		}
	}
	return nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ReplaceCurrentExecutable writes the new binary atomically and restores execution permissions.
func ReplaceCurrentExecutable(newData []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for executable: %w", err)
	}

	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, ".justping-agent-update-*")
	if err != nil {
		tmpFile, err = os.CreateTemp("", ".justping-agent-update-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(newData); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write update data: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod temp binary: %w", err)
	}

	if runtime.GOOS == "linux" {
		_ = exec.Command("setcap", "cap_net_raw+ep", tmpPath).Run()
	}

	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("failed to rename existing binary on Windows: %w", err)
		}
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		if copyErr := copyFileAtomic(tmpPath, execPath); copyErr != nil {
			return fmt.Errorf("failed to replace binary %s: %w (fallback: %v)", execPath, err, copyErr)
		}
	}

	return nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dstDir := filepath.Dir(dst)
	tmpFile, err := os.CreateTemp(dstDir, ".justping-copy-*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmpFile, in); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	_ = os.Chmod(tmpName, 0755)
	if runtime.GOOS == "linux" {
		_ = exec.Command("setcap", "cap_net_raw+ep", tmpName).Run()
	}
	return os.Rename(tmpName, dst)
}

// RunUpdate performs update checking and replacement.
// Returns (updated bool, version string, err error).
func RunUpdate(cfg Config, force bool) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if cfg.Repo == "" {
		cfg.Repo = "guimc233/JustPing"
	}

	log.Printf("[Updater] Checking for updates (current: %s, china_mirror: %v)...", cfg.CurrentVersion, cfg.ChinaMirror)
	latestVersion, err := FetchLatestVersion(ctx, cfg.Repo, cfg.ChinaMirror)
	if err != nil {
		return false, "", fmt.Errorf("failed to check for updates: %w", err)
	}

	needsUpdate := force || CleanVersion(cfg.CurrentVersion) != CleanVersion(latestVersion)
	if !needsUpdate {
		return false, latestVersion, nil
	}

	log.Printf("[Updater] New version detected: %s. Downloading and verifying...", latestVersion)
	data, err := DownloadAndVerify(ctx, cfg.Repo, latestVersion, cfg.ChinaMirror)
	if err != nil {
		return false, latestVersion, fmt.Errorf("update download failed: %w", err)
	}

	if err := ReplaceCurrentExecutable(data); err != nil {
		return false, latestVersion, fmt.Errorf("failed to replace binary: %w", err)
	}

	log.Printf("[Updater] Successfully updated binary to %s", latestVersion)
	return true, latestVersion, nil
}

// StartAutoUpdate begins periodic background update checks.
// Initial check runs after 2-5 minutes jitter, followed by ticks every 8 hours.
func StartAutoUpdate(ctx context.Context, cfg Config, onUpdateSuccess func(newVersion string)) {
	if !cfg.AutoUpdate {
		log.Printf("[Updater] Automatic updates are disabled.")
		return
	}

	if cfg.Repo == "" {
		cfg.Repo = "guimc233/JustPing"
	}

	go func() {
		initialJitter := time.Duration(120+rand.Intn(180)) * time.Second
		log.Printf("[Updater] Background auto-updater enabled; first check in %s", initialJitter.Round(time.Second))

		select {
		case <-ctx.Done():
			return
		case <-time.After(initialJitter):
		}

		checkAndApply := func() bool {
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			latest, err := FetchLatestVersion(checkCtx, cfg.Repo, cfg.ChinaMirror)
			if err != nil {
				log.Printf("[Updater] Background check error: %v", err)
				return false
			}

			if !IsNewer(latest, cfg.CurrentVersion) {
				log.Printf("[Updater] Agent is up to date (%s)", cfg.CurrentVersion)
				return false
			}

			log.Printf("[Updater] New version available: %s (current: %s). Updating...", latest, cfg.CurrentVersion)
			data, err := DownloadAndVerify(checkCtx, cfg.Repo, latest, cfg.ChinaMirror)
			if err != nil {
				log.Printf("[Updater] Background download error: %v", err)
				return false
			}

			if err := ReplaceCurrentExecutable(data); err != nil {
				log.Printf("[Updater] Background replace error: %v", err)
				return false
			}

			log.Printf("[Updater] Auto-update to %s successful.", latest)
			if onUpdateSuccess != nil {
				onUpdateSuccess(latest)
			}
			return true
		}

		if checkAndApply() {
			return
		}

		ticker := time.NewTicker(8 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if checkAndApply() {
					return
				}
			}
		}
	}()
}

// RestartActiveService attempts to restart the running service daemon.
func RestartActiveService() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// 1. systemd
	if out, err := exec.Command("systemctl", "is-active", "justping-agent").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
		if err := exec.Command("systemctl", "restart", "justping-agent").Run(); err == nil {
			fmt.Println("[Updater] Restarted justping-agent service via systemd.")
			return true
		}
	}

	// 2. OpenRC
	if err := exec.Command("rc-service", "justping-agent", "status").Run(); err == nil {
		if err := exec.Command("rc-service", "justping-agent", "restart").Run(); err == nil {
			fmt.Println("[Updater] Restarted justping-agent service via OpenRC.")
			return true
		}
	}

	// 3. SysVinit / procd
	if _, err := os.Stat("/etc/init.d/justping-agent"); err == nil {
		if err := exec.Command("/etc/init.d/justping-agent", "restart").Run(); err == nil {
			fmt.Println("[Updater] Restarted justping-agent service via /etc/init.d.")
			return true
		}
	}

	// 4. runit
	if _, err := os.Stat("/var/service/justping-agent"); err == nil {
		if err := exec.Command("sv", "restart", "justping-agent").Run(); err == nil {
			fmt.Println("[Updater] Restarted justping-agent service via runit.")
			return true
		}
	}

	return false
}
