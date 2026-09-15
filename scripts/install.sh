#!/bin/bash
set -euo pipefail

# ==============================================================================
# JustPing Agent Verified Installer
# Supports: systemd, OpenRC, OpenWrt procd, runit, SysVinit
# Architectures: amd64, 386, arm64, armv7, armv6, armv5, mips, mipsle, riscv64
# ==============================================================================

REPO="guimc233/JustPing"
DEFAULT_VERSION="latest"

# GitHub mirror prefixes used by --china-mirror. Each entry is prepended to the
# full GitHub release URL, e.g. "<mirror>/https://github.com/<owner>/<repo>/...".
CHINA_MIRRORS=(
    "https://ghfast.top"
    "https://gh-proxy.com"
    "https://ghproxy.net"
    "https://gh.llkk.cc"
    "https://gh-proxy.ygxz.in"
    "https://gh.xxooo.cf"
    "https://gh-proxy.net"
    "https://cors.isteed.cc"
)
MIRROR_TIMEOUT=8

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVER_URL=""
TOKEN=""
VERSION="${DEFAULT_VERSION}"
CHINA_MIRROR=0
UNINSTALL=0

print_help() {
    echo "Usage: curl -fsSL <HOST>/install.sh | sudo bash -s -- --server <URL> --token <TOKEN>"
    echo "       curl -fsSL <HOST>/install.sh | sudo bash -s -- --uninstall"
    echo ""
    echo "Options:"
    echo "  -s, --server <URL>     Host URL (e.g. https://ping.example.com)"
    echo "  -t, --token <TOKEN>    Agent enrollment token"
    echo "  -v, --version <TAG>    Agent version (default: latest)"
    echo "      --china-mirror     Probe GitHub mirrors in parallel and use the fastest one"
    echo "      --uninstall        Uninstall and remove JustPing Agent service and files"
    echo "  -h, --help             Show this help message"
}

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -s|--server) SERVER_URL="$2"; shift 2 ;;
        -t|--token) TOKEN="$2"; shift 2 ;;
        -v|--version) VERSION="$2"; shift 2 ;;
        --china-mirror) CHINA_MIRROR=1; shift ;;
        --uninstall|uninstall) UNINSTALL=1; shift ;;
        -h|--help) print_help; exit 0 ;;
        *) echo -e "${RED}Unknown argument: $1${NC}"; print_help; exit 1 ;;
    esac
done

if [ "${UNINSTALL}" -eq 1 ]; then
    if [ "$(id -u)" -ne 0 ]; then
        echo -e "${RED}Error: This script must be run as root (or using sudo).${NC}"
        exit 1
    fi

    echo -e "${BLUE}==> Uninstalling JustPing Agent...${NC}"

    # 1. systemd
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop justping-agent 2>/dev/null || true
        systemctl disable justping-agent 2>/dev/null || true
        rm -f /etc/systemd/system/justping-agent.service
        systemctl daemon-reload 2>/dev/null || true
    fi

    # 2. OpenRC
    if command -v rc-service >/dev/null 2>&1; then
        rc-service justping-agent stop 2>/dev/null || true
        rc-update del justping-agent default 2>/dev/null || true
    fi

    # 3. OpenWrt procd / SysVinit
    if [ -f /etc/init.d/justping-agent ]; then
        /etc/init.d/justping-agent stop 2>/dev/null || true
        if command -v update-rc.d >/dev/null 2>&1; then
            update-rc.d -f justping-agent remove 2>/dev/null || true
        elif command -v chkconfig >/dev/null 2>&1; then
            chkconfig --del justping-agent 2>/dev/null || true
        fi
        /etc/init.d/justping-agent disable 2>/dev/null || true
        rm -f /etc/init.d/justping-agent
    fi

    # 4. runit
    if command -v sv >/dev/null 2>&1; then
        sv down justping-agent 2>/dev/null || true
    fi
    rm -f /var/service/justping-agent /etc/service/justping-agent
    rm -rf /etc/sv/justping-agent

    # 5. Remove PID files if any
    rm -f /run/justping-agent.pid /var/run/justping-agent.pid

    # 6. Remove binary files
    rm -f /usr/local/bin/justping-agent /usr/bin/justping-agent

    # 7. Remove config directory
    rm -rf /etc/justping

    echo -e "${GREEN}JustPing agent uninstalled successfully.${NC}"
    exit 0
