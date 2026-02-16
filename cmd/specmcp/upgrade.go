package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GitHubRelease represents the structure we need from GitHub API
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

func handleUpgradeCommand(args []string) {
	force := false
	quiet := false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
		}
		if arg == "--quiet" || arg == "-q" {
			quiet = true
		}
	}

	fmt.Printf("Checking for updates... (Current version: %s)\n", Version)

	// 1. Get latest version
	latestRelease, err := getLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching latest version: %v\n", err)
		os.Exit(1)
	}

	latestVersion := latestRelease.TagName
	if !force {
		// Handle "v" prefix inconsistencies
		cleanCurrent := strings.TrimPrefix(Version, "v")
		cleanLatest := strings.TrimPrefix(latestVersion, "v")

		// Simple string comparison for now, assuming semantic versioning
		if cleanCurrent == cleanLatest {
			fmt.Printf("SpecMCP is already up to date (%s).\n", Version)
			return
		}
	}

	fmt.Printf("Found new version: %s\n", latestVersion)

	// Show release notes (Priority 3)
	if latestRelease.Body != "" && !quiet {
		fmt.Printf("\n=== What's New in %s ===\n", latestVersion)
		fmt.Println(latestRelease.Body)
		fmt.Println("=====================================\n")
	}

	// Pre-upgrade health checks (Priority 4)
	if err := runPreUpgradeChecks(force); err != nil {
		fmt.Fprintf(os.Stderr, "Pre-upgrade check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Upgrading...")

	// 2. Detect platform
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	// Validate platform against supported ones
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "Unsupported OS for automatic upgrade: %s\n", runtime.GOOS)
		os.Exit(1)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		fmt.Fprintf(os.Stderr, "Unsupported architecture for automatic upgrade: %s\n", runtime.GOARCH)
		os.Exit(1)
	}

	// 3. Construct URL
	// https://github.com/emergent-company/specmcp/releases/download/v1.0.0/specmcp-darwin-arm64.tar.gz
	downloadURL := fmt.Sprintf("https://github.com/emergent-company/specmcp/releases/download/%s/specmcp-%s.tar.gz", latestVersion, platform)

	// 4. Download and Extract
	tmpDir, err := os.MkdirTemp("", "specmcp-upgrade")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Downloading from %s...\n", downloadURL)

	tarballPath := filepath.Join(tmpDir, "specmcp.tar.gz")
	if err := downloadFile(downloadURL, tarballPath); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Extracting...")
	binaryPath, err := extractBinary(tarballPath, tmpDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", err)
		os.Exit(1)
	}

	// 5. Replace Binary
	currentExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining executable path: %v\n", err)
		os.Exit(1)
	}

	// Resolve symlinks to find the real binary
	realExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving symlinks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installing to %s...\n", realExe)

	// Move current binary to backup
	backupExe := realExe + ".old"
	if err := os.Rename(realExe, backupExe); err != nil {
		// Try to handle permission denied or other errors
		if os.IsPermission(err) {
			fmt.Fprintf(os.Stderr, "Permission denied. Please run with sudo:\n  sudo specmcp upgrade\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error moving current binary: %v\n", err)
		}
		os.Exit(1)
	}

	// Move new binary to location
	// We use copyFile instead of Rename because tmpDir might be on a different filesystem
	if err := copyFile(binaryPath, realExe); err != nil {
		// Restore backup
		os.Rename(backupExe, realExe)
		fmt.Fprintf(os.Stderr, "Error installing new binary: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chmod(realExe, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to chmod new binary: %v\n", err)
	}

	// Keep backup for rollback instead of deleting it
	fmt.Printf("\nBackup of previous version saved at: %s\n", backupExe)
	fmt.Println("To rollback if needed: specmcp rollback")

	// Auto-verify installation (Priority 1)
	fmt.Println("\nVerifying installation...")
	cmd := exec.Command(realExe, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Warning: failed to verify installation: %v\n", err)
	} else {
		installedVersion := strings.TrimSpace(string(output))
		if strings.Contains(installedVersion, latestVersion) {
			fmt.Printf("✓ Verification successful: %s\n", installedVersion)
		} else {
			fmt.Fprintf(os.Stderr, "⚠ Verification failed: expected %s, got %s\n",
				latestVersion, installedVersion)
			fmt.Fprintf(os.Stderr, "To restore backup: sudo mv %s %s\n", backupExe, realExe)
			os.Exit(1)
		}
	}

	// Service restart on Arch Linux (Priority 2)
	if runtime.GOOS == "linux" && isArchLinux() {
		serviceActive, _ := exec.Command("systemctl", "is-active", "specmcp").Output()
		if strings.TrimSpace(string(serviceActive)) == "active" {
			fmt.Println("\nRestarting systemd service...")
			cmd := exec.Command("systemctl", "restart", "specmcp")
			if err := cmd.Run(); err != nil {
				if os.IsPermission(err) {
					fmt.Println("⚠ Permission denied. Please restart manually:")
					fmt.Println("  sudo systemctl restart specmcp")
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to restart service: %v\n", err)
					fmt.Println("Please restart manually: sudo systemctl restart specmcp")
				}
			} else {
				fmt.Println("✓ Service restarted successfully")

				// Verify service is running
				time.Sleep(2 * time.Second)
				statusCmd := exec.Command("systemctl", "is-active", "specmcp")
				if output, err := statusCmd.Output(); err == nil {
					if strings.TrimSpace(string(output)) == "active" {
						fmt.Println("✓ Service is running")
					} else {
						fmt.Println("⚠ Service may have failed to start. Check: systemctl status specmcp")
					}
				}
			}
		}
	}

	fmt.Printf("\nSuccessfully upgraded to %s\n", latestVersion)
}

