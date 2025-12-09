$ErrorActionPreference = "Stop"

# Run go-bindata if available (matches Bash behavior)
if (Get-Command go-bindata -ErrorAction SilentlyContinue) {
    go generate github.com/lian/msfs2020-go/simconnect
}

$buildTime = (Get-Date -AsUTC).ToString("yyyy-MM-dd_HH:mm:ss")

# git describe can fail on clean clones; fall back to "unknown" without stopping
$buildVersion = (git describe --tags 2>$null).Trim()
if (-not $buildVersion) { $buildVersion = "unknown" }

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

go build -ldflags "-s -w -X main.buildVersion=$buildVersion -X main.buildTime=$buildTime" `
    -v github.com/lian/msfs2020-go/vfrmap
