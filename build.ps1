# Builds gta-mod-manager.exe with CGO enabled (Fyne needs a C compiler).
# Adjust $mingw if your MinGW-w64 lives elsewhere.
param(
    [switch]$Run,
    [switch]$Test
)

$ErrorActionPreference = "Stop"

$mingwCandidates = @(
    "C:\msys64\mingw64\bin",
    "C:\msys64\ucrt64\bin",
    "C:\mingw64\bin",
    "C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin"
)
$mingw = $mingwCandidates | Where-Object { Test-Path (Join-Path $_ "gcc.exe") } | Select-Object -First 1
if (-not $mingw) {
    throw "No MinGW-w64 gcc.exe found. Install it (e.g. 'choco install mingw') or edit build.ps1."
}

$env:Path = "$mingw;$env:Path"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"

Push-Location $PSScriptRoot
try {
    if ($Test) {
        go test ./...
        return
    }
    go build -o gta-mod-manager.exe .
    Write-Host "Built gta-mod-manager.exe" -ForegroundColor Green
    if ($Run) { & "$PSScriptRoot\gta-mod-manager.exe" }
}
finally {
    Pop-Location
}
