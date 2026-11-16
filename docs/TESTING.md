# Testing

Run `scripts/static-check.sh` for manifest/JSON/JavaScript/shell checks and
secret scanning. Run `go test ./...` in `helper/` with Go 1.25 or newer.

Installer tests use an isolated HOME and fixture binaries. A real user profile
must never be used by automated tests. Windows PowerShell 5.1 E2E requires a
Windows 10/11 x64 host; the repository CI performs parser/static checks and a
per-user install cycle with fixture assets.
