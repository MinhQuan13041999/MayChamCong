# HƯỚNG DẪN TEST VỚI MÁY CHẤM CÔNG THẬT
## Từng bước chi tiết

---

## BƯỚC 0 — Khởi động lại server

```powershell
# Terminal 1: Dừng server cũ nếu còn chạy (Ctrl+C), sau đó:
go run ./cmd/server
```

Phải thấy dòng này trước khi test:
```
INFO  server starting  {"port": 8085}
```

Kiểm tra nhanh:
```powershell
Invoke-RestMethod -Uri "http://localhost:8085/healthz"
```

---

## BƯỚC 1 — Kiểm tra điều kiện mạng

Trước khi test API, đảm bảo máy tính và thiết bị **cùng mạng LAN**.

```powershell
# Mở terminal mới (Terminal 2) - kiểm tra ping tới máy chấm công:

# ZKTeco (thay bằng IP thật của máy)
ping 192.168.1.100

# Hikvision
ping 192.168.1.200

# Sunbeam
ping 192.168.1.150
```

**Nếu ping không được**: kiểm tra cáp mạng, switch, hoặc cùng WiFi. Thử mở trình duyệt và truy cập `http://<IP_MAY>` để xem có phản hồi không.

---

## BƯỚC 2 — Đăng nhập lấy token

```powershell
# Terminal 2 (terminal mới, không phải terminal đang chạy server):
$BASE_URL = "http://localhost:8085/api/v1"

$response = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/auth/login" `
  -ContentType "application/json" `
  -Body '{"username":"admin","password":"admin123"}'

$token = $response.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Token OK: $($token.Substring(0,20))..."
```

---

## BƯỚC 3 — ZKTECO: Test từng bước

### 3.1 Tìm IP máy ZKTeco

Trên màn hình máy ZKTeco:
- Nhấn **Menu** → **Communication** → **Ethernet**
- Ghi lại: IP Address, Subnet Mask, Gateway
- Port mặc định: **4370**

### 3.2 Thêm thiết bị ZKTeco vào hệ thống

```powershell
# Thay <IP_ZKTECO> bằng IP thật của máy (ví dụ: 192.168.1.100)
$zkIP = "192.168.1.100"   # << ĐỔI IP NÀY

$devZK = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body "{
    `"name`": `"ZKTeco Tang 1`",
    `"device_type`": `"zkteco`",
    `"ip_address`": `"$zkIP`",
    `"port`": 4370,
    `"serial_number`": `"ZK-001`",
    `"location`": `"Tang 1 - Cua ra vao`"
  }"

