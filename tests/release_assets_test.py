#!/usr/bin/env python3
import hashlib
from pathlib import Path

root = Path(__file__).resolve().parents[1] / "release"
required = [
    "NyashnyKotik-Extension-1.1.0-fix8.zip",
    "kotik-host-macos-arm64.gz",
    "kotik-host-windows-x64.exe.gz",
    "sing-box-macos-arm64.gz",
    "sing-box-windows-x64.exe.gz",
    "libcronet-windows-x64.dll.gz",
    "xray-macos-arm64.gz",
    "xray-windows-x64.exe.gz",
    "mihomo-macos-arm64.gz",
    "mihomo-windows-x64.exe.gz",
    "geoip.dat.gz",
    "geosite.dat.gz",
]
for name in required:
    path = root / name
    sidecar = root / f"{name}.sha256"
    assert path.is_file(), name
    expected = sidecar.read_text(encoding="ascii").split()[0]
    assert len(expected) == 64 and all(c in "0123456789abcdef" for c in expected)
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    assert actual == expected, name
print(f"Verified {len(required)} release assets.")
