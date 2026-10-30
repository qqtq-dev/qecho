$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$ver = (Get-Content (Join-Path $root "src-tauri\tauri.conf.json") -Raw | ConvertFrom-Json).version
$binDir = Join-Path $root "src-tauri\binaries"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$env:CGO_ENABLED = "0"
$env:GOPROXY = "https://goproxy.cn,direct"
Push-Location (Join-Path $root "engine")
try {
  go build -trimpath -ldflags "-s -w -X main.version=$ver" `
    -o (Join-Path $binDir "engine-x86_64-pc-windows-msvc.exe") ./cmd/engine
  if ($LASTEXITCODE -ne 0) { throw "go build failed: $LASTEXITCODE" }
} finally {
  Pop-Location
}
Write-Host "engine built: $binDir\engine-x86_64-pc-windows-msvc.exe (version $ver)"
