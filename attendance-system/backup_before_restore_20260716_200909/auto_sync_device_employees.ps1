param(
  [string]$BaseUrl = "http://localhost:8085/api/v1",
  [string]$Username = "admin",
  [string]$Password = "admin123",
  [string]$DeviceId = "",
  [string]$DeviceSerialADMS = ""
)

function Write-ErrorAndExit($message) {
  Write-Host "ERROR: $message" -ForegroundColor Red
  exit 1
}

Write-Host "=== Attendance System Auto Sync Device Employees ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

if (-not $DeviceId -and -not $DeviceSerialADMS) {
  Write-ErrorAndExit "Provide -DeviceId or -DeviceSerialADMS"
}

if (-not $DeviceId -and $DeviceSerialADMS) {
  Write-Host "Looking up device by serial_number_adms = $DeviceSerialADMS..."
  try {
    $loginBody = @{ username = $Username; password = $Password } | ConvertTo-Json
    $loginResponse = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/login" -ContentType "application/json" -Body $loginBody
    $token = $loginResponse.token
    if (-not $token) { Write-ErrorAndExit "Login failed, no token returned." }
    $headers = @{ Authorization = "Bearer $token" }
    $devices = Invoke-RestMethod -Method Get -Uri "$BaseUrl/devices" -Headers $headers
  } catch {
    Write-ErrorAndExit "Failed to lookup device: $($_.Exception.Message)"
  }
  $found = $devices | Where-Object { $_.serial_number_adms -eq $DeviceSerialADMS }
  if (-not $found) {
    Write-ErrorAndExit "No device found with serial_number_adms = $DeviceSerialADMS"
  }
  $DeviceId = $found.id
  Write-Host "Found device ID: $DeviceId" -ForegroundColor Green
}

try {
  $loginBody = @{ username = $Username; password = $Password } | ConvertTo-Json
  $loginResponse = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/login" -ContentType "application/json" -Body $loginBody
  $token = $loginResponse.token
  if (-not $token) { Write-ErrorAndExit "Login failed, no token returned." }
  $headers = @{ Authorization = "Bearer $token" }
} catch {
  Write-ErrorAndExit "Login failed: $($_.Exception.Message)"
}

Write-Host "Syncing employees to device ID $DeviceId..."
try {
  $response = Invoke-RestMethod -Method Post -Uri "$BaseUrl/devices/$DeviceId/sync-employees" -Headers $headers
  Write-Host "Sync API response:" -ForegroundColor Green
  $response | ConvertTo-Json -Depth 5 | Write-Host
} catch {
  Write-ErrorAndExit "Sync request failed: $($_.Exception.Message)"
}

Write-Host "Done. Nếu thiết bị đang kết nối ADMS, các lệnh nhân viên đã được xếp hàng." -ForegroundColor Cyan
