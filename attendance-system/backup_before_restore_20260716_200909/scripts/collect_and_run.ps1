$ErrorActionPreference = 'Stop'

try {
    $body = @{ username = 'admin'; password = 'admin123' } | ConvertTo-Json
    $resp = Invoke-RestMethod -Method Post -Uri 'http://localhost:8085/api/v1/auth/login' -Body $body -ContentType 'application/json'
} catch {
    Write-Host "LOGIN ERROR: $_"
    exit 1
}

$token = $resp.token
Write-Host "TOKEN: $token"
$hdr = @{ Authorization = "Bearer $token" }

try {
    $devs = Invoke-RestMethod -Method Get -Uri 'http://localhost:8085/api/v1/devices' -Headers $hdr
    Write-Host "DEVICES:"
    $devs | ConvertTo-Json -Depth 6 | Write-Host
} catch {
    Write-Host "LIST DEVICES ERROR: $_"
    exit 1
}

if ($devs -is [System.Array]) { $deviceId = $devs[0].id } else { $deviceId = $devs.id }
Write-Host "DEVICE_ID: $deviceId"

if ($deviceId) {
    Write-Host "Running scripts/test_push.ps1"
    & .\scripts\test_push.ps1 -DeviceId $deviceId -Token $token
    Write-Host "Running scripts/test_pull.ps1"
    & .\scripts\test_pull.ps1 -DeviceId $deviceId -Token $token
} else {
    Write-Host "No device found to test."
}
