# SpecMCP Upgrade System Analysis & Improvement Suggestions

## Current Implementation Assessment

### ✅ What We Have (Good)

**1. Self-Upgrade Command** (`specmcp upgrade`)
- Downloads latest release from GitHub
- Platform detection (darwin/linux, amd64/arm64)
- Automatic binary replacement with backup
- Permission error handling (suggests sudo)
- Force flag support

**2. Arch Linux Integration**
- Official .pkg.tar.zst package
- systemd service with security hardening
- Config at `/etc/specmcp/config.toml`
- Service management via systemctl
- Proper file locations (`/usr/bin`, `/var/lib/specmcp`, `/var/log/specmcp`)

**3. Install Script**
- Auto-detects Arch Linux and uses pacman
- Falls back to generic install for macOS/other Linux
- PATH management
- Uninstall support

### ❌ What's Missing (Gaps)

**1. No Automatic Version Verification After Upgrade**
- Upgrade completes but doesn't verify the installed version
- Just suggests "Run 'specmcp version' to verify"
- User must manually check

**2. No Service Restart (Arch Linux)**
- After upgrade on Arch, systemd service keeps running old version
- User must manually: `sudo systemctl restart specmcp`
- Install script doesn't handle running services

**3. No Pre-Upgrade Checks**
- Doesn't check if service is running before upgrade
- Doesn't warn about downtime
- No health check after upgrade

**4. No Rollback Mechanism**
- Creates `.old` backup but no automatic rollback
- If upgrade fails, manual recovery required
- No version pinning support

**5. No Release Notes Display**
- Upgrade shows version number but not what changed
- GitHub release notes are available but not shown
- User unaware of breaking changes

## Improvement Suggestions

### Priority 1: Auto-Verify After Upgrade ✨

**What**: Automatically verify installed version matches expected version

**Implementation**:
```go
// After binary replacement in upgrade.go

// Verify installation
fmt.Println("Verifying installation...")
cmd := exec.Command(realExe, "version")
output, err := cmd.CombinedOutput()
if err != nil {
    fmt.Fprintf(os.Stderr, "Warning: failed to verify installation: %v\n", err)
} else {
    installedVersion := strings.TrimSpace(string(output))
    if strings.Contains(installedVersion, latestVersion) {
        fmt.Printf("✓ Verification successful: %s\n", installedVersion)
    } else {
        fmt.Fprintf(os.Stderr, "⚠ Verification failed: expected %s, got %s\n", 
            latestVersion, installedVersion)
        fmt.Fprintf(os.Stderr, "Restore backup: sudo mv %s.old %s\n", realExe, realExe)
    }
}
```

### Priority 2: Service Restart (Arch Linux) 🔄

**What**: Automatically restart systemd service after upgrade on Arch

**Implementation**:
```go
// After successful upgrade in upgrade.go

// Detect and handle systemd service
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

func isArchLinux() bool {
    if _, err := os.Stat("/etc/arch-release"); err == nil {
        return true
    }
    if _, err := os.Stat("/etc/manjaro-release"); err == nil {
        return true
    }
    return false
}
```

### Priority 3: Show Release Notes 📋

**What**: Display what's new in the latest version

**Implementation**:
```go
// In upgrade.go, after fetching latest release

if latestRelease.Body != "" && !quiet {
    fmt.Printf("\n=== What's New in %s ===\n", latestVersion)
    fmt.Println(latestRelease.Body)
    fmt.Println("=====================================\n")
}

// Add --quiet flag to skip release notes
quiet := false
for _, arg := range args {
    if arg == "--quiet" || arg == "-q" {
        quiet = true
    }
}
```

### Priority 4: Pre-Upgrade Health Check 🏥

**What**: Check system health before upgrading

