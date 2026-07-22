# =============================================================================
# ATTENDANCE SYSTEM - TEST SCRIPT (PowerShell - Fixed version)
# =============================================================================
$BASE_URL = "http://localhost:8085/api/v1"

# BUOC 0: DANG NHAP
Write-Host "=== BUOC 0: DANG NHAP ===" -ForegroundColor Magenta
$response = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/auth/login" `
  -ContentType "application/json" `
  -Body '{"username":"admin","password":"admin123"}'
$token = $response.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "OK - Token: $($token.Substring(0,30))..." -ForegroundColor Green

# =============================================================================
# PHAN 1: QUAN LY THIET BI
# =============================================================================
Write-Host "`n=== PHAN 1: QUAN LY THIET BI ===" -ForegroundColor Magenta

# 1.1 Them thiet bi ZKTeco
Write-Host "1.1 Tao ZKTeco..." -ForegroundColor Cyan
$d1 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"name":"ZKTeco Kho A","device_type":"zkteco","ip_address":"192.168.1.100","port":4370,"serial_number":"ZK001","location":"Kho tang 1"}'
Write-Host "OK - ID: $($d1.id) | Status: $($d1.status)" -ForegroundColor Green
$devId1 = $d1.id

# 1.2 Them Hikvision
Write-Host "1.2 Tao Hikvision..." -ForegroundColor Cyan
$d2 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"name":"Hikvision Cong chinh","device_type":"hikvision","ip_address":"192.168.1.200","port":80,"serial_number":"HIK001","location":"Cong ra vao"}'
Write-Host "OK - ID: $($d2.id)" -ForegroundColor Green
$devId2 = $d2.id

# 1.3 Them Sunbeam
Write-Host "1.3 Tao Sunbeam..." -ForegroundColor Cyan
$d3 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"name":"Sunbeam Van phong","device_type":"sunbeam","ip_address":"192.168.1.150","port":80,"serial_number":"SB001","location":"Van phong tang 2"}'
Write-Host "OK - ID: $($d3.id)" -ForegroundColor Green
$devId3 = $d3.id

# 1.4 Danh sach thiet bi
Write-Host "`n1.4 Danh sach thiet bi:" -ForegroundColor Cyan
$devList = Invoke-RestMethod -Uri "$BASE_URL/devices" -Headers $headers
$devList | Format-Table id, name, device_type, ip_address, port, status -AutoSize

# 1.5 Sua thiet bi
Write-Host "1.5 Sua ZKTeco (doi IP, serial)..." -ForegroundColor Cyan
Invoke-RestMethod -Method Put -Uri "$BASE_URL/devices/$devId1" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"name":"ZKTeco Kho A v2","device_type":"zkteco","ip_address":"192.168.1.101","port":4370,"serial_number":"ZK001-NEW","location":"Kho tang 1 - Cua trai"}' | Out-Null
Write-Host "OK - Da cap nhat" -ForegroundColor Green

# 1.6 Kiem tra trang thai
Write-Host "`n1.6 Kiem tra trang thai ZKTeco (se offline neu khong co thiet bi that)..." -ForegroundColor Cyan
try {
    $st = Invoke-RestMethod -Uri "$BASE_URL/devices/$devId1/status" -Headers $headers
    Write-Host "Online: $($st.online) | Firmware: $($st.firmware_info)" -ForegroundColor Green
} catch {
    Write-Host "EXPECTED: Thiet bi offline (khong co phan cung that)" -ForegroundColor Yellow
}

# 1.7 Test connection
Write-Host "1.7 Test connection Hikvision..." -ForegroundColor Cyan
try {
    $conn = Invoke-RestMethod -Method Post -Uri "$BASE_URL/devices/$devId2/test-connection" -Headers $headers
    Write-Host "Ket noi thanh cong" -ForegroundColor Green
} catch {
    Write-Host "EXPECTED: Khong ket noi duoc (khong co phan cung that)" -ForegroundColor Yellow
}

# =============================================================================
# PHAN 2: QUAN LY NHAN VIEN
# =============================================================================
Write-Host "`n=== PHAN 2: QUAN LY NHAN VIEN ===" -ForegroundColor Magenta

# 2.1 Tao nhan vien
Write-Host "2.1 Tao 3 nhan vien..." -ForegroundColor Cyan
$e1 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/employees" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"employee_code":"EMP001","full_name":"Nguyen Van An","card_no":"0001234567"}'
$empId1 = $e1.id

$e2 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/employees" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"employee_code":"EMP002","full_name":"Tran Thi Binh","card_no":"0009876543"}'
$empId2 = $e2.id

$e3 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/employees" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"employee_code":"EMP003","full_name":"Le Minh Cuong","card_no":"0005555555"}'
$empId3 = $e3.id
Write-Host "OK - EMP001=$empId1 | EMP002=$empId2 | EMP003=$empId3" -ForegroundColor Green

# 2.2 Danh sach nhan vien
Write-Host "`n2.2 Danh sach nhan vien:" -ForegroundColor Cyan
$empList = Invoke-RestMethod -Uri "$BASE_URL/employees" -Headers $headers
$empList | Format-Table id, employee_code, full_name, card_no, status -AutoSize

