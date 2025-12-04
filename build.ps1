# Build script for LoL Kind Bot with CGO enabled
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH

Write-Host "Building LoL Kind Bot with Fyne UI..." -ForegroundColor Cyan
Write-Host "CGO_ENABLED: $env:CGO_ENABLED" -ForegroundColor Gray
Write-Host "CC: $env:CC" -ForegroundColor Gray

go build -v -o lol-kind-bot.exe .

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nBuild successful! Executable: lol-kind-bot.exe" -ForegroundColor Green
    Write-Host "`nTo run:" -ForegroundColor Yellow
    Write-Host "  .\lol-kind-bot.exe              # Run normally (background)" -ForegroundColor White
    Write-Host "  .\lol-kind-bot.exe -debug       # Run with debug logging (console visible)" -ForegroundColor White
    Write-Host "  .\lol-kind-bot.exe -d           # Same as -debug" -ForegroundColor White
} else {
    Write-Host "`nBuild failed! Exit code: $LASTEXITCODE" -ForegroundColor Red
    exit $LASTEXITCODE
}

