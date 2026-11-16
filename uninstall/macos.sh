#!/bin/bash
set -euo pipefail
HOST_NAME="ru.dzhuraev.kotik.host"
INSTALL_DIR="$HOME/Library/Application Support/NyashnyKotik/Proxy"

for pid in $(pgrep -f "$INSTALL_DIR/kotik-host" 2>/dev/null || true); do
  command_path=$(ps -p "$pid" -o command= 2>/dev/null || true)
  case "$command_path" in ("$INSTALL_DIR/kotik-host"*) kill "$pid" 2>/dev/null || true;; esac
done

for root in \
  "$HOME/Library/Application Support/Google/Chrome" \
  "$HOME/Library/Application Support/Chromium" \
  "$HOME/Library/Application Support/BraveSoftware/Brave-Browser" \
  "$HOME/Library/Application Support/Microsoft Edge"
do
  target="$root/NativeMessagingHosts/$HOST_NAME.json"
  if [ -f "$target" ] && grep -Fq "$INSTALL_DIR/kotik-host" "$target"; then rm -f "$target"; fi
done
rm -rf "$INSTALL_DIR"
printf 'Nyashny Kotik Helper removed for the current user.\n'
