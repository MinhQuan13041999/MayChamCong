$ErrorActionPreference = 'Stop'

try {
    $creds = @{ username = 'admin'; password = 'admin123' } | ConvertTo-Json
    $resp = Invoke-RestMethod -Method Post -Uri 'http://localhost:8085/api/v1/auth/login' -Body $creds -ContentType 'application/json'
} catch {
    Write-Output "LOGIN_ERROR: $_"
    exit 1
}

$token = $resp.token
Write-Output "TOKEN: $token"
$hdr = @{ 'Authorization' = "Bearer $token" }
$deviceId = 'dba701f3-fc74-4d79-8a43-5f989c29622d'
Write-Output "PUSH: $deviceId"

try {
    $push = Invoke-RestMethod -Method Post -Uri ("http://localhost:8085/api/v1/devices/" + $deviceId + "/sync-employees") -Headers $hdr -Body '{}' -ContentType 'application/json' -ErrorAction Stop
    Write-Output 'PUSH_RESPONSE:'
    $push | ConvertTo-Json -Depth 6 | Write-Output
} catch {
    Write-Output 'PUSH_ERROR:'
    Write-Output $_.Exception.Message
}

Write-Output 'DEBUG_QUEUE:'
try {
    $dq = Invoke-RestMethod -Method Get -Uri ("http://localhost:8085/api/v1/devices/" + $deviceId + "/debug-queue") -Headers $hdr -ErrorAction Stop
    $dq | ConvertTo-Json -Depth 6 | Write-Output
} catch {
    Write-Output 'DEBUG_QUEUE_ERROR:'
    Write-Output $_.Exception.Message
}

Write-Output '--- SERVER LOG (last 300 lines) ---'
try {
    Get-Content .\server_run.log -Tail 300 | Write-Output
} catch {
    Write-Output 'TAIL_ERROR:'
    Write-Output $_.Exception.Message
}
