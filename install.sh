#!/bin/sh
# SpecMCP Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/emergent-company/specmcp/main/install.sh | sh
#
# Environment variables:
#   SPECMCP_VERSION  - Specific version to install (default: latest)
#   SPECMCP_DIR      - Installation directory (default: ~/.specmcp)

set -e

# Configuration
GITHUB_REPO="emergent-company/specmcp"
BINARY_NAME="specmcp"
DEFAULT_INSTALL_DIR="${HOME}/.specmcp"

# Colors (if terminal supports them)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    MUTED='\033[0;2m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    MUTED=''
    NC=''
fi

info() {
    printf "${BLUE}==>${NC} %s\n" "$1"
}

success() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1"
}

error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Darwin)
            OS="darwin"
            ;;
        Linux)
            OS="linux"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
}

# Check for Arch Linux / Pacman
is_arch_linux() {
    if [ -f "/etc/arch-release" ] || [ -f "/etc/manjaro-release" ]; then
        return 0
    fi
    if command -v pacman > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# Get latest version from GitHub releases
get_latest_version() {
    printf "${BLUE}==>${NC} Fetching latest version...\n" >&2
    LATEST=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST" ]; then
        error "Failed to get latest version. Check your internet connection."
    fi
    
    echo "$LATEST"
}

# Install using Pacman (Arch Linux)
install_arch() {
    VERSION="${SPECMCP_VERSION:-$(get_latest_version)}"
    # Strip 'v' prefix if present for PKGBUILD versioning compatibility
    CLEAN_VERSION="${VERSION#v}"
    
    info "Arch Linux detected. Installing SpecMCP ${VERSION}..."

    # Check if service is running before upgrade
    SERVICE_WAS_RUNNING=0
    if systemctl is-active --quiet specmcp 2>/dev/null; then
        info "Stopping SpecMCP service..."
        SERVICE_WAS_RUNNING=1
        if command -v sudo > /dev/null 2>&1; then
            sudo systemctl stop specmcp
        else
            if [ "$(id -u)" -eq 0 ]; then
                systemctl stop specmcp
            else
                su -c "systemctl stop specmcp"
            fi
        fi
    fi

    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT
    
    # URL for the pre-built package
    ARCH_PKG_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/specmcp-${CLEAN_VERSION}-1-x86_64.pkg.tar.zst"
    
    if [ "$(uname -m)" != "x86_64" ]; then
        warn "Pre-built Arch packages are only available for x86_64. Falling back to generic install."
        install_generic
        return
    fi
    
    info "Downloading pre-built package: ${ARCH_PKG_URL}"
    
    if curl -fsSL "${ARCH_PKG_URL}" -o "${TMP_DIR}/specmcp.pkg.tar.zst"; then
        info "Installing package..."
        if command -v sudo > /dev/null 2>&1; then
            sudo pacman -U --noconfirm --overwrite '*' "${TMP_DIR}/specmcp.pkg.tar.zst"
        else
            if [ "$(id -u)" -eq 0 ]; then
                pacman -U --noconfirm --overwrite '*' "${TMP_DIR}/specmcp.pkg.tar.zst"
            else
                su -c "pacman -U --noconfirm --overwrite '*' ${TMP_DIR}/specmcp.pkg.tar.zst"
            fi
        fi
        success "SpecMCP installed successfully via Pacman!"
        
        # Restart service if it was running
        if [ "$SERVICE_WAS_RUNNING" = "1" ]; then
            info "Restarting SpecMCP service..."
            sleep 1
            if command -v sudo > /dev/null 2>&1; then
                sudo systemctl start specmcp
            else
                if [ "$(id -u)" -eq 0 ]; then
                    systemctl start specmcp
                else
                    su -c "systemctl start specmcp"
                fi
            fi
            sleep 2
            if systemctl is-active --quiet specmcp 2>/dev/null; then
                success "Service restarted successfully"
            else
                warn "Service may have failed to start. Check: systemctl status specmcp"
            fi
        else
            echo ""
            info "Service installed at /usr/lib/systemd/system/specmcp.service"
            info "Config installed at /etc/specmcp/config.toml"
            echo ""
            info "To enable and start the service:"
            echo "  sudo systemctl enable --now specmcp"
            echo ""
            info "To check status:"
            echo "  sudo systemctl status specmcp"
            echo ""
        fi
    else
        warn "Pre-built package not found (maybe release is still building?). Falling back to generic install."
        install_generic
    fi
}

# Generic Download and install (macOS, non-Arch Linux)
install_generic() {
    INSTALL_DIR="${SPECMCP_DIR:-$DEFAULT_INSTALL_DIR}"
    VERSION="${SPECMCP_VERSION:-$(get_latest_version)}"
    
    detect_platform
    
    info "Installing SpecMCP ${VERSION} to ${INSTALL_DIR}..."
    
    # Create installation directory
    mkdir -p "${INSTALL_DIR}/bin"
    mkdir -p "${INSTALL_DIR}/data"
    mkdir -p "${INSTALL_DIR}/logs"
    
    # Construct download URL
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}.tar.gz"
    
    info "Downloading from: ${DOWNLOAD_URL}"
    
    # Download and extract
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT
    
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/specmcp.tar.gz" || error "Download failed. Check if version ${VERSION} exists."
    
    # Extract
    tar -xzf "${TMP_DIR}/specmcp.tar.gz" -C "${TMP_DIR}"
    
    # Install binary
    if [ -f "${TMP_DIR}/specmcp" ]; then
        mv "${TMP_DIR}/specmcp" "${INSTALL_DIR}/bin/specmcp"
    else
        error "Binary not found in tarball"
    fi
    chmod +x "${INSTALL_DIR}/bin/specmcp"
    
    success "SpecMCP installed to ${INSTALL_DIR}/bin/specmcp"
    
    # Path warning
    case ":${PATH}:" in
        *":${INSTALL_DIR}/bin:"*)
            ;;
        *)
            echo ""
            warn "Add ${INSTALL_DIR}/bin to your PATH:"
            echo ""
            echo "  export PATH=\"\${HOME}/.specmcp/bin:\${PATH}\""
            echo ""
            ;;
    esac
}