func getLatestRelease() (*GitHubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/emergent-company/specmcp/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(tarballPath, destDir string) (string, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// We are looking for the 'specmcp' binary
		cleanName := filepath.Base(header.Name)
		if cleanName == "specmcp" {
			destPath := filepath.Join(destDir, "specmcp-new")

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			// Copy allows for limited memory usage vs ReadAll
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			// Ensure it's executable
			os.Chmod(destPath, 0755)

			return destPath, nil
		}
	}
	return "", fmt.Errorf("binary 'specmcp' not found in archive")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// runPreUpgradeChecks performs health checks before upgrading
func runPreUpgradeChecks(force bool) error {
	fmt.Println("\nRunning pre-upgrade checks...")

	// Check if running as service
	if runtime.GOOS == "linux" && isArchLinux() {
		serviceActive, _ := exec.Command("systemctl", "is-active", "specmcp").Output()
		if strings.TrimSpace(string(serviceActive)) == "active" {
			fmt.Println("⚠ SpecMCP service is currently running")
			fmt.Println("  The service will be restarted after upgrade")

			if !force {
				fmt.Print("\nContinue with upgrade? [y/N] ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					return fmt.Errorf("upgrade cancelled by user")
				}
			}
		}
	}

	return nil
}

// isArchLinux detects if running on Arch Linux or derivatives
func isArchLinux() bool {
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return true
	}
	if _, err := os.Stat("/etc/manjaro-release"); err == nil {
		return true
	}
	return false
}

// handleRollbackCommand restores the previous version from backup
func handleRollbackCommand() {
	currentExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining executable path: %v\n", err)
		os.Exit(1)
	}

	realExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving symlinks: %v\n", err)
		os.Exit(1)
	}

	backupExe := realExe + ".old"

	if _, err := os.Stat(backupExe); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No backup found at %s\n", backupExe)
		fmt.Println("Rollback is only possible after an upgrade that kept a backup.")
		os.Exit(1)
	}

	fmt.Println("Rolling back to previous version...")

	// Get version from backup
	cmd := exec.Command(backupExe, "version")
	output, err := cmd.CombinedOutput()
	var oldVersion string
	if err == nil {
		oldVersion = strings.TrimSpace(string(output))
		fmt.Printf("Restoring: %s\n", oldVersion)
	} else {
		fmt.Println("Restoring previous version...")
	}

	// Move current to .failed
	if err := os.Rename(realExe, realExe+".failed"); err != nil {
		if os.IsPermission(err) {
			fmt.Fprintf(os.Stderr, "Permission denied. Please run with sudo:\n  sudo specmcp rollback\n")
		} else {
			fmt.Fprintf(os.Stderr, "Failed to move current binary: %v\n", err)
		}
		os.Exit(1)
	}

	// Restore backup
	if err := os.Rename(backupExe, realExe); err != nil {
		// Restore current
		os.Rename(realExe+".failed", realExe)
		fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
		os.Exit(1)
	}

	// Cleanup failed binary
	os.Remove(realExe + ".failed")

	if oldVersion != "" {
		fmt.Printf("✓ Successfully rolled back to %s\n", oldVersion)
	} else {
		fmt.Println("✓ Successfully rolled back to previous version")
	}

	// Restart service if on Arch Linux
	if runtime.GOOS == "linux" && isArchLinux() {
		serviceActive, _ := exec.Command("systemctl", "is-active", "specmcp").Output()
		if strings.TrimSpace(string(serviceActive)) == "active" {
			fmt.Println("\nRestarting systemd service...")
			cmd := exec.Command("systemctl", "restart", "specmcp")
			if err := cmd.Run(); err != nil {
				fmt.Println("⚠ Please restart the service manually: sudo systemctl restart specmcp")
			} else {
				fmt.Println("✓ Service restarted")
			}
		}
	}

	fmt.Println("\nRollback complete. Run 'specmcp version' to verify.")
}
