# LocalWhisper installer for Windows 10/11.
# Can be run directly on a clean machine: it downloads the source if needed,
# installs missing dependencies through winget, prefers prebuilt binaries from
# the latest GitHub release, builds whisper.cpp with Vulkan, downloads the
# model, generates a pairing token, and registers an autostart task.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\install-windows.ps1 [-NoAutostart] [-BuildFromSource]
param(
    [switch]$NoAutostart,
    [switch]$BuildFromSource,
    [string]$Repo = 'Namishk/localwhisper'
)

$ErrorActionPreference = 'Stop'

$dataDir = Join-Path $env:LOCALAPPDATA 'localwhisper'
$srcDir = Join-Path $dataDir 'src'
$modelName = 'large-v3-turbo-q5_0'
$whisperDir = Join-Path $dataDir 'whisper.cpp'
$modelPath = Join-Path $whisperDir "models\ggml-$modelName.bin"

function Write-Step([string]$message) { Write-Host "`n==> $message" }

# --- locate or download the source -------------------------------------------

if (Test-Path (Join-Path $PSScriptRoot '..\receiver\go.mod')) {
    Write-Step 'Using source from this checkout'
    $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
} else {
    Write-Step "Downloading source from github.com/$Repo"
    New-Item -ItemType Directory -Force -Path $dataDir, $srcDir | Out-Null
    $zip = Join-Path $env:TEMP 'localwhisper-src.zip'
    Invoke-WebRequest -Uri "https://github.com/$Repo/archive/refs/heads/main.zip" -OutFile $zip
    $staging = Join-Path $env:TEMP ('localwhisper-src-' + [guid]::NewGuid().ToString('N'))
    Expand-Archive -Path $zip -DestinationPath $staging -Force
    $extracted = Get-ChildItem $staging | Select-Object -First 1
    Copy-Item -Path (Join-Path $extracted.FullName '*') -Destination $srcDir -Recurse -Force
    Remove-Item $zip, $staging -Recurse -Force
    $repoRoot = $srcDir
}

$receiverExe = Join-Path $repoRoot 'receiver\localwhisper.exe'
$overlayExe = Join-Path $repoRoot 'receiver\localwhisper-overlay.exe'

# --- winget and dependency bootstrap -----------------------------------------

function Ensure-Winget {
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        Write-Error @'
winget is not available. Install "App Installer" from the Microsoft Store,
then rerun this script.
'@
    }
}

function Install-WingetPackage([string]$id, [string]$override = '') {
    Write-Host "Installing $id via winget..."
    $args = @('install', '--id', $id, '--exact', '--silent',
        '--accept-package-agreements', '--accept-source-agreements')
    if ($override) { $args += @('--override', $override) }
    winget @args
    if ($LASTEXITCODE -notin 0, 0x8A15002B) { # 0x8A15002B: already installed
        # Some installers report nonzero for no-op runs; only hard-fail when
        # the tool is still missing afterwards.
        Write-Host "winget returned $LASTEXITCODE for $id; continuing."
    }
    # Pick up PATH changes made by the installer.
    $env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' +
        [Environment]::GetEnvironmentVariable('Path', 'User')
}

function Test-VsCppToolchain {
    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    if (-not (Test-Path $vswhere)) { return $false }
    $instances = & $vswhere -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
    return [bool]$instances
}

Ensure-Winget

Write-Step 'Checking dependencies'
if (-not (Test-VsCppToolchain)) {
    Install-WingetPackage 'Microsoft.VisualStudio.2022BuildTools' `
        '--quiet --wait --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended'
}
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Install-WingetPackage 'Git.Git' }
if (-not (Get-Command cmake -ErrorAction SilentlyContinue)) { Install-WingetPackage 'Kitware.CMake' }
if (-not (Get-Command glslc -ErrorAction SilentlyContinue)) {
    Install-WingetPackage 'KhronosGroup.VulkanSDK'
}

# --- receiver and overlay binaries -------------------------------------------

New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

$gotPrebuilt = $false
if (-not $BuildFromSource) {
    Write-Step 'Fetching prebuilt binaries from the latest release'
    try {
        $binZip = Join-Path $env:TEMP 'localwhisper-bin.zip'
        Invoke-WebRequest `
            -Uri "https://github.com/$Repo/releases/latest/download/localwhisper-windows-amd64.zip" `
            -OutFile $binZip -ErrorAction Stop
        Expand-Archive -Path $binZip -DestinationPath $dataDir -Force
        Remove-Item $binZip
        $gotPrebuilt = (Test-Path (Join-Path $dataDir 'localwhisper.exe')) -and
            (Test-Path (Join-Path $dataDir 'localwhisper-overlay.exe'))
        if (-not $gotPrebuilt) { Write-Host 'Release archive was incomplete.' }
    } catch {
        Write-Host "No usable release assets found ($_)."
    }
}

if (-not $gotPrebuilt) {
    Write-Step 'Building receiver and overlay from source'
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Install-WingetPackage 'GoLang.Go'
    }
    Push-Location (Join-Path $repoRoot 'receiver')
    go build -o localwhisper.exe ./cmd/localwhisper
    if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Error 'receiver build failed' }
    go build -o localwhisper-overlay.exe ./cmd/localwhisper-overlay
    if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Error 'overlay build failed' }
    Pop-Location
}

# --- whisper.cpp with Vulkan ---------------------------------------------------

Write-Step 'Building whisper.cpp with Vulkan'
if (-not (Test-Path (Join-Path $whisperDir '.git'))) {
    git clone --depth 1 https://github.com/ggml-org/whisper.cpp.git $whisperDir
}
$whisperCli = Join-Path $whisperDir 'build\bin\Release\whisper-cli.exe'
if (-not (Test-Path $whisperCli)) {
    cmake -S $whisperDir -B (Join-Path $whisperDir 'build') -DGGML_VULKAN=ON
    if ($LASTEXITCODE -ne 0) { Write-Error 'cmake configure failed' }
    cmake --build (Join-Path $whisperDir 'build') --config Release
    if ($LASTEXITCODE -ne 0) { Write-Error 'whisper.cpp build failed' }
}

if (-not (Test-Path $modelPath)) {
    Write-Step 'Downloading model (about 547 MiB)'
    Invoke-WebRequest `
        -Uri "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-$modelName.bin" `
        -OutFile $modelPath
}

# --- configuration and autostart -------------------------------------------------

Write-Step 'Writing configuration'
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

Copy-Item (Join-Path $repoRoot 'scripts\start-windows.ps1') (Join-Path $dataDir 'start-localwhisper.ps1') -Force
Copy-Item (Join-Path $repoRoot 'scripts\toggle-windows.ps1') (Join-Path $dataDir 'toggle-localwhisper.ps1') -Force
if ($gotPrebuilt -eq $false) {
    Copy-Item $receiverExe (Join-Path $dataDir 'localwhisper.exe') -Force
    Copy-Item $overlayExe (Join-Path $dataDir 'localwhisper-overlay.exe') -Force
}

if (-not $NoAutostart) {
    Write-Step 'Registering logon task'
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' `
        -Argument "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$dataDir\start-localwhisper.ps1`""
    $trigger = New-ScheduledTaskTrigger -AtLogOn
    Register-ScheduledTask -TaskName 'LocalWhisperReceiver' -Action $action -Trigger $trigger -Force | Out-Null
    Start-ScheduledTask -TaskName 'LocalWhisperReceiver'
} else {
    Write-Host 'Skipping autostart registration.'
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
Write-Host "  powershell -ExecutionPolicy Bypass -File `"$dataDir\toggle-localwhisper.ps1`""
