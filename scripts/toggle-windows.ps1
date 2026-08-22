# Toggle LocalWhisper recording. Bind this to Ctrl+Space.
# Exit code 1 means no phone is connected.
$ErrorActionPreference = 'Stop'
try {
    $status = Invoke-RestMethod -Uri 'http://127.0.0.1:8766/status' -TimeoutSec 1
    if (-not $status.phone_connected) {
        Write-Error 'LocalWhisper: no phone connected'
        exit 1
    }
    Invoke-RestMethod -Uri 'http://127.0.0.1:8766/toggle' -Method Post -TimeoutSec 5 | Out-Null
} catch [Microsoft.PowerShell.Commands.HttpResponseException] {
    # 409: transcription already in progress
}
