# Hướng dẫn kết nối và test với thiết bị chấm công thực tế

## 1. Mục tiêu

Tài liệu này hướng dẫn cách:
- khởi động hệ thống attendance system;
- kết nối với thiết bị chấm công thật;
- test kết nối;
- đồng bộ nhân viên và log chấm công từ thiết bị vào hệ thống.

---

## 2. Yêu cầu trước khi test

### 2.1. Môi trường
- Máy tính chạy server và thiết bị chấm công phải cùng mạng LAN.
- Đảm bảo thiết bị chấm công đang bật.
- Biết đúng IP của thiết bị.
- Có thể ping được thiết bị từ máy tính.

### 2.2. Cài đặt
- Go đã được cài đặt.
- PostgreSQL đang chạy.
- Database `attendance_db` đã tồn tại.
- File `config.yaml` đã được cấu hình đúng.

---

## 3. Khởi động hệ thống

Mở terminal tại thư mục dự án:

```powershell
cd E:\Project\attendance-system
go run ./cmd/server
```

Nếu thành công, bạn sẽ thấy dòng tương tự:

```text
server starting {"port": 8085}
```

Kiểm tra nhanh server:

```powershell
Invoke-RestMethod -Uri "http://localhost:8085/healthz"
```

Kết quả mong đợi:

```json
{"status":"ok"}
```

---

## 4. Đăng nhập lấy token

```powershell
$BASE_URL = "http://localhost:8085/api/v1"

$response = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/auth/login" `
  -ContentType "application/json" `
  -Body '{"username":"admin","password":"admin123"}'

$token = $response.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Token OK"
```

---

## 5. Thêm thiết bị vào hệ thống

### 5.1. Ví dụ cho ZKTeco

```powershell
$deviceIP = "192.168.11.151"   # đổi thành IP thật của thiết bị

$body = @{
  name = "ZKTeco Test"
  device_type = "zkteco"
  ip_address = $deviceIP
  port = 4370
  serial_number = "ZK-001"
  location = "Tầng 1"
} | ConvertTo-Json

$device = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" `
  -Headers $headers `
  -Body $body

$device.id
```

### 5.2. Ví dụ cho Hikvision

```powershell
$deviceIP = "192.168.11.151"

$body = @{
  name = "Hikvision Test"
  device_type = "hikvision"
  ip_address = $deviceIP
  port = 80
  serial_number = "HIK-001"
  location = "Cổng chính"
} | ConvertTo-Json

$device = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" `
  -Headers $headers `
  -Body $body

$device.id
```

### 5.3. Ví dụ cho Sunbeam / Timmy

```powershell
$deviceIP = "192.168.1.150"

$body = @{
  name = "Sunbeam Test"
  device_type = "sunbeam"
  ip_address = $deviceIP
  port = 80
  serial_number = "SB-001"
  location = "Lối vào"
} | ConvertTo-Json

$device = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices" `
  -ContentType "application/json" `
  -Headers $headers `
  -Body $body

$device.id
```

---

## 6. Test kết nối thiết bị

```powershell
$deviceId = $device.id

Invoke-RestMethod -Method Get `
  -Uri "$BASE_URL/devices/$deviceId/status" `
  -Headers $headers
```

### Nếu kết nối thành công
Bạn sẽ nhận được dữ liệu trạng thái thiết bị, thường có thông tin như:
- online: true/false
- firmware_info
- user_count
- log_count

### Nếu kết nối thất bại
Hãy kiểm tra các điểm sau:
- IP đúng không?
- Thiết bị đang bật không?
- Thiết bị và máy tính cùng mạng LAN không?
- Port mở đúng không?
- Firewall/antivirus có chặn kết nối không?
- Username/password có đúng nếu thiết bị yêu cầu xác thực?

---

## 7. Tạo nhân viên và đồng bộ xuống thiết bị

```powershell
$emp = Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/employees" `
  -ContentType "application/json" `
  -Headers $headers `
  -Body '{
    "employee_code": "001",
    "full_name": "Nguyen Van A",
    "card_no": "0001234567"
  }'

$emp
```

Đồng bộ nhân viên xuống thiết bị:

```powershell
Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices/$deviceId/sync-employees" `
  -Headers $headers
```

---

## 8. Đồng bộ log chấm công từ thiết bị

```powershell
Invoke-RestMethod -Method Post `
  -Uri "$BASE_URL/devices/$deviceId/sync-attendance" `
  -Headers $headers
```

---

## 9. Kiểm tra kết quả

### 9.1. Qua web UI
Mở trình duyệt và truy cập:

```text
http://localhost:8085/
```

### 9.2. Qua API
- Danh sách chấm công:

```powershell
Invoke-RestMethod -Method Get `
  -Uri "$BASE_URL/attendance-logs" `
  -Headers $headers
