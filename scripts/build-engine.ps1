$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$engineDir = Join-Path $root 'engine'
$outDir = Join-Path $root 'shell\src-tauri\binaries'
$out = Join-Path $outDir 'agentdock-engine-x86_64-pc-windows-msvc.exe'

if (-not $env:GOPROXY) {
    $env:GOPROXY = 'https://goproxy.cn,direct'
}

New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$env:CGO_ENABLED = '0'
Push-Location $engineDir
try {
    go build -tags nogui -trimpath -o $out .
} finally {
    Pop-Location
}
Write-Host "Wrote $out"

# Tauri looks up externalBin using the rustc host triple. Keep the contract
# msvc name and also emit a gnu-named copy for MinGW host builds.
$gnu = Join-Path $outDir 'agentdock-engine-x86_64-pc-windows-gnu.exe'
Copy-Item -Force $out $gnu
