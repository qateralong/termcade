#!/usr/bin/env bash
# First-time installation of Termcade on a Linux server.
#
# Downloads the newest release, verifies its checksum and hands over to
# "termcade install", which sets up the systemd service. After that, updating
# is just: sudo termcade update
#
# Usage, from a private repository (the token needs read access to it):
#
#   read -rs GITHUB_TOKEN && export GITHUB_TOKEN
#   curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
#        -H "Accept: application/vnd.github.raw" \
#        https://api.github.com/repos/qateralong/termcade/contents/deploy/install.sh \
#     | sudo -E bash
#
# Optional environment: TERMCADE_REPO (owner/name), TERMCADE_ADDR (:2222).
set -euo pipefail

REPO="${TERMCADE_REPO:-qateralong/termcade}"
ADDR="${TERMCADE_ADDR:-:2222}"
API="https://api.github.com/repos/$REPO"

say()  { printf '  \033[1;35m◆\033[0m %s\n' "$*"; }
die()  { printf '\n  \033[1;31m✗\033[0m %s\n\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Run this as root (pipe it into: sudo -E bash)"
command -v curl >/dev/null || die "curl is required"
command -v sha256sum >/dev/null || die "sha256sum is required"
command -v systemctl >/dev/null || die "systemd is required"

case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Unsupported CPU architecture: $(uname -m)" ;;
esac
ASSET="termcade-linux-$ARCH"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Keep the token out of the process list by passing headers through a file.
HEADERS="$WORK/headers"
: > "$HEADERS"
chmod 600 "$HEADERS"
echo "X-GitHub-Api-Version: 2022-11-28" >> "$HEADERS"
if [ -n "${GITHUB_TOKEN:-}" ]; then
  echo "Authorization: Bearer $GITHUB_TOKEN" >> "$HEADERS"
fi
api() { curl -fsSL -H @"$HEADERS" "$@"; }

echo
say "Looking up the latest release of $REPO"
api -H "Accept: application/vnd.github+json" "$API/releases/latest" > "$WORK/release.json" \
  || die "Couldn't fetch the latest release. Is GITHUB_TOKEN set and allowed to read $REPO?"

# Print "<asset id> <asset name>" for every asset, without needing jq.
asset_ids() {
  tr -d '\n' < "$WORK/release.json" \
    | grep -o '"url": *"[^"]*/releases/assets/[0-9]*"[^}]*"name": *"[^"]*"' \
    | sed -E 's/.*assets\/([0-9]+)".*"name": *"([^"]*)"/\1 \2/'
}
asset_id() { asset_ids | awk -v n="$1" '$2 == n { print $1 }'; }

BIN_ID="$(asset_id "$ASSET")"
SUMS_ID="$(asset_id checksums.txt)"
[ -n "$BIN_ID" ]  || die "The latest release has no $ASSET"
[ -n "$SUMS_ID" ] || die "The latest release has no checksums.txt"

say "Downloading $ASSET"
api -L -H "Accept: application/octet-stream" "$API/releases/assets/$BIN_ID"  -o "$WORK/$ASSET"
api -L -H "Accept: application/octet-stream" "$API/releases/assets/$SUMS_ID" -o "$WORK/checksums.txt"

(cd "$WORK" && grep " $ASSET\$" checksums.txt | sha256sum -c --quiet -) \
  || die "Checksum mismatch; not installing"
chmod 755 "$WORK/$ASSET"

GITHUB_TOKEN="${GITHUB_TOKEN:-}" "$WORK/$ASSET" install -addr "$ADDR"
