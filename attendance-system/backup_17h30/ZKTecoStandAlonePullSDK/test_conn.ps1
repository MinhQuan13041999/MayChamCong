$BASE_URL = "http://localhost:8085/api/v1"

Write-Host "1. Logging in to get token..."
try {
    $response = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/auth/login" `
      -ContentType "application/json" `
      -Body '{"username":"admin","password":"admin123"}'
    
    $token = $response.token
    $headers = @{ Authorization = "Bearer $token" }
    Write-Host "=> Login Success! Token obtained."
} catch {
    Write-Host "Login error: $_"
    exit 1
}

Write-Host "2. Registering ZKTeco device (192.168.11.151:4370)..."
$zkDevId = $null
try {
    $body = @{
      name = "ZKTeco Real Device"
      device_type = "zkteco"
      ip_address = "192.168.11.151"
      port = 4370
      serial_number = "ZK-REAL-151"
      location = "Technical Room"
    } | ConvertTo-Json

    $device = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices" `
      -ContentType "application/json" `
      -Headers $headers `
      -Body $body

    $zkDevId = $device.id
    Write-Host "=> Registered successfully! ID: $zkDevId"
} catch {
    Write-Host "Register failed or already exists. Searching for device ID..."
    try {
        $devices = Invoke-RestMethod -Method Get -Uri "$BASE_URL/devices" -Headers $headers
        $existing = $devices | Where-Object { $_.ip_address -eq "192.168.11.151" }
        if ($existing) {
            $zkDevId = $existing.id
            Write-Host "=> Found existing device ID: $zkDevId"
        } else {
            Write-Host "=> Device not found."
            exit 1
        }
    } catch {
        Write-Host "Error fetching devices: $_"
        exit 1
    }
}

Write-Host "3. Testing connection from Server to ZKTeco device..."
try {
    $status = Invoke-RestMethod -Method Get `
      -Uri "$BASE_URL/devices/$zkDevId/status" `
      -Headers $headers
    
    Write-Host "=== DEVICE STATUS ==="
    Write-Host "Online:       $($status.online)"
    Write-Host "Firmware:     $($status.firmware_info)"
    Write-Host "User Count:   $($status.user_count)"
    Write-Host "Log Count:    $($status.log_count)"
} catch {
    Write-Host "Error connecting to device via server: $_"
}
