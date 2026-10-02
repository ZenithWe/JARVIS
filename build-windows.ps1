Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location (Join-Path $root "codigo-fonte")
try {
    go test ./...

    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"

    go build -buildvcs=false -trimpath -ldflags="-s -w -H=windowsgui" -o (Join-Path $root "JARVIS.exe") .
}
finally {
    Pop-Location
}

Get-FileHash (Join-Path $root "JARVIS.exe") -Algorithm SHA256
