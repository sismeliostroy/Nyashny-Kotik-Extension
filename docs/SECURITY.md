# Security model

- Native Messaging is restricted to the fixed extension ID.
- Core listeners are injected as `127.0.0.1` listeners.
- Downloaded release assets are checked with SHA-256 before decompression.
- Installation is staged and committed only after all selected assets pass.
- No administrator privileges, system service, driver, TUN or system proxy.
- The extension has no analytics, advertising SDK or external crash reporter.
- Subscription URLs, proxy URIs, UUIDs, keys and tokens must not be logged.
- Diagnostic output is redacted before it is exported.

Report security issues privately to the repository owner. Do not include live
profiles, subscription URLs or credentials in a public issue.