fi

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
    RELEASE_PATH="/${REPO}/releases/latest/download"
else
    RELEASE_PATH="/${REPO}/releases/download/${VERSION}"
fi
GITHUB_BASE="https://github.com${RELEASE_PATH}"

if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
else
    echo -e "${RED}Error: Neither curl nor wget is installed.${NC}"
    exit 1
fi

# wget timeout flags differ between GNU wget and BusyBox. Use them only when
# the installed version advertises support.
WGET_TIMEOUT_ARGS=()
if [ "${DOWNLOADER}" = "wget" ] && wget --help 2>&1 | grep -q -- '-T'; then
    WGET_TIMEOUT_ARGS=(-T "${MIRROR_TIMEOUT}")
fi

# mirror_url <mirror> <github-url> -> "<mirror>/https://github.com/..."
mirror_url() {
    printf '%s/%s' "$1" "$2"
}

# fetch <url> <output-file>
fetch() {
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -fsSL --connect-timeout "${MIRROR_TIMEOUT}" --max-time 300 \
            --retry 2 --retry-delay 1 -o "$2" "$1"
    else
        wget -q "${WGET_TIMEOUT_ARGS[@]+"${WGET_TIMEOUT_ARGS[@]}"}" -O "$2" "$1"
    fi
}

# now_ns: nanoseconds since epoch, or 0 when the platform cannot report them.
now_ns() {
    local value
    value="$(date +%s%N 2>/dev/null || echo 0)"
    case "${value}" in
        ''|*[!0-9]*) printf '0' ;;
        *) printf '%s' "${value}" ;;
    esac
}

# probe_url <url> <probe-file>: download a small asset and print elapsed seconds.
# Measuring the real checksum file both ranks mirrors by transfer speed and
# rejects mirrors that serve error pages instead of release assets.
probe_url() {
    local url="$1" target="$2" latency start end
    if [ "${DOWNLOADER}" = "curl" ]; then
        latency="$(curl -fsSL -o "${target}" \
            --connect-timeout 3 --max-time "${MIRROR_TIMEOUT}" \
            -w '%{time_total}' "${url}" 2>/dev/null)" || return 1
    else
        start="$(now_ns)"
        wget -q "${WGET_TIMEOUT_ARGS[@]+"${WGET_TIMEOUT_ARGS[@]}"}" \
            -O "${target}" "${url}" 2>/dev/null || return 1
        end="$(now_ns)"
        latency="$(awk -v s="${start}" -v e="${end}" \
            'BEGIN { if (s > 0 && e >= s) printf "%.3f", (e - s) / 1000000000; else printf "0" }')"
    fi
    [ -n "${latency}" ] || return 1
    grep -qE "[[:space:]]${BIN_NAME}\$" "${target}" 2>/dev/null || return 1
    printf '%s' "${latency}"
}

DOWNLOAD_CANDIDATES=()

