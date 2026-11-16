# Architecture

The Manifest V3 extension owns user interaction, profile parsing, routing
selection and Chrome's per-extension proxy settings. It exchanges framed JSON
messages with the per-user native host `ru.dzhuraev.kotik.host`.

The Go helper validates the requested core, writes an ephemeral configuration,
selects a free loopback port, starts sing-box, Xray or Mihomo, waits for the
loopback listener and supervises the child process. Closing the Native
Messaging channel stops the child. No system service, TUN driver or system
proxy is installed.

Supported release platforms are macOS arm64 and Windows x64. All installed
files are per-user.
