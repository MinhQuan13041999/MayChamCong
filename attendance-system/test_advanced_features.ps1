# =============================================================================
# ATTENDANCE SYSTEM - ADVANCED FEATURES TEST SCRIPT (PowerShell)
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
# PHAN 1: QUAN LY CA LAM VIEC (SHIFT)
# =============================================================================
Write-Host "`n=== PHAN 1: QUAN LY CA LAM VIEC (SHIFT) ===" -ForegroundColor Magenta

# 1.1 Tao ca hanh chinh
Write-Host "1.1 Tao ca Hanh Chinh (08:00 - 17:00)..." -ForegroundColor Cyan
$s1 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/shifts" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"name":"Ca Hanh Chinh","start_time":"08:00","end_time":"17:00"}'
Write-Host "OK - Ca ID: $($s1.id) | Ten: $($s1.name) | Gio: $($s1.start_time) - $($s1.end_time)" -ForegroundColor Green
$shiftId = $s1.id

# 1.2 Danh sach ca
Write-Host "1.2 Danh sach ca lam viec:" -ForegroundColor Cyan
$shiftList = Invoke-RestMethod -Uri "$BASE_URL/shifts" -Headers $headers
$shiftList | Format-Table id, name, start_time, end_time -AutoSize

# =============================================================================
# PHAN 2: GAN CA CHO NHAN VIEN (EMPLOYEE SHIFT)
# =============================================================================
Write-Host "`n=== PHAN 2: GAN CA CHO NHAN VIEN ===" -ForegroundColor Magenta

# 2.1 Tao nhan vien de test (Don dep neu da ton tai de dam bao tinh idempotent)
Write-Host "2.1 Don dep & Tao nhan vien test (EMP999)..." -ForegroundColor Cyan
try {
    $res = Invoke-RestMethod -Uri "$BASE_URL/employees" -Headers $headers
    $allEmployees = @()
    if ($res -is [string]) {
        $allEmployees = $res | ConvertFrom-Json
    } else {
        $allEmployees = $res
    }
    foreach ($empItem in $allEmployees) {
        if ($empItem.employee_code -eq "EMP999") {
            Write-Host "  Phat hien EMP999 cu (ID: $($empItem.id)), dang xoa..." -ForegroundColor Yellow
            Invoke-RestMethod -Method Delete -Uri "$BASE_URL/employees/$($empItem.id)" -Headers $headers | Out-Null
        }
    }
} catch {
    Write-Host "  Loi khi kiem tra/don dep EMP999: $_" -ForegroundColor Yellow
}

$emp = Invoke-RestMethod -Method Post -Uri "$BASE_URL/employees" `
  -ContentType "application/json" -Headers $headers `
  -Body '{"employee_code":"EMP999","full_name":"Test Advanced User","card_no":"99999999"}'
Write-Host "OK - Employee ID: $($emp.id)" -ForegroundColor Green
$empId = $emp.id

# 2.2 Gan ca
Write-Host "2.2 Gan ca Hanh Chinh cho EMP999..." -ForegroundColor Cyan
$es = Invoke-RestMethod -Method Post -Uri "$BASE_URL/employees/$empId/shifts" `
  -ContentType "application/json" -Headers $headers `
  -Body "{`"shift_id`":`"$shiftId`",`"start_date`":`"2026-07-01`"}"
Write-Host "OK - Gan ca ID: $($es.id) | Bat dau: $($es.start_date)" -ForegroundColor Green

# =============================================================================
# PHAN 3: DANG KY & DUYET NGHIPHEP (LEAVE REQUEST)
# =============================================================================
Write-Host "`n=== PHAN 3: DANG KY & DUYET NGHI PHEP ===" -ForegroundColor Magenta

# 3.1 Gui don xin nghi (Nghi om ngay 2026-07-15)
Write-Host "3.1 Gui don xin nghi om ngay 2026-07-15..." -ForegroundColor Cyan
$lr = Invoke-RestMethod -Method Post -Uri "$BASE_URL/leave-requests" `
  -ContentType "application/json" -Headers $headers `
  -Body "{
    `"employee_id`": `"$empId`",
    `"leave_type`": `"sick`",
    `"start_date`": `"2026-07-15`",
    `"end_date`": `"2026-07-15`",
    `"reason`": `"Bi sot xuat huyet`"
  }"
Write-Host "OK - Don nghi ID: $($lr.id) | Trang thai: $($lr.status)" -ForegroundColor Green
$leaveId = $lr.id

# 3.2 Duyet don
Write-Host "3.2 Duyet don nghi..." -ForegroundColor Cyan
$approveLr = Invoke-RestMethod -Method Post -Uri "$BASE_URL/leave-requests/$leaveId/approve" -Headers $headers
Write-Host "OK - $($approveLr.message)" -ForegroundColor Green

# 3.3 Xem danh sach don nghi cua employee
$leaves = Invoke-RestMethod -Uri "$BASE_URL/leave-requests?employee_id=$empId" -Headers $headers
Write-Host "Danh sach don nghi: $($leaves.Count) don" -ForegroundColor Green
$leaves | Format-Table id, leave_type, start_date, end_date, status -AutoSize

# =============================================================================
# PHAN 4: DANG KY & DUYET OT (OVERTIME REQUEST)
# =============================================================================
Write-Host "`n=== PHAN 4: DANG KY & DUYET OT ===" -ForegroundColor Magenta

# 4.1 Gui don OT (Ngay 2026-07-16 tu 17:30 den 20:30)
Write-Host "4.1 Gui don OT ngay 2026-07-16 (17:30 - 20:30)..." -ForegroundColor Cyan
$ot = Invoke-RestMethod -Method Post -Uri "$BASE_URL/overtime-requests" `
  -ContentType "application/json" -Headers $headers `
  -Body "{
    `"employee_id`": `"$empId`",
    `"date`": `"2026-07-16`",
    `"start_time`": `"17:30`",
    `"end_time`": `"20:30`"
  }"
