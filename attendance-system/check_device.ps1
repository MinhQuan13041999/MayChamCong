$ErrorActionPreference = "Stop"
$baseUrl = "http://localhost:8085/api/v1"

# Login
$loginBody = [System.Text.Encoding]::UTF8.GetBytes('{"username":"admin","password":"admin123"}')
$loginResp = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginResp.token
Write-Host "Token OK: $($token.Substring(0,20))..."

$headers = @{ Authorization = "Bearer $token" }

# Get devices
$devices = Invoke-RestMethod -Uri "$baseUrl/devices" -Method Get -Headers $headers
Write-Host "`n=== DANH SACH THIET BI ==="
foreach ($d in $devices) {
    Write-Host "ID   : $($d.id)"
    Write-Host "Name : $($d.name)"
    Write-Host "IP   : $($d.ip_address)"
    Write-Host "Port : $($d.port)"
    Write-Host "Type : $($d.device_type)"
    Write-Host "ADMS : $($d.adms_enabled)"
    Write-Host "SN   : $($d.serial_number)"
    Write-Host "SNADMS: $($d.serial_number_adms)"
    Write-Host "Status: $($d.status)"
    Write-Host "Heartbeat: $($d.last_heartbeat_at)"
    Write-Host "---"
}