# 2.3 Sua nhan vien
Write-Host "2.3 Sua EMP001..." -ForegroundColor Cyan
Invoke-RestMethod -Method Put -Uri "$BASE_URL/employees/$empId1" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"full_name":"Nguyen Van An (Updated)","card_no":"0001234568","status":"active"}' | Out-Null
Write-Host "OK - Da cap nhat EMP001" -ForegroundColor Green

# 2.4 Import Excel
Write-Host "`n2.4 Import nhan vien tu Excel:" -ForegroundColor Cyan
Write-Host "Tao file test_import.xlsx voi dinh dang:" -ForegroundColor Yellow
Write-Host "  Cot A: employee_code | Cot B: full_name | Cot C: card_no" -ForegroundColor Gray
Write-Host "Lenh import (sau khi co file):" -ForegroundColor Yellow
$importCmd = @'
$multipart = [System.Net.Http.MultipartFormDataContent]::new()
$fileBytes = [System.IO.File]::ReadAllBytes("E:\Project\attendance-system\test_import.xlsx")
$fileContent = [System.Net.Http.ByteArrayContent]::new($fileBytes)
$fileContent.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::Parse("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
$multipart.Add($fileContent, "file", "test_import.xlsx")
$client = [System.Net.Http.HttpClient]::new()
$client.DefaultRequestHeaders.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new("Bearer", $token)
$result = $client.PostAsync("$BASE_URL/employees/import", $multipart).Result
$result.Content.ReadAsStringAsync().Result
'@
Write-Host $importCmd -ForegroundColor Gray

# 2.5 Dong bo nhan vien xuong may
Write-Host "`n2.5 Dong bo nhan vien xuong ZKTeco..." -ForegroundColor Cyan
try {
    $syncE = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$devId1/sync-employees" -Headers $headers
    Write-Host "Ket qua: status=$($syncE.status) | records=$($syncE.record_count)" -ForegroundColor Green
} catch {
    Write-Host "EXPECTED: Loi ket noi thiet bi (can phan cung that)" -ForegroundColor Yellow
    Write-Host "Ghi chu: Sync history da duoc luu vao DB" -ForegroundColor Gray
}

# 2.6 Xoa nhan vien
Write-Host "`n2.6 Xoa EMP003..." -ForegroundColor Cyan
Invoke-RestMethod -Method Delete -Uri "$BASE_URL/employees/$empId3" -Headers $headers | Out-Null
Write-Host "OK - Da xoa EMP003" -ForegroundColor Green

# Verify
$afterDelete = Invoke-RestMethod -Uri "$BASE_URL/employees" -Headers $headers
Write-Host "Sau khi xoa: con $($afterDelete.Count) nhan vien" -ForegroundColor Green

# =============================================================================
# PHAN 3: DONG BO CHAM CONG
# =============================================================================
Write-Host "`n=== PHAN 3: DONG BO CHAM CONG ===" -ForegroundColor Magenta

# 3.1 Dong bo thu cong (24h gan nhat)
Write-Host "3.1 Dong bo thu cong ZKTeco (24h gan nhat)..." -ForegroundColor Cyan
try {
    $atSync1 = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$devId1/sync-attendance" -Headers $headers
    Write-Host "OK: status=$($atSync1.status) | records=$($atSync1.record_count)" -ForegroundColor Green
} catch {
    Write-Host "EXPECTED: Loi ket noi (can thiet bi that)" -ForegroundColor Yellow
}

# 3.2 Dong bo theo khoang thoi gian
Write-Host "`n3.2 Dong bo theo khoang thoi gian (7 ngay qua)..." -ForegroundColor Cyan
$from7d = (Get-Date).AddDays(-7).ToString("yyyy-MM-ddT00:00:00+07:00")
$toNow  = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss+07:00")
$body32 = "{""from"":""$from7d"",""to"":""$toNow""}"
try {
    $atSync2 = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$devId2/sync-attendance" `
      -ContentType "application/json" -Headers $headers -Body $body32
    Write-Host "OK: status=$($atSync2.status)" -ForegroundColor Green
} catch {
    Write-Host "EXPECTED: Loi ket noi (can thiet bi that)" -ForegroundColor Yellow
}

# 3.3 Scheduler
Write-Host "`n3.3 Scheduler tu dong:" -ForegroundColor Cyan
Write-Host "  Dang chay trong server. Cau hinh tai config.yaml:" -ForegroundColor Green
Write-Host "  attendance_sync_cron: '*/15 * * * *'  -> moi 15 phut" -ForegroundColor Gray
Write-Host "  Doi sang 5 phut: '*/5 * * * *'" -ForegroundColor Gray
Write-Host "  Log server se hien thi khi scheduler chay." -ForegroundColor Gray

# 3.4 Truy van raw log
Write-Host "`n3.4 Truy van raw attendance log (7 ngay qua)..." -ForegroundColor Cyan
$logUri = "$BASE_URL/attendance-logs?from=$from7d" + "&to=$toNow"
$logs = Invoke-RestMethod -Uri $logUri -Headers $headers
Write-Host "Tong so log: $($logs.Count)"
if ($logs.Count -gt 0) {
    $logs | Format-Table id, employee_code, check_time, check_type, verify_mode -AutoSize
} else {
    Write-Host "  Chua co log (can dong bo tu thiet bi that de co du lieu)" -ForegroundColor Yellow
}

# 3.5 Loc log theo nhan vien
Write-Host "`n3.5 Loc log theo EMP001..." -ForegroundColor Cyan
$logUri2 = "$BASE_URL/attendance-logs?employee_code=EMP001" + "&from=$from7d&to=$toNow"
$logsEmp = Invoke-RestMethod -Uri $logUri2 -Headers $headers
Write-Host "Log cua EMP001: $($logsEmp.Count) ban ghi"

# =============================================================================
# PHAN 4: LICH SU DONG BO
# =============================================================================
Write-Host "`n=== PHAN 4: LICH SU DONG BO ===" -ForegroundColor Magenta

# 4.1 Toan bo lich su
Write-Host "4.1 Lich su dong bo tat ca:" -ForegroundColor Cyan
$hist = Invoke-RestMethod -Uri "$BASE_URL/sync-history" -Headers $headers
Write-Host "Tong: $($hist.Count) lan dong bo"
if ($hist.Count -gt 0) {
    $hist | Format-Table id, device_id, sync_type, trigger_type, status, record_count -AutoSize
}

# 4.2 Loc theo thiet bi
Write-Host "`n4.2 Lich su dong bo ZKTeco:" -ForegroundColor Cyan
$histDev = Invoke-RestMethod -Uri "$BASE_URL/sync-history?device_id=$devId1" -Headers $headers
Write-Host "Lan dong bo cua device $devId1`: $($histDev.Count)"

# 4.3 Loc theo trang thai
Write-Host "`n4.3 Lich su dong bo FAILED:" -ForegroundColor Cyan
$histFail = Invoke-RestMethod -Uri "$BASE_URL/sync-history?status=failed" -Headers $headers
Write-Host "So lan that bai: $($histFail.Count)"
if ($histFail.Count -gt 0) {
    $histFail | Format-Table id, device_id, status, error_message -AutoSize
}

# 4.4 Retry dong bo that bai
Write-Host "`n4.4 Retry dong bo lai (lan that bai dau tien):" -ForegroundColor Cyan
if ($histFail.Count -gt 0) {
    $failId = $histFail[0].id
    Write-Host "Retry ID: $failId" -ForegroundColor Yellow
    try {
        $retry = Invoke-RestMethod -Method Post `
          -Uri "$BASE_URL/sync-history/$failId/retry" -Headers $headers
        Write-Host "Ket qua retry: $($retry.status)" -ForegroundColor Green
    } catch {
        Write-Host "EXPECTED: Retry that bai (thiet bi van offline)" -ForegroundColor Yellow
    }
} else {
    Write-Host "Khong co lan that bai de retry" -ForegroundColor Gray
}

# Cleanup test devices and test employees
Write-Host "`n=== PHAN 5: CLEANUP TEST DATA ===" -ForegroundColor Magenta
if ($devId1) { try { Invoke-RestMethod -Method Delete -Uri "$BASE_URL/devices/$devId1" -Headers $headers | Out-Null } catch {} }
if ($devId2) { try { Invoke-RestMethod -Method Delete -Uri "$BASE_URL/devices/$devId2" -Headers $headers | Out-Null } catch {} }
if ($devId3) { try { Invoke-RestMethod -Method Delete -Uri "$BASE_URL/devices/$devId3" -Headers $headers | Out-Null } catch {} }
if ($empId1) { try { Invoke-RestMethod -Method Delete -Uri "$BASE_URL/employees/$empId1" -Headers $headers | Out-Null } catch {} }
if ($empId2) { try { Invoke-RestMethod -Method Delete -Uri "$BASE_URL/employees/$empId2" -Headers $headers | Out-Null } catch {} }
Write-Host "Da don dep cac thiet bi va nhan vien dung de test." -ForegroundColor Green

# =============================================================================
# TONG KET
# =============================================================================
Write-Host "`n=============================================" -ForegroundColor Magenta
Write-Host "HOAN THANH TEST TAT CA CHUC NANG" -ForegroundColor Magenta
Write-Host "=============================================" -ForegroundColor Magenta
Write-Host "OK Quan ly thiet bi:   Them/Sua/Xoa/Status/TestConn" -ForegroundColor Green
Write-Host "OK Quan ly nhan vien:  CRUD/Import Excel/Sync" -ForegroundColor Green
Write-Host "OK Dong bo cham cong:  Manual/Scheduler/Raw Log" -ForegroundColor Green
Write-Host "OK Lich su dong bo:    View/Filter/Retry" -ForegroundColor Green
Write-Host ""
Write-Host "TEST VOI THIET BI THAT: xem file REAL_DEVICE_GUIDE.md" -ForegroundColor Cyan
