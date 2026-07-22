# Script test device deletion via API
$body = @{
    username = "admin"
    password = "admin123"
} | ConvertTo-Json

$loginUrl = "http://localhost:8085/api/v1/auth/login"
Write-Host "Logging in..."
$response = Invoke-RestMethod -Uri $loginUrl -Method Post -Body $body -ContentType "application/json"
$token = $response.token
Write-Host "Logged in successfully. Token: $token"

# List devices
$devicesUrl = "http://localhost:8085/api/v1/devices"
$headers = @{
    Authorization = "Bearer $token"
}
$devices = Invoke-RestMethod -Uri $devicesUrl -Method Get -Headers $headers
Write-Host "Current devices in system:"
$targetDevice = $null
foreach ($dev in $devices) {
    Write-Host "ID: $($dev.id), Name: $($dev.name), IP: $($dev.ip_address), SN: $($dev.serial_number)"
    if ($dev.name -eq "ZKTeco Tang 1" -or $dev.serial_number -eq "8116255100516") {
        $targetDevice = $dev
    }
}

if ($targetDevice) {
    Write-Host "Found target device to delete: $($targetDevice.name) ($($targetDevice.id))"
    $deleteUrl = "http://localhost:8085/api/v1/devices/$($targetDevice.id)"
    
    try {
        # Sử dụng WebRequest để có thể đọc được mã trạng thái 204 No Content dễ dàng hơn
        $deleteResp = Invoke-WebRequest -Uri $deleteUrl -Method Delete -Headers $headers
        Write-Host "Delete response status code: $($deleteResp.StatusCode)"
        Write-Host "Delete response content length: $($deleteResp.Content.Length)"
    } catch {
        Write-Host "Error during delete request: $_"
    }

    # Verify deletion by listing again
    $devicesAfter = Invoke-RestMethod -Uri $devicesUrl -Method Get -Headers $headers
    Write-Host "Devices in system after deletion:"
    $foundAfter = $false
    foreach ($dev in $devicesAfter) {
        Write-Host "ID: $($dev.id), Name: $($dev.name)"
        if ($dev.id -eq $targetDevice.id) {
            $foundAfter = $true
        }
    }
    if (-not $foundAfter) {
        Write-Host "Verification Success: Device has been successfully removed from database!"
    } else {
        Write-Host "Verification Failure: Device still exists in database!"
    }
} else {
    Write-Host "Target device 'ZKTeco Tang 1' (SN: 8116255100516) was NOT found in the database. It might have already been deleted!"
}
