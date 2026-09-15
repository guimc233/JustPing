#!/bin/bash
set -euo pipefail

# ==============================================================================
# JustPing Agent Verified Installer
# Supports: systemd, OpenRC, OpenWrt procd, runit, SysVinit
# Architectures: amd64, 386, arm64, armv7, armv6, armv5, mips, mipsle, riscv64
# ==============================================================================

REPO="guimc233/JustPing"
DEFAULT_VERSION="latest"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -s|--server) SERVER_URL="$2"; shift 2 ;;
        -t|--token) TOKEN="$2"; shift 2 ;;
        -v|--version) VERSION="$2"; shift 2 ;;
        -h|--help) print_help; exit 0 ;;
        *) echo -e "${RED}Unknown argument: $1${NC}"; print_help; exit 1 ;;
    esac
done

if [ -z "${SERVER_URL}" ] || [ -z "${TOKEN}" ]; then
    echo -e "${RED}Error: Both --server and --token are required.${NC}"
    print_help
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}Error: This script must be run as root (or using sudo).${NC}"
    exit 1
fi

echo -e "${BLUE}==> Installing JustPing Agent...${NC}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "${OS}" != "linux" ]; then
    echo -e "${RED}Error: Unsupported operating system: ${OS}.${NC}"
    exit 1
fi

ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64) ARCH="amd64" ;;
    i386|i486|i586|i686) ARCH="386" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armv7) ARCH="armv7" ;;
    armv6l|armv6) ARCH="armv6" ;;
    armv5*|arm) ARCH="armv5" ;;
    mips) ARCH="mips" ;;
    mipsel|mipsle) ARCH="mipsle" ;;
    mips64) ARCH="mips64" ;;
    mips64el|mips64le) ARCH="mips64le" ;;
    riscv64) ARCH="riscv64" ;;
    s390x) ARCH="s390x" ;;
    ppc64le) ARCH="ppc64le" ;;
    *) echo -e "${RED}Error: Unsupported architecture: ${ARCH_RAW}${NC}"; exit 1 ;;
esac

echo -e "${GREEN}Detected platform: ${OS}-${ARCH}${NC}"

TMP_DIR="$(mktemp -d /tmp/justping-install.XXXXXX)"
trap 'rm -rf "${TMP_DIR}"' EXIT

BIN_NAME="justping-agent-${OS}-${ARCH}"
TMP_BIN="${TMP_DIR}/${BIN_NAME}"
CHECKSUMS_FILE="${TMP_DIR}/SHA256SUMS.txt"

if [ "${VERSION}" = "latest" ]; then
    BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
    BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

DOWNLOAD_URL="${BASE_URL}/${BIN_NAME}"
CHECKSUM_URL="${BASE_URL}/SHA256SUMS.txt"

echo -e "${BLUE}==> Downloading JustPing agent binary from GitHub Release...${NC}"
if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${TMP_BIN}" "${DOWNLOAD_URL}"
    curl -fsSL -o "${CHECKSUMS_FILE}" "${CHECKSUM_URL}"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "${TMP_BIN}" "${DOWNLOAD_URL}"
    wget -qO "${CHECKSUMS_FILE}" "${CHECKSUM_URL}"
else
    echo -e "${RED}Error: Neither curl nor wget is installed.${NC}"
    exit 1
fi

if [ ! -s "${CHECKSUMS_FILE}" ]; then
    echo -e "${RED}Error: Failed to retrieve SHA256 checksums file from ${CHECKSUM_URL}.${NC}"
    exit 1
fi

echo -e "${BLUE}==> Verifying SHA256 checksum...${NC}"
EXPECTED_HASH="$(grep -E "[\t ]${BIN_NAME}\$" "${CHECKSUMS_FILE}" | awk '{print $1}' || true)"
if [ -z "${EXPECTED_HASH}" ]; then
    echo -e "${RED}Error: Checksum for ${BIN_NAME} not found in SHA256SUMS.txt!${NC}"
    exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_HASH="$(sha256sum "${TMP_BIN}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL_HASH="$(shasum -a 256 "${TMP_BIN}" | awk '{print $1}')"
else
    echo -e "${RED}Error: Neither sha256sum nor shasum tool available on this system.${NC}"
    exit 1
fi

if [ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]; then
    echo -e "${RED}Error: Checksum verification mismatch! (Expected: ${EXPECTED_HASH}, Got: ${ACTUAL_HASH})${NC}"
    exit 1
fi
echo -e "${GREEN}Checksum verified successfully.${NC}"

chmod +x "${TMP_BIN}"

# Write temporary 0600 config file so secret token never appears on process argv
CFG_TMP="${TMP_DIR}/config.json"
cat <<EOF > "${CFG_TMP}"
{
  "server": "${SERVER_URL}",
  "token": "${TOKEN}"
}
EOF
chmod 0600 "${CFG_TMP}"

echo -e "${BLUE}==> Installing agent service...${NC}"
"${TMP_BIN}" --install --config "${CFG_TMP}"

echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  JustPing Agent successfully installed and verified! ${NC}"
echo -e "${GREEN}=====================================================${NC}"
