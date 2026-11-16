#!/bin/bash
set -euo pipefail

TAG="v1.1.0-fix8"
REPO="sismeliostroy/Nyashny-Kotik-Extension"
HOST_NAME="ru.dzhuraev.kotik.host"
EXPECTED_ID="fgogfmjjadljkpiejmlmhoaafepfnooo"
EXT_ID="${1:-$EXPECTED_ID}"
CORES="${2:-sing-box,xray,mihomo}"

fail() { printf 'Nyashny Kotik: %s\n' "$1" >&2; exit 1; }
[ "$(uname -s)" = "Darwin" ] || fail "macOS is required."
[ "$(uname -m)" = "arm64" ] || fail "This release supports Apple Silicon (arm64) only."
[ "$EXT_ID" = "$EXPECTED_ID" ] || fail "Unexpected extension ID."
command -v curl >/dev/null || fail "curl is required."
command -v shasum >/dev/null || fail "shasum is required."
command -v gzip >/dev/null || fail "gzip is required."

umask 077
INSTALL_DIR="$HOME/Library/Application Support/NyashnyKotik/Proxy"
TXN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/nyashny-kotik-install.XXXXXX")"
STAGE="$TXN_DIR/stage"
BACKUP="$TXN_DIR/backup"
MANIFESTS="$TXN_DIR/manifests"
COMMITTED=0
mkdir -p "$STAGE" "$BACKUP" "$MANIFESTS"

cleanup() {
  status=$?
  if [ "$status" -ne 0 ] && [ "$COMMITTED" -eq 1 ]; then
    rm -rf "$INSTALL_DIR"
    if [ -d "$BACKUP/install" ]; then mv "$BACKUP/install" "$INSTALL_DIR"; fi
    while IFS= read -r record; do
      target=${record%%|*}; saved=${record#*|}
      rm -f "$target"
      if [ "$saved" != "-" ] && [ -f "$saved" ]; then mkdir -p "$(dirname "$target")"; cp "$saved" "$target"; fi
    done < "$BACKUP/manifest-records" 2>/dev/null || true
  fi
  rm -rf "$TXN_DIR"
  exit "$status"
}
trap cleanup EXIT HUP INT TERM

BASE="https://github.com/$REPO/releases/download/$TAG"
fetch_checked() {
  asset=$1
  output=$2
  curl -fL --proto '=https' --tlsv1.2 --retry 2 -o "$TXN_DIR/$asset" "$BASE/$asset"
  curl -fL --proto '=https' --tlsv1.2 --retry 2 -o "$TXN_DIR/$asset.sha256" "$BASE/$asset.sha256"
  expected=$(awk 'NR==1 {print $1}' "$TXN_DIR/$asset.sha256")
  case "$expected" in (*[!0-9a-fA-F]*|'') fail "Invalid checksum for $asset.";; esac
  [ "${#expected}" -eq 64 ] || fail "Invalid checksum length for $asset."
  actual=$(shasum -a 256 "$TXN_DIR/$asset" | awk '{print $1}')
  [ "$actual" = "$expected" ] || fail "Checksum mismatch for $asset."
  gzip -dc "$TXN_DIR/$asset" > "$output"
}

fetch_checked "kotik-host-macos-arm64.gz" "$STAGE/kotik-host"
chmod 700 "$STAGE/kotik-host"

case ",$CORES," in (*,sing-box,*) fetch_checked "sing-box-macos-arm64.gz" "$STAGE/sing-box"; chmod 700 "$STAGE/sing-box";; esac
case ",$CORES," in (*,xray,*) fetch_checked "xray-macos-arm64.gz" "$STAGE/xray"; chmod 700 "$STAGE/xray";; esac
case ",$CORES," in (*,mihomo,*) fetch_checked "mihomo-macos-arm64.gz" "$STAGE/mihomo"; chmod 700 "$STAGE/mihomo";; esac
fetch_checked "geoip.dat.gz" "$STAGE/geoip.dat"
fetch_checked "geosite.dat.gz" "$STAGE/geosite.dat"
chmod 600 "$STAGE/geoip.dat" "$STAGE/geosite.dat"

MANIFEST_CONTENT=$(printf '{"name":"%s","description":"Nyashny Kotik 3000 native helper","path":"%s/kotik-host","type":"stdio","allowed_origins":["chrome-extension://%s/"]}' "$HOST_NAME" "$INSTALL_DIR" "$EXT_ID")
for root in \
  "$HOME/Library/Application Support/Google/Chrome" \
  "$HOME/Library/Application Support/Chromium" \
  "$HOME/Library/Application Support/BraveSoftware/Brave-Browser" \
  "$HOME/Library/Application Support/Microsoft Edge"
do
  dir="$root/NativeMessagingHosts"
  mkdir -p "$dir"
  target="$dir/$HOST_NAME.json"
  safe=$(printf '%s' "$target" | shasum -a 256 | awk '{print $1}')
  saved="-"
  if [ -f "$target" ]; then saved="$BACKUP/$safe"; cp "$target" "$saved"; fi
  printf '%s|%s\n' "$target" "$saved" >> "$BACKUP/manifest-records"
  printf '%s\n' "$MANIFEST_CONTENT" > "$MANIFESTS/$safe"
  chmod 600 "$MANIFESTS/$safe"
done

mkdir -p "$(dirname "$INSTALL_DIR")"
if [ -d "$INSTALL_DIR" ]; then mv "$INSTALL_DIR" "$BACKUP/install"; fi
mv "$STAGE" "$INSTALL_DIR"
COMMITTED=1

while IFS='|' read -r target saved; do
  safe=$(printf '%s' "$target" | shasum -a 256 | awk '{print $1}')
  tmp="$target.tmp.$$"
  cp "$MANIFESTS/$safe" "$tmp"
  chmod 600 "$tmp"
  mv "$tmp" "$target"
done < "$BACKUP/manifest-records"

COMMITTED=0
printf 'Nyashny Kotik Helper installed for the current user. Reload the extension.\n'