# Uninstall
uninstall() {
    detect_platform
    
    if [ "$OS" = "linux" ] && is_arch_linux; then
        info "Uninstalling via pacman..."
        if command -v sudo > /dev/null 2>&1; then
            sudo pacman -Rns specmcp
        else
            if [ "$(id -u)" -eq 0 ]; then
                pacman -Rns specmcp
            else
                su -c "pacman -Rns specmcp"
            fi
        fi
        success "Uninstalled."
    else
        INSTALL_DIR="${SPECMCP_DIR:-$DEFAULT_INSTALL_DIR}"
        if [ ! -d "${INSTALL_DIR}" ]; then
            error "SpecMCP is not installed at ${INSTALL_DIR}"
        fi
        
        info "Uninstalling from ${INSTALL_DIR}..."
        rm -rf "${INSTALL_DIR}/bin/specmcp"
        
        success "Binaries removed."
        warn "Data directory preserved at ${INSTALL_DIR}/data"
        echo "  To remove completely: rm -rf ${INSTALL_DIR}"
    fi
}

version() {
    detect_platform
    if [ "$OS" = "linux" ] && is_arch_linux; then
        if pacman -Qi specmcp > /dev/null 2>&1; then
            pacman -Qi specmcp | grep Version
        else
            echo "specmcp is not installed via pacman"
        fi
    else
        INSTALL_DIR="${SPECMCP_DIR:-$DEFAULT_INSTALL_DIR}"
        if [ -x "${INSTALL_DIR}/bin/specmcp" ]; then
            "${INSTALL_DIR}/bin/specmcp" version
        else
            echo "specmcp is not installed at ${INSTALL_DIR}/bin/specmcp"
        fi
    fi
}

# Main
main() {
    CMD="${1:-install}"
    detect_platform
    
    case "$CMD" in
        install|upgrade)
            # Check for Arch
            if [ "$OS" = "linux" ] && is_arch_linux; then
                install_arch
            else
                install_generic
            fi
            ;;
        uninstall)
            uninstall
            ;;
        version)
            version
            ;;
        --help|-h)
            echo "SpecMCP Installer"
            echo "Usage: $0 [install|upgrade|uninstall|version]"
            ;;
        *)
            # If no argument or unknown, default to install
            if [ "$OS" = "linux" ] && is_arch_linux; then
                install_arch
            else
                install_generic
            fi
            ;;
    esac
}

main "$@"
