param(
    [string]$ListenHttp = "0.0.0.0:9000",
    [string]$ListenHttps = "0.0.0.0:9443",
    [switch]$Verbose,
    [switch]$DisableTeleport
)

$ErrorActionPreference = "Stop"

$goArgs = @('go', 'run', './simconnect-ws', '-listen-http', $ListenHttp, '-listen-https', $ListenHttps)
if ($Verbose) { $goArgs += '-verbose' }
if ($DisableTeleport) { $goArgs += '-disable-teleport' }

Write-Host "Running: $($goArgs -join ' ')"
$cmd = $goArgs[0]
$cmdArgs = @()
if ($goArgs.Length -gt 1) {
    $cmdArgs = $goArgs[1..($goArgs.Length - 1)]
}
& $cmd @cmdArgs
