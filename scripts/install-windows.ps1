# LocalWhisper installer for Windows 10/11.
# Builds the receiver, builds whisper.cpp with Vulkan, downloads the model,
# generates a pairing token, and registers an optional logon autostart task.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\install-windows.ps1 [-NoAutostart]
param(
    [switch]$NoAutostart
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$dataDir = Join-Path $env:LOCALAPPDATA 'localwhisper'
$modelName = 'large-v3-turbo-q5_0'
$whisperDir = Join-Path $dataDir 'whisper.cpp'
$modelPath = Join-Path $whisperDir "models\ggml-$modelName.bin"
$receiverExe = Join-Path $repoRoot 'receiver\localwhisper.exe'
$overlayExe = Join-Path $repoRoot 'receiver\localwhisper-overlay.exe'

function Need-Command([string]$name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        Write-Error "Missing required command: $name"
    }
}

foreach ($command in @('go', 'git', 'cmake')) {
    Need-Command $command
}

New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

if (-not (Test-Path $receiverExe)) {
    Push-Location (Join-Path $repoRoot 'receiver')
    go build -o localwhisper.exe ./cmd/localwhisper
    Pop-Location
}

if (-not (Test-Path $overlayExe)) {
    Push-Location (Join-Path $repoRoot 'receiver')
    go build -o localwhisper-overlay.exe ./cmd/localwhisper-overlay
    Pop-Location
}

if (-not (Test-Path (Join-Path $whisperDir '.git'))) {
    git clone --depth 1 https://github.com/ggml-org/whisper.cpp.git $whisperDir
}

$whisperCli = Join-Path $whisperDir 'build\bin\Release\whisper-cli.exe'
if (-not (Test-Path $whisperCli)) {
    cmake -S $whisperDir -B (Join-Path $whisperDir 'build') -DGGML_VULKAN=ON
    cmake --build (Join-Path $whisperDir 'build') --config Release
}

if (-not (Test-Path $modelPath)) {
    Write-Host "Downloading ggml-$modelName (about 547 MiB)..."
    Invoke-WebRequest -Uri "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-$modelName.bin" -OutFile $modelPath
}

$envFile = Join-Path $dataDir 'receiver.env'
if (-not (Test-Path $envFile)) {
    $tokenBytes = New-Object byte[] 32
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($tokenBytes)
    $token = ($tokenBytes | ForEach-Object { $_.ToString('x2') }) -join ''
    @"
WHISPER_BIN=$whisperCli
WHISPER_MODEL=$modelPath
WHISPER_THREADS=8
WS_ADDR=:8765
CONTROL_ADDR=127.0.0.1:8766
LOCALWHISPER_INDICATOR=$dataDir\localwhisper-overlay.exe
LOCALWHISPER_TOKEN=$token
"@ | Set-Content -NoNewline -Encoding ascii $envFile
}

Copy-Item $receiverExe (Join-Path $dataDir 'localwhisper.exe') -Force
Copy-Item $overlayExe (Join-Path $dataDir 'localwhisper-overlay.exe') -Force
Copy-Item (Join-Path $repoRoot 'scripts\start-windows.ps1') (Join-Path $dataDir 'start-localwhisper.ps1') -Force

if (-not $NoAutostart) {
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' `
        -Argument "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$dataDir\start-localwhisper.ps1`""
    $trigger = New-ScheduledTaskTrigger -AtLogOn
    Register-ScheduledTask -TaskName 'LocalWhisperReceiver' -Action $action -Trigger $trigger -Force | Out-Null
    Start-ScheduledTask -TaskName 'LocalWhisperReceiver'
}

$ip = (Get-NetIPAddress -AddressFamily IPv4 |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.PrefixOrigin -ne 'WellKnown' } |
    Select-Object -First 1).IPAddress
if (-not $ip) { $ip = '<your-laptop-ip>' }
$token = (Select-String -Path $envFile -Pattern '^LOCALWHISPER_TOKEN=(.+)$').Matches[0].Groups[1].Value

Write-Host ''
Write-Host 'LocalWhisper desktop is ready. Laptop IP:' $ip
Write-Host 'Pairing token:' $token
Write-Host ''
Write-Host 'Install the Android APK, enter both values, then bind Ctrl+Space to:'
Write-Host "  powershell -ExecutionPolicy Bypass -File `"$repoRoot\scripts\toggle-windows.ps1`""
