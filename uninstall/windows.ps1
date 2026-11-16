$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0
$HostName = 'ru.dzhuraev.kotik.host'
$InstallDir = Join-Path $env:LOCALAPPDATA 'NyashnyKotik\Proxy'

Get-CimInstance Win32_Process -Filter "Name='kotik-host.exe'" -ErrorAction SilentlyContinue | ForEach-Object {
    if ($_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [StringComparison]::OrdinalIgnoreCase)) {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

$RegPaths = @(
    "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$HostName",
    "HKCU:\Software\Chromium\NativeMessagingHosts\$HostName",
    "HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\$HostName",
    "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\$HostName"
)
foreach ($RegPath in $RegPaths) {
    if (Test-Path $RegPath) {
        $Value = (Get-Item -LiteralPath $RegPath).GetValue('')
        if ($Value -and $Value.StartsWith($InstallDir, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -Recurse -Force -LiteralPath $RegPath
        }
    }
}
if (Test-Path $InstallDir) { Remove-Item -Recurse -Force -LiteralPath $InstallDir }
Write-Output 'Nyashny Kotik Helper removed for the current user.'
