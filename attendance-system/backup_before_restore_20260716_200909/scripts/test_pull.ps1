param(
    [string]$DeviceId,
    [string]$Token = ""
)
if (-not $DeviceId) { Write-Error "Usage: .\test_pull.ps1 -DeviceId <id> [-Token <jwt>]"; exit 1 }
$headers = @{}
if ($Token) { $headers.Add('Authorization', "Bearer $Token") }
Write-Host "POST /api/v1/devices/$DeviceId/pull-employees"
try {
    $r = Invoke-RestMethod -Method Post -Uri "http://localhost:8085/api/v1/devices/$DeviceId/pull-employees" -Headers $headers -Body '{}' -ContentType 'application/json' -ErrorAction Stop
    Write-Host (ConvertTo-Json $r -Depth 4)
} catch {
    Write-Host "ERROR: $_"
}
Write-Host "GET /api/v1/devices/$DeviceId/debug-queue"
try {
    $r2 = Invoke-RestMethod -Method Get -Uri "http://localhost:8085/api/v1/devices/$DeviceId/debug-queue" -Headers $headers -ErrorAction Stop
    Write-Host (ConvertTo-Json $r2 -Depth 6)
} catch {
    Write-Host "ERROR: $_"
}
