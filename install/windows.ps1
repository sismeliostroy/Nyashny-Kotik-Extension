$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Tag = 'v1.1.0-fix8'
$Repo = 'sismeliostroy/Nyashny-Kotik-Extension'
$HostName = 'ru.dzhuraev.kotik.host'
$ExpectedId = 'fgogfmjjadljkpiejmlmhoaafepfnooo'
$ExtensionId = if ($env:KOTIK_EXT_ID) { $env:KOTIK_EXT_ID } else { $ExpectedId }
$CoreList = if ($env:KOTIK_CORES) { $env:KOTIK_CORES } else { 'sing-box,xray,mihomo' }
if ($ExtensionId -ne $ExpectedId) { throw 'Unexpected extension ID.' }
if (-not [Environment]::Is64BitOperatingSystem) { throw 'Windows x64 is required.' }

$InstallDir = Join-Path $env:LOCALAPPDATA 'NyashnyKotik\Proxy'
$Txn = Join-Path ([IO.Path]::GetTempPath()) ('nyashny-kotik-install-' + [Guid]::NewGuid().ToString('N'))
$Stage = Join-Path $Txn 'stage'
$Backup = Join-Path $Txn 'backup'
$Base = "https://github.com/$Repo/releases/download/$Tag"
$Committed = $false
$RegistryState = @()
New-Item -ItemType Directory -Force -Path $Stage, $Backup | Out-Null

function Get-CheckedAsset([string]$Asset, [string]$Output) {
    $Archive = Join-Path $Txn $Asset
    $Sidecar = $Archive + '.sha256'
    Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Asset" -OutFile $Archive
    Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Asset.sha256" -OutFile $Sidecar
    $Line = (Get-Content -LiteralPath $Sidecar -TotalCount 1).Trim()
    if ($Line -notmatch '^([0-9a-fA-F]{64})\s+') { throw "Invalid checksum for $Asset." }
    $Expected = $Matches[1].ToLowerInvariant()
    $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Archive).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected) { throw "Checksum mismatch for $Asset." }
    $InputStream = [IO.File]::OpenRead($Archive)
    try {
        $Gzip = New-Object IO.Compression.GzipStream($InputStream, [IO.Compression.CompressionMode]::Decompress)
        try {
            $OutputStream = [IO.File]::Create($Output)
            try { $Gzip.CopyTo($OutputStream) } finally { $OutputStream.Dispose() }
        } finally { $Gzip.Dispose() }
    } finally { $InputStream.Dispose() }
}

try {
    Get-CheckedAsset 'kotik-host-windows-x64.exe.gz' (Join-Path $Stage 'kotik-host.exe')
    $Cores = @($CoreList.Split(',') | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($Cores -contains 'sing-box') {
        Get-CheckedAsset 'sing-box-windows-x64.exe.gz' (Join-Path $Stage 'sing-box.exe')
        Get-CheckedAsset 'libcronet-windows-x64.dll.gz' (Join-Path $Stage 'libcronet.dll')
    }
    if ($Cores -contains 'xray') { Get-CheckedAsset 'xray-windows-x64.exe.gz' (Join-Path $Stage 'xray.exe') }
    if ($Cores -contains 'mihomo') { Get-CheckedAsset 'mihomo-windows-x64.exe.gz' (Join-Path $Stage 'mihomo.exe') }
    Get-CheckedAsset 'geoip.dat.gz' (Join-Path $Stage 'geoip.dat')
    Get-CheckedAsset 'geosite.dat.gz' (Join-Path $Stage 'geosite.dat')

    $ManifestPath = Join-Path $InstallDir "$HostName.json"
    $Manifest = [ordered]@{
        name = $HostName
        description = 'Nyashny Kotik 3000 native helper'
        path = (Join-Path $InstallDir 'kotik-host.exe')
        type = 'stdio'
        allowed_origins = @("chrome-extension://$ExtensionId/")
    }
    $Manifest | ConvertTo-Json -Depth 4 -Compress | Set-Content -Encoding ASCII -LiteralPath (Join-Path $Stage "$HostName.json")

    $RegPaths = @(
        "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$HostName",
        "HKCU:\Software\Chromium\NativeMessagingHosts\$HostName",
        "HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\$HostName",
        "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\$HostName"
    )
    foreach ($RegPath in $RegPaths) {
        $Exists = Test-Path $RegPath
        $Old = $null
        if ($Exists) { $Old = (Get-Item -LiteralPath $RegPath).GetValue('') }
        $RegistryState += [pscustomobject]@{ Path = $RegPath; Exists = $Exists; Value = $Old }
    }

    New-Item -ItemType Directory -Force -Path (Split-Path $InstallDir -Parent) | Out-Null
    if (Test-Path $InstallDir) { Move-Item -LiteralPath $InstallDir -Destination (Join-Path $Backup 'install') }
    Move-Item -LiteralPath $Stage -Destination $InstallDir
    $Committed = $true

    foreach ($RegPath in $RegPaths) {
        New-Item -Force -Path $RegPath | Out-Null
        Set-Item -LiteralPath $RegPath -Value $ManifestPath
    }
    $Committed = $false
    Write-Output 'Nyashny Kotik Helper installed for the current user. Reload the extension.'
}
catch {
    if ($Committed) {
        if (Test-Path $InstallDir) { Remove-Item -Recurse -Force -LiteralPath $InstallDir }
        $OldInstall = Join-Path $Backup 'install'
        if (Test-Path $OldInstall) { Move-Item -LiteralPath $OldInstall -Destination $InstallDir }
        foreach ($State in $RegistryState) {
            if ($State.Exists) {
                New-Item -Force -Path $State.Path | Out-Null
                Set-Item -LiteralPath $State.Path -Value $State.Value
            } elseif (Test-Path $State.Path) {
                Remove-Item -Recurse -Force -LiteralPath $State.Path
            }
        }
    }
    throw
}
finally {
    if (Test-Path $Txn) { Remove-Item -Recurse -Force -LiteralPath $Txn }
}