```

- Lịch sử đồng bộ:

```powershell
Invoke-RestMethod -Method Get `
  -Uri "$BASE_URL/sync-history" `
  -Headers $headers
```

---

## 10. Gợi ý cho từng loại thiết bị

### ZKTeco
- thường dùng port `4370`;
- kiểm tra bằng ping và test connection;
- nếu không được, kiểm tra cấu hình mạng trên thiết bị.

### Hikvision
- thường dùng port `80`;
- có thể cần username/password đúng;
- kiểm tra endpoint truy cập từ trình duyệt trước.

### Sunbeam / Timmy
- thường dùng HTTP/REST;
- kiểm tra bằng cách truy cập IP thiết bị từ mạng nội bộ;
- nếu thiết bị không mở API, việc sync có thể bị lỗi.

---

## 11. Lưu ý quan trọng

- Hệ thống hiện hỗ trợ quản lý thiết bị, test kết nối và đồng bộ dữ liệu cơ bản.
- Với một số thiết bị thật, thao tác như push nhân viên hoặc reboot có thể cần SDK chính hãng hoặc cấu hình bổ sung.
- Nên bắt đầu bằng bước test kết nối trước, rồi mới thử đồng bộ dữ liệu.

---

## 12. Nếu cần hỗ trợ

Bạn có thể gửi cho tôi:
- IP của thiết bị;
- loại thiết bị (ZKTeco, Hikvision, Sunbeam);
- lỗi trả về từ API;
- screenshot màn hình thiết bị hoặc cấu hình mạng.

Tôi sẽ hướng dẫn bạn từng bước để test trên thiết bị thật.

---

## 13. Hướng dẫn cấu hình ADMS (ZKTeco Push Protocol)

Để thiết bị tự động đẩy dữ liệu và nhận lệnh từ Server thông qua giao thức ADMS (Push SDK), hãy thực hiện cấu hình trực tiếp trên màn hình máy chấm công và trên trang Web như sau:

### 13.1. Cấu hình trên Máy chấm công (ZKTeco RJ800)
Vào **Menu chính** -> **Thiết lập liên kết** (hoặc **Mạng**) -> **Cài đặt máy chủ đám mây** (Cloud Server Settings):

1. **Kiểu máy chủ**: Chọn **Tự động tải dữ liệu** (hoặc **ADMS** / **Web server**).
2. **Khởi động tên miền**: Chọn **OFF** (sử dụng IP trực tiếp).
3. **Địa chỉ máy chủ**: Điền chính xác IP của máy tính chạy server: `192.168.11.122`.
4. **Cổng máy chủ**: Điền cổng HTTP của server: `8085`.
5. **Cho phép máy chủ ủy nhiệm**: Chọn **OFF**.
6. **HTTPS**: Chuyển sang **OFF** (Tắt HTTPS).
   > [!IMPORTANT]
   > Hiện tại trên máy của bạn đang bật **HTTPS = ON**. Do server chạy trên cổng HTTP thường (`http`), bạn **BẮT BUỘC phải gạt HTTPS sang OFF** thì máy chấm công mới kết nối và gửi dữ liệu về server được.

Sau khi cài đặt xong, hãy khởi động lại máy chấm công hoặc nhấn lưu để thiết bị áp dụng cấu hình mới.

### 13.2. Cấu hình trên Web Dashboard
1. Truy cập vào trang Web: `http://localhost:8085` (Đăng nhập `admin` / `admin123`).
2. Vào tab **Thiết bị** -> Tìm thiết bị **ZKTeco Phong Chinh** (hoặc tạo mới thiết bị với IP `192.168.11.151`).
3. Điền các thông tin sau:
   - **Số serial ADMS (Push)**: Điền chính xác số Serial Number từ tem dán đằng sau máy: `8116255100515`.
   - **Bật chế độ ADMS Push**: Tích chọn **Bật**.
4. Nhấn **Lưu**.

### 13.3. Kiểm tra kết nối ADMS
Khi máy chấm công kết nối thành công, bạn sẽ thấy:
- Trên Web: Cột **Trạng thái ADMS** của thiết bị sẽ chuyển sang badge màu xanh **🟢 ADMS Live**.
- Server Console sẽ in log nhận kết nối từ thiết bị:
  ```text
  [ADMS] Cdata requested: SN=8116255100515 method=GET ...
  ```
- Khi quét vân tay trên máy, dữ liệu sẽ tự động đẩy về hệ thống và cập nhật thời gian thực qua cơ chế SSE (không cần refresh trang).

