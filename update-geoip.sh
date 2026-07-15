#!/bin/bash
# Daily update of GeoLite2-City.mmdb and GeoLite2-ASN.mmdb from MaxMind.
# Place in crontab: 30 3 * * * /path/to/update-geoip.sh >> /var/log/geoip-update.log 2>&1

set -euo pipefail

# Get your free license key at https://www.maxmind.com/en/accounts/current/license-key
MAXMIND_LICENSE_KEY="__YOUR_MAXMIND_LICENSE_KEY__"

# Directory where this script lives (same dir as docker-compose.yml)
WORKDIR="$(cd "$(dirname "$0")" && pwd)"

TMP_DIR="$(mktemp -d /tmp/geoip_update.XXXXXX)"

cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Helper: download and extract one MaxMind edition
# Usage: fetch_edition <edition_id> <target_path>
# ---------------------------------------------------------------------------
fetch_edition() {
    local EDITION_ID="$1"
    local TARGET="$2"
    local DOWNLOAD_URL="https://download.maxmind.com/app/geoip_download?edition_id=${EDITION_ID}&license_key=${MAXMIND_LICENSE_KEY}&suffix=tar.gz"
    local TMP_ARCHIVE="${TMP_DIR}/${EDITION_ID}.tar.gz"

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Downloading ${EDITION_ID}..."
    curl -fsSL \
        --connect-timeout 30 \
        --max-time 600 \
        --retry 3 \
        --retry-delay 5 \
        --retry-all-errors \
        -o "${TMP_ARCHIVE}" \
        "${DOWNLOAD_URL}"

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Extracting ${EDITION_ID}..."
    tar -xzf "${TMP_ARCHIVE}" -C "${TMP_DIR}" --wildcards --no-anchored "*.mmdb"

    local MMDB_PATH
    MMDB_PATH="$(find "${TMP_DIR}" -name "${EDITION_ID}.mmdb" | head -n1)"
    if [[ -z "${MMDB_PATH}" ]]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: ${EDITION_ID}.mmdb not found in archive."
        return 1
    fi

    # Cross-device move: always results in a fresh inode at TARGET
    mv -f "${MMDB_PATH}" "${TARGET}"
    chmod 644 "${TARGET}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Updated: ${TARGET}"
}

# ---------------------------------------------------------------------------
# Download both databases
# ---------------------------------------------------------------------------
fetch_edition "GeoLite2-City" "${WORKDIR}/GeoLite2-City.mmdb"
fetch_edition "GeoLite2-ASN"  "${WORKDIR}/GeoLite2-ASN.mmdb"

# ---------------------------------------------------------------------------
# Restart analyzer so the new inodes are picked up by the bind mounts.
# (mv swaps the inode; a bind mount pins the old one until remounted.)
# ---------------------------------------------------------------------------
if docker compose -f "${WORKDIR}/docker-compose.yml" restart analyzer-server; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] analyzer-server restarted, new GeoIP DBs loaded."
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: failed to restart analyzer-server; new DBs load on next restart."
fi
