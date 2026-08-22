# Starts the LocalWhisper receiver with environment from receiver.env.
# Used by the LocalWhisperReceiver scheduled task created by install-windows.ps1.
$dataDir = Join-Path $env:LOCALAPPDATA 'localwhisper'
$envFile = Join-Path $dataDir 'receiver.env'
Get-Content $envFile | ForEach-Object {
    if ($_ -match '^\s*([A-Z_]+)=(.*?)\s*$') {
        Set-Item -Path "env:$($Matches[1])" -Value $Matches[2]
    }
}
Set-Location $dataDir

# Overlay serve owns the status pill window; receiver posts states to it.
Start-Process -FilePath (Join-Path $dataDir 'localwhisper-overlay.exe') `
    -ArgumentList 'serve' -WindowStyle Hidden
Start-Sleep -Milliseconds 500
& (Join-Path $dataDir 'localwhisper.exe')