**Implementation**:
```go
// Before download in handleUpgradeCommand

fmt.Println("Running pre-upgrade checks...")

// Check if running as service
if runtime.GOOS == "linux" && isArchLinux() {
    serviceActive, _ := exec.Command("systemctl", "is-active", "specmcp").Output()
    if strings.TrimSpace(string(serviceActive)) == "active" {
        fmt.Println("⚠ SpecMCP service is currently running")
        fmt.Println("  The service will need to be restarted after upgrade")
        
        // Check if HTTP mode - connections will be dropped
        config := loadConfig() // Load current config
        if config.Transport.Mode == "http" {
            fmt.Println("⚠ Running in HTTP mode - active connections will be dropped")
        }
        
        if !force {
            fmt.Print("\nContinue with upgrade? [y/N] ")
            var response string
            fmt.Scanln(&response)
            if response != "y" && response != "Y" {
                fmt.Println("Upgrade cancelled")
                return
            }
        }
    }
}

// Check disk space
// Check write permissions
// etc.
```

### Priority 5: Rollback Support 🔙

**What**: Easy rollback if upgrade fails or causes issues

**Implementation**:
```go
// Add to upgrade.go

// Instead of deleting .old immediately, keep it
// Don't delete: os.Remove(backupExe)

fmt.Printf("\nBackup of previous version kept at: %s\n", backupExe)
fmt.Println("To rollback: specmcp rollback")

// Add new command
func handleRollbackCommand() {
    currentExe, _ := os.Executable()
    realExe, _ := filepath.EvalSymlinks(currentExe)
    backupExe := realExe + ".old"
    
    if _, err := os.Stat(backupExe); os.IsNotExist(err) {
        error("No backup found at %s", backupExe)
    }
    
    fmt.Println("Rolling back to previous version...")
    
    // Get version from backup
    cmd := exec.Command(backupExe, "version")
    output, _ := cmd.CombinedOutput()
    oldVersion := strings.TrimSpace(string(output))
    
    fmt.Printf("Restoring: %s\n", oldVersion)
    
    if err := os.Rename(realExe, realExe+".failed"); err != nil {
        error("Failed to move current binary: %v", err)
    }
    
    if err := os.Rename(backupExe, realExe); err != nil {
        // Restore current
        os.Rename(realExe+".failed", realExe)
        error("Rollback failed: %v", err)
    }
    
    os.Remove(realExe + ".failed")
    success("Rolled back to %s", oldVersion)
    
    // Restart service if needed
    // ...
}
```

### Priority 6: Install Script Service Integration 🛠️

**What**: Handle running services during install/upgrade

**Implementation for install.sh**:
```bash
# In install_arch() function

install_arch() {
    # ... existing code ...
    
    # Check if service is running
    if systemctl is-active --quiet specmcp 2>/dev/null; then
        info "Stopping SpecMCP service..."
        sudo systemctl stop specmcp
        SERVICE_WAS_RUNNING=1
    fi
    
    # ... install package ...
    
    # Restart if it was running
    if [ "$SERVICE_WAS_RUNNING" = "1" ]; then
        info "Restarting SpecMCP service..."
        sudo systemctl start specmcp
        sleep 2
        if systemctl is-active --quiet specmcp; then
            success "Service restarted successfully"
        else
            warn "Service may have failed to start. Check: systemctl status specmcp"
        fi
    fi
}
```

## Recommended Implementation Order

1. **Auto-verify after upgrade** (easy win, high value)
2. **Show release notes** (easy, good UX)
3. **Service restart on Arch** (medium complexity, high value for Arch users)
4. **Pre-upgrade health check** (medium complexity, good safety)
5. **Rollback support** (higher complexity, nice-to-have)
6. **Install script service integration** (easy, completes the picture)

## Summary: What Arch Linux (and others) Do Well

**Arch/systemd best practices we should adopt:**
- ✅ Auto-verify version after upgrade
- ✅ Auto-restart service if running
- ✅ Health check service after restart
- ✅ Show what's being upgraded (release notes)
- ✅ Keep backup for rollback
- ✅ Warn about downtime before upgrade

**Commands to add:**
```bash
specmcp upgrade              # Already exists, enhance it
specmcp upgrade --quiet      # Skip release notes
specmcp rollback             # Restore previous version
specmcp health               # Check if service is healthy (new!)
```

These improvements would make `specmcp upgrade` production-ready and match the UX quality of well-maintained system packages.
