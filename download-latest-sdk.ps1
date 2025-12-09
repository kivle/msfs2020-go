$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$metadataUrl = "https://sdk.flightsimulator.com/files/sdk.json"
$baseUrl = "https://sdk.flightsimulator.com/files/"
$destDir = Join-Path $PSScriptRoot "_vendor"
New-Item -ItemType Directory -Force -Path $destDir | Out-Null

$sdk = Invoke-RestMethod -Uri $metadataUrl
if (-not $sdk.game_versions) {
    throw "No game_versions found in sdk.json"
}

# Pick the latest SDK entry by game version
$latest = $sdk.game_versions | Sort-Object { [version]$_.version } -Descending | Select-Object -First 1
$core = $latest.downloads_menu.'SDK Installer (Core)'
if (-not $core -or -not $core.value) {
    throw "Could not locate core installer URL in sdk.json"
}

$relativePath = $core.value
$downloadUri = if ($relativePath -match '^https?://') { $relativePath } else { "$baseUrl$relativePath" }
$targetFile = Split-Path -Path $downloadUri -Leaf
$targetPath = Join-Path $destDir $targetFile

Write-Host "Downloading SDK $($latest.version) -> $targetPath"
Invoke-WebRequest -Uri $downloadUri -OutFile $targetPath

if (-not (Test-Path $targetPath)) {
    throw "Download failed: $targetPath was not created"
}

Write-Host "Done. Saved to $targetPath"
