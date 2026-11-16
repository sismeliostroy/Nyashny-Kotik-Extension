#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 -m json.tool "$ROOT/extension/manifest.json" >/dev/null
find "$ROOT/extension/_locales" -name '*.json' -type f -exec python3 -m json.tool '{}' \; >/dev/null
bash -n "$ROOT/install/macos.sh"
bash -n "$ROOT/uninstall/macos.sh"

test "$(python3 - "$ROOT/extension/manifest.json" <<'PY'
import base64, hashlib, json, sys
k=json.load(open(sys.argv[1],encoding='utf-8'))['key']
d=hashlib.sha256(base64.b64decode(k)).digest()[:16]
print(''.join(chr(97+(b>>4))+chr(97+(b&15)) for b in d))
PY
)" = "fgogfmjjadljkpiejmlmhoaafepfnooo"

if grep -RinE '(gh[pousr]_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9]{20,}|BEGIN (RSA|OPENSSH|EC) PRIVATE KEY)' \
  "$ROOT" --exclude-dir=.git --exclude='static-check.sh'; then
  echo 'Potential secret detected.' >&2
  exit 1
fi

if grep -RinE 'noctis|c0nn3ct' "$ROOT/extension"; then
  echo 'Noctis user-facing branding detected in extension.' >&2
  exit 1
fi

grep -Fq 'v1.1.0-fix8' "$ROOT/install/macos.sh"
grep -Fq 'v1.1.0-fix8' "$ROOT/install/windows.ps1"
grep -Fq 'releases/tags/v1.1.0-fix8' "$ROOT/extension/assets/sidepanel-BzVe7YbO.js"
! grep -Rq 'xattr' "$ROOT/install" "$ROOT/uninstall"
! grep -RqE 'sudo|/Library/Launch|HKLM:' "$ROOT/install" "$ROOT/uninstall"

if command -v node >/dev/null 2>&1; then
  find "$ROOT/extension" -name '*.js' -type f -exec node --check '{}' \;
fi

echo 'Static checks passed.'