Write-Host "OK - Don OT ID: $($ot.id) | Trang thai: $($ot.status)" -ForegroundColor Green
$otId = $ot.id

# 4.2 Duyet don OT
Write-Host "4.2 Duyet don OT..." -ForegroundColor Cyan
$approveOt = Invoke-RestMethod -Method Post -Uri "$BASE_URL/overtime-requests/$otId/approve" -Headers $headers
Write-Host "OK - $($approveOt.message)" -ForegroundColor Green

# =============================================================================
# PHAN 5: MOCK CHAM CONG THO (RAW ATTENDANCE LOG)
# =============================================================================
Write-Host "`n=== PHAN 5: MOCK CHAM CONG THO ===" -ForegroundColor Magenta

# 5.1 Post mock raw logs cho ngay 2026-07-16 (Check-in 07:55, Check-out 17:05)
Write-Host "5.1 Post mock check-in & check-out..." -ForegroundColor Cyan
# Device ID gia lap (de trong de test raw insert khong phu thuoc device that)
$dummyDeviceId = ""

$logBody = @"
[
  {
    "device_id": "$dummyDeviceId",
    "employee_code": "EMP999",
    "check_time": "2026-07-16T07:55:00+07:00",
    "check_type": "in",
    "verify_mode": "fingerprint"
  },
  {
    "device_id": "$dummyDeviceId",
    "employee_code": "EMP999",
    "check_time": "2026-07-16T17:05:00+07:00",
    "check_type": "out",
    "verify_mode": "fingerprint"
  }
]
"@

$mockLogs = Invoke-RestMethod -Method Post -Uri "$BASE_URL/attendance-logs" `
  -ContentType "application/json" -Headers $headers -Body $logBody
Write-Host "OK - $($mockLogs.message) | inserted: $($mockLogs.inserted)" -ForegroundColor Green

# =============================================================================
# PHAN 6: TINH CONG TU DONG (ATTENDANCE PROCESSING ENGINE)
# =============================================================================
Write-Host "`n=== PHAN 6: TINH CONG TU DONG ===" -ForegroundColor Magenta

# 6.1 Tinh cong ngay nghi phep (2026-07-15)
Write-Host "6.1 Tinh cong ngay 2026-07-15 (Ngay nghi phep)..." -ForegroundColor Cyan
$p1 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/daily-attendance/process" `
  -ContentType "application/json" -Headers $headers -Body '{"date":"2026-07-15"}'
Write-Host "OK - $($p1.message)" -ForegroundColor Green

# 6.2 Tinh cong ngay lam viec va OT (2026-07-16)
Write-Host "6.2 Tinh cong ngay 2026-07-16 (Di lam du gio + OT)..." -ForegroundColor Cyan
$p2 = Invoke-RestMethod -Method Post -Uri "$BASE_URL/daily-attendance/process" `
  -ContentType "application/json" -Headers $headers -Body '{"date":"2026-07-16"}'
Write-Host "OK - $($p2.message)" -ForegroundColor Green

# =============================================================================
# PHAN 7: BAO CAO CONG (DAILY ATTENDANCE REPORT)
# =============================================================================
Write-Host "`n=== PHAN 7: BAO CAO CONG (REPORT) ===" -ForegroundColor Magenta

Write-Host "7.1 Xem bao cao cong cua EMP999 tu 2026-07-14 den 2026-07-17:" -ForegroundColor Cyan
$report = Invoke-RestMethod -Uri "$BASE_URL/daily-attendance/report?employee_id=$empId&from=2026-07-14&to=2026-07-17" -Headers $headers

# Hien thi chi tiet bao cao
$report | Format-Table @{Label="Ngay"; Expression={([datetime]$_.date).ToString("yyyy-MM-dd")}}, 
                       attendance_status, 
                       @{Label="Check-in"; Expression={if($_.first_in){([datetime]$_.first_in).ToString("HH:mm")}else{"-"}}}, 
                       @{Label="Check-out"; Expression={if($_.last_out){([datetime]$_.last_out).ToString("HH:mm")}else{"-"}}}, 
                       late_minutes, early_minutes, working_hours -AutoSize

# =============================================================================
# PHAN 8: NHAT KY HOAT DONG (AUDIT LOG)
# =============================================================================
Write-Host "`n=== PHAN 8: NHAT KY HOAT DONG (AUDIT LOG) ===" -ForegroundColor Magenta

$auditList = Invoke-RestMethod -Uri "$BASE_URL/audit-logs?limit=5" -Headers $headers
Write-Host "5 Audit Logs gan nhat:" -ForegroundColor Cyan
$auditList | Format-Table @{Label="Thoi gian"; Expression={([datetime]$_.created_at).ToString("yyyy-MM-dd HH:mm:ss")}}, 
                          action, object_type, description, ip_address -AutoSize

# Clean up
Write-Host "`nClean up test user & shift..." -ForegroundColor Cyan
Invoke-RestMethod -Method Delete -Uri "$BASE_URL/employees/$empId" -Headers $headers | Out-Null
Invoke-RestMethod -Method Delete -Uri "$BASE_URL/shifts/$shiftId" -Headers $headers | Out-Null
Write-Host "OK - Da clean up" -ForegroundColor Green

Write-Host "`n=============================================" -ForegroundColor Magenta
Write-Host "HOAN THANH TEST TOAN BO TINH NANG NANG CAO" -ForegroundColor Magenta
Write-Host "=============================================" -ForegroundColor Magenta
