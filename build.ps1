$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

powershell -ExecutionPolicy Bypass -File .\build-engine.ps1
if ($LASTEXITCODE -ne 0) { throw "build-engine.ps1 failed: $LASTEXITCODE" }

pnpm install --frozen-lockfile
if ($LASTEXITCODE -ne 0) { throw "pnpm install failed: $LASTEXITCODE" }

pnpm tauri build
if ($LASTEXITCODE -ne 0) { throw "pnpm tauri build failed: $LASTEXITCODE" }

$conf = Get-Content .\src-tauri\tauri.conf.json -Raw | ConvertFrom-Json
Write-Host "installer: src-tauri\target\release\bundle\nsis\$($conf.productName)_$($conf.version)_x64-setup.exe"