if [ "${CHINA_MIRROR}" -eq 1 ]; then
    echo -e "${BLUE}==> Probing ${#CHINA_MIRRORS[@]} GitHub mirrors in parallel...${NC}"
    PROBE_DIR="${TMP_DIR}/probe"
    mkdir -p "${PROBE_DIR}"

    probe_index=0
    for mirror in "${CHINA_MIRRORS[@]}"; do
        probe_index=$((probe_index + 1))
        (
            latency="$(probe_url "$(mirror_url "${mirror}" "${GITHUB_BASE}/SHA256SUMS.txt")" \
                "${PROBE_DIR}/${probe_index}.sha256")" || exit 1
            printf '%s %s\n' "${latency}" "${mirror}" > "${PROBE_DIR}/${probe_index}.result"
        ) &
    done
    wait || true

    # Reachable mirrors, fastest transfer first.
    RANKED_MIRRORS=()
    while read -r _latency mirror; do
        [ -n "${mirror}" ] || continue
        RANKED_MIRRORS+=("${mirror}")
        DOWNLOAD_CANDIDATES+=("$(mirror_url "${mirror}" "${GITHUB_BASE}")")
        echo -e "${GREEN}    reachable: ${mirror} (${_latency}s)${NC}"
    done < <(cat "${PROBE_DIR}"/*.result 2>/dev/null | sort -n)

    if [ "${#RANKED_MIRRORS[@]}" -eq 0 ]; then
        echo -e "${YELLOW}    No mirror responded; trying all of them in order.${NC}"
        RANKED_MIRRORS=("${CHINA_MIRRORS[@]}")
    fi

    # Mirrors that timed out still get a chance if every probed one fails.
    for mirror in "${CHINA_MIRRORS[@]}"; do
        for ranked in "${RANKED_MIRRORS[@]}"; do
            [ "${ranked}" = "${mirror}" ] && continue 2
        done
        DOWNLOAD_CANDIDATES+=("$(mirror_url "${mirror}" "${GITHUB_BASE}")")
    done
fi

# Direct GitHub is always the last resort.
DOWNLOAD_CANDIDATES+=("${GITHUB_BASE}")

# candidate_looks_valid: reject empty files and HTML error pages that some
# mirrors return with a 200 status code for missing assets.
candidate_looks_valid() {
    [ -s "${TMP_BIN}" ] && [ -s "${CHECKSUMS_FILE}" ] || return 1
    [ "$(head -c 4 "${TMP_BIN}" | od -An -tx1 | tr -d ' \n')" = "7f454c46" ] || return 1
    grep -qE "[[:space:]]${BIN_NAME}\$" "${CHECKSUMS_FILE}" || return 1
    return 0
}

echo -e "${BLUE}==> Downloading JustPing agent binary from GitHub Release...${NC}"
DOWNLOAD_OK=0
for candidate in "${DOWNLOAD_CANDIDATES[@]}"; do
    if [ "${candidate}" = "${GITHUB_BASE}" ]; then
        echo -e "${BLUE}    source: GitHub (${DOWNLOADER})${NC}"
    else
        echo -e "${BLUE}    source: ${candidate%%/https://*}${NC}"
    fi

    if fetch "${candidate}/${BIN_NAME}" "${TMP_BIN}" && \
       fetch "${candidate}/SHA256SUMS.txt" "${CHECKSUMS_FILE}" && \
       candidate_looks_valid; then
        DOWNLOAD_OK=1
        break
    fi

    echo -e "${YELLOW}    Download failed, trying next source...${NC}"
    rm -f "${TMP_BIN}" "${CHECKSUMS_FILE}"
done

if [ "${DOWNLOAD_OK}" -ne 1 ]; then
    echo -e "${RED}Error: Failed to download the agent binary from any source.${NC}"
    exit 1
fi

echo -e "${BLUE}==> Verifying SHA256 checksum...${NC}"
EXPECTED_HASH="$(grep -E "[[:space:]]${BIN_NAME}\$" "${CHECKSUMS_FILE}" | awk '{print $1}' || true)"
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
CHINA_MIRROR_JSON="false"
if [ "${CHINA_MIRROR}" -eq 1 ]; then
    CHINA_MIRROR_JSON="true"
fi
cat <<EOF > "${CFG_TMP}"
{
  "server": "${SERVER_URL}",
  "token": "${TOKEN}",
  "china_mirror": ${CHINA_MIRROR_JSON}
}
EOF
chmod 0600 "${CFG_TMP}"

echo -e "${BLUE}==> Installing agent service...${NC}"
INSTALL_ARGS=("--install" "--config" "${CFG_TMP}")
if [ "${CHINA_MIRROR}" -eq 1 ]; then
    INSTALL_ARGS+=("--china-mirror")
fi
"${TMP_BIN}" "${INSTALL_ARGS[@]}"

echo -e "${GREEN}=====================================================${NC}"
echo -e "${GREEN}  JustPing Agent successfully installed and verified! ${NC}"
echo -e "${GREEN}=====================================================${NC}"
