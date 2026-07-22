param(
  [string]$BaseUrl = "http://localhost:8085/api/v1",
  [string]$Username = "admin",
  [string]$Password = "admin123",
  [string]$DeviceId = "",
  [string]$DeviceSerialADMS = "",
  [string]$EmployeeCode = "NV001",
  [string]$FullName = "Nguyen Van A",
  [string]$CardNo = "0001234567",
  [string]$DepartmentId = "Kỹ thuật / CNTT",
  [string]$Email = "nv.a@example.com",
  [string]$Phone = "0901234567",
  [string]$Gender = "male",
  [string]$Dob = "1990-01-01",
  [string]$JoinDate = "2026-07-15",
  [string]$JobTitle = "Kỹ sư",
  [string]$AvatarUrl = "",
  [string]$DeviceUserId = ""
)

function Write-ErrorAndExit($message) {
  Write-Host "ERROR: $message" -ForegroundColor Red
  exit 1
}

Write-Host "=== Attendance System Auto Enroll Script ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

if (-not $Username) {
  $Username = Read-Host "Admin username"
}
if (-not $Password) {
  $securePw = Read-Host -AsSecureString "Admin password"
  $Password = [System.Net.NetworkCredential]::new("", $securePw).Password
}
if (-not $DeviceUserId) {
  $DeviceUserId = $EmployeeCode
}

$loginBody = @{ username = $Username; password = $Password } | ConvertTo-Json
try {
  $loginResponse = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/login" -ContentType "application/json" -Body $loginBody
} catch {
  Write-ErrorAndExit "Login failed: $($_.Exception.Message)"
}

if (-not $loginResponse.token) {
  Write-ErrorAndExit "Login did not return a token."
}

$token = $loginResponse.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Logged in successfully. Token received." -ForegroundColor Green

if (-not $DeviceId -and $DeviceSerialADMS) {
  Write-Host "Looking up device by serial_number_adms = $DeviceSerialADMS..."
  try {
    $devices = Invoke-RestMethod -Method Get -Uri "$BaseUrl/devices" -Headers $headers
  } catch {
    Write-ErrorAndExit "Failed to fetch devices: $($_.Exception.Message)"
  }
  $found = $devices | Where-Object { $_.serial_number_adms -eq $DeviceSerialADMS }
  if (-not $found) {
    Write-ErrorAndExit "No device found with serial_number_adms='$DeviceSerialADMS'."
  }
  $DeviceId = $found.id
  Write-Host "Found device ID: $DeviceId" -ForegroundColor Green
}

if (-not $DeviceId) {
  Write-ErrorAndExit "deviceId is required. Provide -DeviceId or -DeviceSerialADMS."
}

Write-Host "Checking device status for device_id = $DeviceId..."
try {
  $status = Invoke-RestMethod -Method Get -Uri "$BaseUrl/devices/$DeviceId/status" -Headers $headers
  Write-Host "Device status:" -ForegroundColor Green
  $status | Format-List
} catch {
  Write-Host "Warning: Could not get device status. The device may be offline or not reachable." -ForegroundColor Yellow
  Write-Host "Exception: $($_.Exception.Message)" -ForegroundColor Yellow
}

$employeeBody = @{
  employee_code      = $EmployeeCode
  full_name          = $FullName
  department_id      = $DepartmentId
  card_no            = $CardNo
  email              = $Email
  phone              = $Phone
  gender             = $Gender
  dob                = $Dob
  join_date          = $JoinDate
  job_title          = $JobTitle
  avatar_url         = $AvatarUrl
  enroll_fingerprint = $true
  device_id          = $DeviceId
  device_user_id     = $DeviceUserId
} | ConvertTo-Json

Write-Host "Creating employee and enqueueing ADMS enroll commands..."
try {
  $employee = Invoke-RestMethod -Method Post -Uri "$BaseUrl/employees" -ContentType "application/json" -Headers $headers -Body $employeeBody
} catch {
  Write-ErrorAndExit "Employee creation failed: $($_.Exception.Message)"
}

Write-Host "Employee created successfully:" -ForegroundColor Green
$employee | Format-List

Write-Host "Fetching employee-device mappings..."
try {
  $mappings = Invoke-RestMethod -Method Get -Uri "$BaseUrl/employees/$($employee.id)/device-mappings" -Headers $headers
  $mappings | Format-Table id, device_id, device_user_id, sync_status, fingerprint_enrolled, fingerprint_enrolled_at -AutoSize
} catch {
  Write-Host "Warning: unable to fetch device mappings: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "Script completed. Nếu thiết bị đang kết nối ADMS, lệnh đã được xếp hàng và máy sẽ nhận khi poll /iclock/getrequest." -ForegroundColor Cyan
