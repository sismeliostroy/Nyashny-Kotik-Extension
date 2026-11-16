# Test report — v1.1.0-fix8

Date: 2026-11-16

## Passed locally

- Helper Go test suite: PASS.
- Helper cross-build: macOS arm64 and Windows x64: PASS.
- Extension manifest and locale JSON parsing: PASS.
- Fixed extension ID derived from `manifest.key`: `fgogfmjjadljkpiejmlmhoaafepfnooo`.
- Extension ZIP integrity (`unzip -t`): PASS.
- macOS installer syntax and per-user isolated first install: PASS.
- macOS repeat update with selected Xray-only core set: PASS.
- SHA-256 mismatch fail-closed test: PASS; installed tree remained byte-identical.
- macOS uninstaller in isolated HOME: PASS; installed files and manifests removed.
- Native Messaging `hello` and `ping`: PASS.
- Fixture Xray `start`, `stats`, `stop`: PASS.
- Xray listener bound to loopback TCP only; no UDP listener: PASS.
- Listener and child process absent after `stop`: PASS.
- Official upstream archive digests checked before repackaging: PASS.
- Release sidecars and aggregate `SHA256SUMS`: PASS.
- Secret-pattern scan and user-facing Noctis branding scan: PASS.

## CI / publication gates

- JavaScript `node --check`: delegated to GitHub CI because Node is not installed
  on the build Mac.
- Windows PowerShell 5.1 parser and ASCII checks: delegated to GitHub CI.
- Real Windows 10 x64 install/update/uninstall E2E: not available locally; owner
  testing remains required for this prerelease.
- Chrome UI copy-to-clipboard and execution of the command fetched from public
  GitHub URLs: requires the public repository and Release to exist, and must be
  repeated after publication.
- No real user proxy profile was used by automated tests.

The release remains a prerelease/manual test candidate, not a stable mass
distribution.
