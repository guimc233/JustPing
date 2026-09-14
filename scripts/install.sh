#!/bin/bash
set -e

# ==============================================================================
# JustPing Agent One-Line Installer
# Supports: systemd, OpenRC, OpenWrt procd, runit, SysVinit
# Architectures: amd64, 386, arm64, armv7, armv6, armv5, mips, mipsel, riscv64
# ==============================================================================

REPO="guimc233/JustPing"
DEFAULT_VERSION="latest"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SERVER_URL=""
TOKEN=""
VERSION="${DEFAULT_VERSION}"

print_help() {
    echo "Usage: curl -fsSL <HOST>/install.sh | sudo bash -s -- --server <URL> --token <TOKEN>"
    echo ""
    echo "Options:"
    echo "  -s, --server <URL>     Host URL (e.g. https://ping.example.com)"
    echo "  -t, --token <TOKEN>    Agent enrollment token"
    echo "  -v, --version <TAG>    Agent version (default: latest)"
    echo "  -h, --help             Show this help message"
}

# Parse Arguments
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -s|--server)
            SERVER_URL="$2"
            shift 2
            ;;
        -t|--token)
            TOKEN="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -h|--help)
            print_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            print_help
            exit 1
            ;;
    esac
done

if [ -z "${SERVER_URL}" ] || [ -z "${TOKEN}" ]; then
    echo -e "${RED}Error: Both --server and --token are required.${NC}"
    print_help
    exit 1
fi

# Check root privilege
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}Error: This script must be run as root (or using sudo).${NC}"
    exit 1
fi

echo -e "${BLUE}==> Installing JustPing Agent...${NC}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "${OS}" != "linux" ]; then
    echo -e "${RED}Error: Unsupported operating system: ${OS}. Currently only Linux is supported by this installer.${NC}"
    exit 1
fi

# Detect Architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    i386|i486|i586|i686)
        ARCH="386"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l|armv7)
        ARCH="armv7"
        ;;
    armv6l|armv6)
        ARCH="armv6"
        ;;
    armv5*|arm)
        ARCH="armv5"
        ;;
    mips)
        ARCH="mips"
        ;;
    mipsel|mipsle)
        ARCH="mipsel"
        ;;
    mips64)
        ARCH="mips64"
        ;;
    mips64el|mips64le)
        ARCH="mips64el"
        ;;
    riscv64)
        ARCH="riscv64"
        ;;
    s390x)
        ARCH="s390x"
        ;;
    ppc64le)
        ARCH="ppc64le"
        ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: ${ARCH_RAW}${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}Detected platform: ${OS}-${ARCH}${NC}"

BIN_NAME="justping-agent-${OS}-${ARCH}"
TMP_BIN="/tmp/justping-agent-tmp"
rm -f "${TMP_BIN}"

# Determine Download URL
if [ "${VERSION}" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}"
fi

echo -e "${BLUE}==> Downloading JustPing agent binary from ${DOWNLOAD_URL}...${NC}"

# Download using curl or wget
if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${TMP_BIN}" "${DOWNLOAD_URL}" || true
elif command -v wget >/dev/null 2>&1; then
    wget -qO "${TMP_BIN}" "${DOWNLOAD_URL}" || true
else
    echo -e "${RED}Error: Neither curl nor wget is installed.${NC}"
    exit 1
fi

if [ ! -s "${TMP_BIN}" ]; then
    echo -e "${YELLOW}Notice: Could not download pre-built binary from GitHub release directly.${NC}"
    echo -e "${YELLOW}Attempting to download from host server if hosted...${NC}"
    FALLBACK_URL="${SERVER_URL}/binaries/${BIN_NAME}"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "${TMP_BIN}" "${FALLBACK_URL}" || true
    fi
fi

if [ ! -s "${TMP_BIN}" ]; then
    echo -e "${RED}Failed to download agent binary. Please verify network connectivity or version release.${NC}"
    exit 1
fi

chmod +x "${TMP_BIN}"

echo -e "${BLUE}==> Installing service...${NC}"
"${TMP_BIN}" --install --server "${SERVER_URL}" --token "${TOKEN}"

rm -f "${TMP_BIN}"

echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  JustPing Agent successfully installed and started! ${NC}"
echo -e "${GREEN}=====================================================${NC}"