$devZK | Format-List
$zkDevId = $devZK.id
Write-Host "ZKTeco Device ID: $zkDevId"
```

### 3.3 Kiểm tra kết nối ZKTeco

```powershell
# Test connection - sẽ kết nối TCP tới máy ZKTeco
try {
    $status = Invoke-RestMethod `
      -Uri "$BASE_URL/devices/$zkDevId/status" -Headers $headers
    Write-Host "=== ZKTECO STATUS ==="
    Write-Host "Online:   $($status.online)"
    Write-Host "Firmware: $($status.firmware_info)"
    Write-Host "Users:    $($status.user_count)"
    Write-Host "Logs:     $($status.log_count)"
} catch {
    Write-Host "Loi ket noi: $($_.Exception.Message)"
    Write-Host "Kiem tra: 1) IP dung chua? 2) Cung mang LAN? 3) Firewall?"
}
```

**Kết quả mong đợi khi thành công:**
```
Online:   True
Firmware: Ver 6.60 Sep 3 2019
Users:    5
Logs:     320
```

### 3.4 Thêm nhân viên vào hệ thống

```powershell
# Tạo nhân viên trong DB
$emp = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/employees" `
  -ContentType "application/json" -Headers $headers `
  -Body '{
    "employee_code": "001",
    "full_name": "Nguyen Van A",
    "card_no": "0001234567"
  }'
Write-Host "Employee ID: $($emp.id)"
```

### 3.5 Đồng bộ nhân viên xuống ZKTeco

```powershell
# Push nhân viên từ DB xuống máy ZKTeco
try {
    $syncEmp = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$zkDevId/sync-employees" -Headers $headers
    Write-Host "=== SYNC EMPLOYEE RESULT ==="
    Write-Host "Status:  $($syncEmp.status)"
    Write-Host "Records: $($syncEmp.record_count)"
    Write-Host "Error:   $($syncEmp.error_message)"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

### 3.6 Đọc log chấm công từ ZKTeco (thủ công)

```powershell
# Đọc 24h gần nhất
try {
    $attSync = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$zkDevId/sync-attendance" -Headers $headers
    Write-Host "=== ATTENDANCE SYNC RESULT ==="
    Write-Host "Status:  $($attSync.status)"
    Write-Host "Records: $($attSync.record_count)"
    Write-Host "Started: $($attSync.started_at)"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

### 3.7 Đọc log theo khoảng thời gian

```powershell
# Đọc 7 ngày qua
$from = (Get-Date).AddDays(-7).ToString("yyyy-MM-ddT00:00:00+07:00")
$to   = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss+07:00")
$body = "{`"from`":`"$from`",`"to`":`"$to`"}"

try {
    $attSync7d = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$zkDevId/sync-attendance" `
      -ContentType "application/json" -Headers $headers -Body $body
    Write-Host "Dong bo 7 ngay: $($attSync7d.record_count) ban ghi"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

---

## BƯỚC 4 — HIKVISION: Test từng bước

### 4.1 Tìm IP máy Hikvision

Trên trình duyệt, mở: `http://<IP_HIKVISION>` → đăng nhập bằng admin/admin hoặc mật khẩu thiết bị.

Hoặc dùng **SADP Tool** của Hikvision để scan thiết bị trên mạng.

### 4.2 Cập nhật credentials trong config.yaml

```powershell
# Mở config.yaml và thêm:
notepad E:\Project\attendance-system\config.yaml
```

Thêm vào file `config.yaml`:
```yaml
hikvision_username: "admin"
hikvision_password: "Admin@123"   # đổi thành mật khẩu thật
```

> **Lưu ý:** Hiện tại adapter dùng Digest Auth với username/password từ cấu hình thiết bị.
> Cập nhật field này trong entity Device sau khi thêm device.

### 4.3 Thêm thiết bị Hikvision

```powershell
$hikIP = "192.168.1.200"   # << ĐỔI IP NÀY

$devHik = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body "{
    `"name`": `"Hikvision Cong chinh`",
    `"device_type`": `"hikvision`",
    `"ip_address`": `"$hikIP`",
    `"port`": 80,
    `"serial_number`": `"HIK-001`",
    `"location`": `"Cong ra vao chinh`"
  }"

$hikDevId = $devHik.id
Write-Host "Hikvision Device ID: $hikDevId"
```

### 4.4 Test connection Hikvision

```powershell
try {
    $hikConn = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$hikDevId/test-connection" -Headers $headers
    Write-Host "Hikvision: Online=$($hikConn.online) | FW=$($hikConn.firmware_info)"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

### 4.5 Đọc log chấm công Hikvision

```powershell
try {
    $hikSync = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$hikDevId/sync-attendance" -Headers $headers
    Write-Host "Hikvision sync: $($hikSync.record_count) log"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

---

## BƯỚC 5 — SUNBEAM (TIMMY): Test từng bước

### 5.1 Tìm IP máy Sunbeam

Trên màn hình máy Sunbeam:
- Menu → Network Settings → IP Address
- Port mặc định: **80** (HTTP)

### 5.2 Thêm thiết bị Sunbeam

```powershell
$sbIP = "192.168.1.150"   # << ĐỔI IP NÀY

$devSB = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" -Headers $headers `
  -Body "{
    `"name`": `"Sunbeam Van phong`",
    `"device_type`": `"sunbeam`",
    `"ip_address`": `"$sbIP`",
    `"port`": 80,
    `"serial_number`": `"SB-001`",
    `"location`": `"Van phong tang 2`"
  }"

$sbDevId = $devSB.id
Write-Host "Sunbeam Device ID: $sbDevId"
```

### 5.3 Test kết nối và đọc log Sunbeam

```powershell
try {
    $sbStatus = Invoke-RestMethod `
      -Uri "$BASE_URL/devices/$sbDevId/status" -Headers $headers
    Write-Host "Sunbeam Online: $($sbStatus.online)"

    $sbSync = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/devices/$sbDevId/sync-attendance" -Headers $headers
    Write-Host "Sunbeam sync: $($sbSync.record_count) log"
} catch {
    Write-Host "Loi: $($_.Exception.Message)"
}
```

---

## BƯỚC 6 — Xem log chấm công đã lưu

```powershell
# Xem toàn bộ log 7 ngày qua
$from7d = (Get-Date).AddDays(-7).ToString("yyyy-MM-ddT00:00:00+07:00")
$toNow  = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss+07:00")
$logUri = "$BASE_URL/attendance-logs?from=$from7d" + "&to=$toNow"

$allLogs = Invoke-RestMethod -Uri $logUri -Headers $headers
Write-Host "Tong so log: $($allLogs.Count)"
$allLogs | Format-Table employee_code, check_time, check_type, verify_mode, device_id -AutoSize

# Lọc theo nhân viên
$empLogUri = "$BASE_URL/attendance-logs?employee_code=001" + "&from=$from7d&to=$toNow"
$empLogs = Invoke-RestMethod -Uri $empLogUri -Headers $headers
Write-Host "Log nhan vien 001: $($empLogs.Count) ban ghi"

# Lọc theo thiết bị
$devLogUri = "$BASE_URL/attendance-logs?device_id=$zkDevId" + "&from=$from7d&to=$toNow"
$devLogs = Invoke-RestMethod -Uri $devLogUri -Headers $headers
Write-Host "Log tu ZKTeco: $($devLogs.Count) ban ghi"
```

---

## BƯỚC 7 — Xem và retry lịch sử đồng bộ

```powershell
# Xem toàn bộ lịch sử
$hist = Invoke-RestMethod -Uri "$BASE_URL/sync-history" -Headers $headers
$hist | Format-Table id, device_id, sync_type, status, record_count, started_at -AutoSize

# Xem lần thất bại
$failed = Invoke-RestMethod -Uri "$BASE_URL/sync-history?status=failed" -Headers $headers
Write-Host "So lan that bai: $($failed.Count)"

# Retry lần thất bại gần nhất
if ($failed.Count -gt 0) {
    $failId = $failed[0].id
    $retry = Invoke-RestMethod -Method Post `
      -Uri "$BASE_URL/sync-history/$failId/retry" -Headers $headers
    Write-Host "Retry result: $($retry.status) | records=$($retry.record_count)"
}
```

---

## BƯỚC 8 — Cấu hình Scheduler tự động

Mở `config.yaml` và chỉnh:

```yaml
attendance_sync_cron: "*/5 * * * *"   # Mỗi 5 phút
# hoặc
attendance_sync_cron: "0 * * * *"     # Mỗi giờ
# hoặc
attendance_sync_cron: "0 6,12,18 * * *"  # 6h, 12h, 18h mỗi ngày
```

Sau khi sửa, **restart server**. Log server sẽ hiển thị:
```
INFO  scheduler job started  {"cron": "*/5 * * * *"}
INFO  auto sync attendance   {"device_id": "...", "records": 15}
```

---

## XỬ LÝ LỖI THƯỜNG GẶP

| Lỗi | Nguyên nhân | Giải pháp |
|-----|-------------|-----------|
| `connection refused` | Sai IP hoặc port | Kiểm tra IP trên màn hình máy, thử `telnet <IP> 4370` |
| `i/o timeout` | Firewall chặn | Tắt Windows Firewall tạm thời, hoặc mở port 4370/80 |
| `invalid or expired token` | Token hết hạn (24h) | Đăng nhập lại lấy token mới |
| `network unreachable` | Khác subnet | Máy tính và máy chấm công phải cùng /24 subnet |
| `HTTP 401` (Hikvision) | Sai password | Kiểm tra password admin trong device, reset nếu cần |
| `circuit breaker open` | Lỗi 5 lần liên tiếp | Đợi 30 giây, circuit tự half-open và thử lại |

---

## KIỂM TRA NHANH (Checklist)

```powershell
# Chạy script này để kiểm tra tất cả điều kiện trước khi test:
Write-Host "=== CHECKLIST ===" -ForegroundColor Cyan

# 1. Server chạy?
try {
    Invoke-RestMethod -Uri "http://localhost:8080/healthz" | Out-Null
    Write-Host "OK  Server dang chay" -ForegroundColor Green
} catch {
    Write-Host "FAIL Server CHUA chay - hay chay: go run ./cmd/server" -ForegroundColor Red
}

# 2. Login OK?
try {
    $r = Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/auth/login" `
      -ContentType "application/json" -Body '{"username":"admin","password":"admin123"}'
    Write-Host "OK  Login thanh cong" -ForegroundColor Green
    $global:testToken = $r.token
} catch {
    Write-Host "FAIL Login that bai" -ForegroundColor Red
}

# 3. Ping máy chấm công?
$devices = @(
    @{name="ZKTeco"; ip="192.168.1.100"},   # << sua IP nay
    @{name="Hikvision"; ip="192.168.1.200"}, # << sua IP nay
    @{name="Sunbeam"; ip="192.168.1.150"}    # << sua IP nay
)
foreach ($d in $devices) {
    $ping = Test-Connection -ComputerName $d.ip -Count 1 -Quiet
    if ($ping) {
        Write-Host "OK  $($d.name) ($($d.ip)) - Ping duoc" -ForegroundColor Green
    } else {
        Write-Host "WARN $($d.name) ($($d.ip)) - Khong ping duoc (co the chua bat may)" -ForegroundColor Yellow
    }
}
```
