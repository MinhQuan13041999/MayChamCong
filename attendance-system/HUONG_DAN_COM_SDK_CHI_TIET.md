# Hướng Dẫn Chi Tiết: COM SDK - Kéy & Đẩy Dữ Liệu Nhân Viên và Vân Tay

## 📌 Tổng Quan COM SDK

COM SDK cho phép bạn:
- ✅ **Kéy danh sách nhân viên** từ máy chấm công
- ✅ **Kéy trạng thái vân tay** (xem ai đã quét, ai chưa)
- ✅ **Đẩy nhân viên mới** xuống máy
- ✅ **Đẩy vân tay** xuống máy
- ✅ **Quét vân tay trực tiếp** từ web
- ✅ **Kéy dữ liệu lịch sử cũ** từ máy
- ⚠️ **Chỉ chạy trên Windows** (yêu cầu DLL)

---

## 🔧 PHẦN 1: CÀI ĐẶT SDK TRÊN WINDOWS

### Bước 1: Tải ZKemKeeper SDK

**Cách 1: Tải từ trang chính thức ZKTeco**
1. Truy cập: https://www.zkteco.com/en/download/
2. Tìm mục **"Software & SDK"**
3. Tìm **"ZKemkeeper SDK"** hoặc **"ZKemkeeper Setup"**
   - Phiên bản: v6.x hoặc v7.x
   - Loại: Windows
4. Nhấp **Download** (có thể cần đăng ký tài khoản)
5. Lưu file về (thường là `ZKemkeeper_Setup_v6.x.exe` hoặc `.msi`)

**Cách 2: Nếu bạn đã có DLL sẵn**
- Nếu máy chấm công đi kèm theo USB SDK
- Hoặc bạn có file `zkemkeeper.dll` sẵn rồi

### Bước 2: Cài Đặt SDK

#### Cách A: Cài Từ File Installer

1. **Mở File Installer**
   - Nhấp đúp vào `ZKemkeeper_Setup_v6.x.exe` (hoặc `.msi`)

2. **Làm theo Wizard**
   - Chọn **Next** → chấp nhận điều khoản
   - Chọn đường dẫn cài đặt (mặc định: `C:\Program Files\ZKTeco\ZKemkeeper\`)
   - Nhấn **Install**
   - Đợi cài đặt hoàn tất (2-3 phút)

3. **Xác Nhận Cài Đặt Thành Công**
   - Thư mục `C:\Program Files\ZKTeco\ZKemkeeper\` đã tồn tại
   - Có file `zkemkeeper.dll` bên trong

#### Cách B: Cài Thủ Công (Nếu Chỉ Có DLL)

Nếu bạn chỉ có file `zkemkeeper.dll`:
1. Tạo thư mục: `C:\Program Files\ZKTeco\ZKemkeeper\` (nếu chưa có)
2. Copy file `zkemkeeper.dll` vào thư mục này
3. Bước tiếp theo: Đăng ký DLL

### Bước 3: Đăng Ký DLL (Quan Trọng!)

**⚠️ Bước này bắt buộc! Phải chạy với quyền Admin**

#### Trên Windows 10/11:

1. **Mở PowerShell với Quyền Admin**
   - Nhấn `Win + X` → chọn **Windows PowerShell (Admin)**
   - Hoặc: tìm "PowerShell" → chuột phải → "Run as administrator"

2. **Chạy Lệnh Đăng Ký DLL**
   ```powershell
   regsvr32 "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"
   ```

3. **Chờ Kết Quả**
   - Nếu thành công: cửa sổ thông báo `"DllRegisterServer in ... succeeded"`
   - Nhấn **OK**

4. **Nếu Lỗi**
   - Kiểm tra đường dẫn có đúng không
   - Kiểm tra file `zkemkeeper.dll` có tồn tại không
   - Kiểm tra quyền Admin có không
   - Thử cài lại SDK

#### Trên Command Prompt (CMD):
```cmd
regsvr32 "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"
```

#### Trên Git Bash hoặc Terminal Linux Subsystem:
```bash
# Không hỗ trợ trực tiếp, phải dùng PowerShell/CMD trên Windows
```

### Bước 4: Test Đăng Ký DLL

Chạy PowerShell script này để kiểm tra DLL có được đăng ký không:

```powershell
# Test COM SDK
try {
    $obj = New-Object -ComObject zkemkeeper.ZKEM.1
    Write-Host "✅ COM SDK đã được đăng ký thành công!" -ForegroundColor Green
    $version = ""
    if ($obj.GetSDKVersion([ref]$version)) {
        Write-Host "Phiên bản: $version" -ForegroundColor Green
    } else {
        Write-Host "⚠️ Không thể lấy thông tin phiên bản" -ForegroundColor Yellow
    }
} catch {
    Write-Host "❌ Lỗi: COM SDK chưa được đăng ký hoặc không tìm thấy" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
}
```

**Kết quả mong đợi:**
```
✅ COM SDK đã được đăng ký thành công!
Phiên bản: 6.x hoặc 7.x
```

---

## 📡 PHẦN 2: KẾT NỐI MÁY CHẤM CÔNG VÀO MẠNG

### Bước 1: Kiểm Tra IP Máy Chấm Công

Có 3 cách tìm IP:

#### Cách 1: Từ Màn Hình Máy
1. Mở Settings trên máy chấm công (nhấn nút Settings/Menu)
2. Vào **Network** → **TCP/IP**
3. Xem mục **IP Address**
4. Ghi lại (ví dụ: `192.168.11.151`)

#### Cách 2: Từ Router WiFi
1. Mở trang quản lý router (192.168.11.1 hoặc 192.168.1.1)
2. Tìm mục **Connected Devices** hoặc **DHCP Clients**
3. Tìm tên máy ZKTeco
4. Xem IP của nó

#### Cách 3: Scan Mạng Từ Máy Tính
```powershell
# Trong PowerShell, chạy lệnh này:
for ($i=1; $i -le 254; $i++) {
    $ip = "192.168.11.$i"
    if (Test-Connection -ComputerName $ip -Count 1 -ErrorAction SilentlyContinue) {
        Write-Host "Máy online: $ip"
    }
}
```

### Bước 2: Ping Kiểm Tra Kết Nối

```powershell
ping 192.168.11.151
```

**Kết quả mong đợi:**
```
Reply from 192.168.11.151: bytes=32 time=2ms TTL=64
```

Nếu không trả lời → máy chưa kết nối mạng hoặc IP sai.

### Bước 3: Test Port 4370

```powershell
Test-NetConnection -ComputerName 192.168.11.151 -Port 4370
```

**Kết quả mong đợi:**
```
TcpTestSucceeded : True
```

Nếu `False` → port 4370 đóng (kiểm tra cấu hình máy)

---

## 🌐 PHẦN 3: CẤU HÌNH TRONG WEB UI

### Bước 1: Mở Web Interface

1. Mở trình duyệt
2. Truy cập: `http://localhost:8085/`
3. Đăng nhập:
   - Username: `admin`
   - Password: `admin123`

### Bước 2: Thêm Thiết Bị

1. Vào tab **🖥 Thiết Bị**
2. Nhấn **➕ Thêm thiết bị**
3. Điền thông tin:

| Trường | Giá Trị Ví Dụ | Ghi Chú |
|--------|---------------|--------|
| Tên thiết bị | ZKTeco Phòng Chính | Tên tuỳ ý |
| Loại thiết bị | ZKTeco | Chọn từ dropdown |
| Địa chỉ IP | 192.168.11.151 | IP máy chấm công |
| Port | 4370 | Cổng mặc định ZKTeco |
| Số serial (SDK Pull) | ZK-239102 | Mã serial máy (tìm ở System Info) |
| Số serial ADMS | SN trên máy | Bắt buộc nếu muốn chạy ADMS song song với SDK |
| Firmware | v6.60 | Tuỳ ý (ghi chú) |
| Địa chỉ MAC | 00:1A:2B:3C:4D:5E | Tuỳ ý (ghi chú) |
| Vị trí đặt | Cửa chính, Tầng 1 | Ghi chú vị trí |

4. Chọn giao thức vận hành:
   - Chỉ dùng SDK: bỏ tích **Bật chế độ ADMS Push**.
   - Dùng song song ADMS + SDK: giữ tích ADMS; luồng ATTLOG qua SDK vẫn tự động
     chạy mỗi 10 giây và có thể gọi thủ công bằng nút **Lấy qua SDK**.

5. Nhấn **Lưu cấu hình**

### Bước 3: Tìm Mã Serial Máy (Nếu Chưa Biết)

Trên máy chấm công:
1. Mở **Settings** → **System Info**
2. Tìm **Device Serial Number** hoặc **SN**
3. Ghi lại (ví dụ: `ZK-239102`)
4. Nhập vào trường **"Số serial (SDK Pull)"** trong web

---

## ✅ PHẦN 4: TEST KẾT NỐI

### Bước 1: Kiểm Tra Trạng Thái Thiết Bị

1. Vào tab **🖥 Thiết Bị**
2. Tìm thiết bị vừa thêm
3. Nhấn nút **"⚙️"** hoặc **"Chi tiết"** hoặc **"Test"**
4. Hệ thống sẽ:
   - Kết nối tới máy qua COM SDK
   - Đọc thông tin firmware
   - Hiển thị trạng thái **Online** hoặc **Offline**

**Nếu thành công:**
```
✅ Trạng thái: Online
✅ Firmware: v6.60
✅ Tổng nhân viên: 45
✅ Tổng log công: 2348
```

**Nếu thất bại:**
```
❌ Lỗi: "Kết nối máy chấm công thất bại"
```

Hãy xem phần **Troubleshooting** ở dưới.

### Bước 2: Kiểm Tra Kết Nối Từ PowerShell

Chạy script này:
```powershell
$ip = "192.168.11.151"
$port = 4370

try {
    $zkem = New-Object -ComObject zkemkeeper.ZKEM.1
    
    Write-Host "Đang kết nối tới $ip`:$port ..."
    $connected = $zkem.Connect_Net($ip, $port)
    
    if ($connected) {
        Write-Host "✅ Kết nối thành công!" -ForegroundColor Green
        
        # Lấy thông tin firmware
        $fw = $zkem.GetFirmwareVersion()
        Write-Host "Firmware: $fw" -ForegroundColor Green
        
        # Lấy thông tin cấu hình
        $zkem.RefreshData(1)
        Write-Host "✅ Dữ liệu đã được tải" -ForegroundColor Green
        
        # Ngắt kết nối
        $zkem.Disconnect()
    } else {
        Write-Host "❌ Kết nối thất bại" -ForegroundColor Red
        $errorCode = $zkem.GetLastError()
        Write-Host "Mã lỗi: $errorCode" -ForegroundColor Red
    }
} catch {
    Write-Host "❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
}
```

---

## 👤 PHẦN 5: KÉY DANH SÁCH NHÂN VIÊN TỪ MÁY

### Bước 1: Kéy Nhân Viên Và Kiểm Tra Vân Tay

1. Vào tab **👤 Nhân viên**
2. Nhấn **📥 Kéo NV từ máy** (nút xanh)
3. Cửa sổ modal hiện lên:
   - **Chọn thiết bị nguồn**: chọn thiết bị vừa thêm
   - Nhấn **📥 Kéy dữ liệu**

4. Hệ thống sẽ:
   - Kết nối tới máy qua COM SDK
   - Đọc danh sách nhân viên
   - **Kiểm tra trạng thái vân tay** của từng người
   - Tạo nhân viên mới nếu chưa có trong hệ thống

### Bước 2: Xem Kết Quả

Sau khi hoàn tất:
```
✅ Danh sách nhân viên:
  - 👤 Nguyễn Văn A (Code: 001) - 🖐 Đã quét VT
  - 👤 Trần Thị B (Code: 002) - ⚠️ Chưa quét VT
  - 👤 Phạm Văn C (Code: 003) - 🖐 Đã quét VT
  ...

📊 Tóm tắt:
  - Tổng nhân viên: 45
  - Nhân viên mới: 5
  - Nhân viên đã có: 40
  - Đã quét vân tay: 38
  - Chưa quét vân tay: 7
```

### Bước 3: Kiểm Tra Chi Tiết

1. Vào tab **👤 Nhân viên**
2. Tìm nhân viên trong danh sách
3. Xem cột **"Vân tay"**:
   - 🖐 = Đã quét (có vân tay)
   - ⚠️ Chưa = Chưa quét (không có vân tay)

---

## 📤 PHẦN 6: ĐẨY NHÂN VIÊN XUỐNG MÁY

### Cách 1: Đẩy Nhân Viên Mới + Quét Vân Tay Ngay

**Bước 1: Thêm Nhân Viên Mới**

1. Vào tab **👤 Nhân viên**
2. Nhấn **➕ Thêm nhân viên**
3. Điền thông tin:

| Trường | Giá Trị Ví Dụ |
|--------|---------------|
| Mã NV | 046 |
| Họ tên | Lê Minh Đức |
| Chức danh | Lập trình viên |
| Phòng ban | Kỹ thuật |
| Email | duc@company.com |
| Số điện thoại | 0901234567 |
| Ngày sinh | 1995-05-15 |
| Ngày nhận việc | 2024-01-01 |

4. ✅ **Tích vào** "Đồng bộ và yêu cầu quét vân tay trên máy ngay"

5. Cửa sổ mở rộng hiện lên:
   - **Chọn thiết bị ADMS**: chọn thiết bị ZKTeco
   - **ID người dùng trên máy (PIN)**: nhập `46` (thường là mã NV cuối cùng + 1)

6. Nhấn **Lưu nhân viên**

**Bước 2: Máy Sẽ Hiển Thị Màn Hình Quét Vân Tay**

Trên màn hình máy chấm công:
```
┌─────────────────────────┐
│ Vui lòng quét vân tay   │
│ Ngón tay: 1/3           │
│                         │
│ Đặt ngón tay lên        │
│ lăng kính quét          │
└─────────────────────────┘
```

**Bước 3: Người Dùng Quét Vân Tay**

- Đặt ngón tay (chẳng hạn ngón trỏ) lên lăng kính quét
- Giữ 2-3 giây
- Nâng ngón tay lên
- **Lặp lại 3 lần** (máy sẽ yêu cầu)
- Máy hiển thị **"Thành công"**

**Bước 4: Xác Nhận Trong Web**

- Quay lại web
- Kiểm tra nhân viên vừa thêm
- Cột **"Vân tay"** sẽ hiển thị **🖐 (xanh)**

---

### Cách 2: Đẩy Nhiều Nhân Viên Cùng Lúc (Batch)

**Bước 1: Chọn Nhân Viên**

1. Vào tab **👤 Nhân viên**
2. Nhấn **🖐 Chọn để quét** (nút cam)
3. Cửa sổ modal hiện lên
4. **Chọn thiết bị**: chọn ZKTeco
5. **Chọn nhân viên**: tích vào danh sách các nhân viên chưa có vân tay
   - ☑️ Nguyễn Văn A
   - ☑️ Trần Thị B
   - ☑️ Phạm Văn C
6. Hoặc nhấn **"Chọn tất cả"** để chọn hàng loạt

**Bước 2: Bắt Đầu Quét**

1. Nhấn **🖐 Bắt đầu quét**
2. Hệ thống sẽ:
   - Đẩy thông tin nhân viên xuống máy
   - Gửi lệnh quét vân tay cho từng người
   - Máy sẽ hiển thị danh sách người cần quét

**Bước 3: Quét Vân Tay Lần Lượt**

Trên màn hình máy:
```
Danh sách quét vân tay:
1. Nguyễn Văn A (PIN: 1)
2. Trần Thị B (PIN: 2)
3. Phạm Văn C (PIN: 3)
```

- Nhân viên thứ 1 đặt vân tay
- Quét 3 lần
- Nhân viên thứ 2 thay thế
- Quét 3 lần
- ...cứ tiếp tục cho hết danh sách

**Bước 4: Dừng Quét (Nếu Cần)**

Vào tab **👤 Nhân viên** → nhấn **🛑 Dừng quét** để hủy bỏ lệnh quét hàng loạt.

---

### Cách 3: Đẩy Tất Cả Nhân Viên (Không Quét)

**Chỉ đẩy thông tin nhân viên, không quét vân tay:**

1. Vào tab **👤 Nhân viên**
2. Nhấn **📤 Đẩy tất cả xuống máy**
3. Chọn **thiết bị đích**
4. Nhấn **📤 Đẩy xuống máy**
5. Hệ thống sẽ:
   - Kết nối tới máy
   - Đẩy tất cả nhân viên **active** có trong hệ thống
   - Máy sẽ lưu thông tin

---

## 🖐️ PHẦN 7: QUÉT VÂN TAY RIÊNG LẺ

### Bước 1: Mở Cửa Sổ Quản Lý Vân Tay

1. Vào tab **👤 Nhân viên**
2. Tìm nhân viên cần quét vân tay
3. Nhấn nút **"🖐"** hoặc **"Vân tay"** trong hàng
4. Cửa sổ modal hiện lên

### Bước 2: Đăng Ký Vân Tay Mới

1. Chọn **thiết bị quét**: chọn ZKTeco
2. Nhấn **⚡ Bắt đầu quét vân tay**
3. Máy sẽ hiển thị:
   ```
   Vui lòng quét vân tay
   Ngón tay: 1/3
   ```

4. **Đặt ngón tay** (chẳng hạn ngón trỏ) lên lăng kính
5. **Lặp lại 3 lần**
6. Máy hiển thị **"Quét thành công"**

### Bước 3: Xem Vân Tay Đã Quét

Trong cửa sổ modal:
```
🖐️ Vân Tay Đã Quét:
  - Ngón 1 (Trỏ): ✅ Có
  - Ngón 2 (Giữa): ❌ Không
```

### Bước 4: Quét Thêm Hoặc Xoá Vân Tay

- **🔁 Đăng ký lại**: quét lại vân tay hiện tại (ghi đè)
- **🗑️ Xoá vân tay**: xoá vân tay khỏi hệ thống
- **🔄 Đồng bộ máy khác**: copy vân tay sang máy chấm công khác

---

## 📝 PHẦN 8: KÉY DỮ LIỆU CHẤM CÔNG

### Chạy song song ADMS và SDK

Thiết bị có thể tiếp tục bật **ADMS Push** để gửi log theo thời gian thực,
đồng thời hệ thống vẫn kết nối trực tiếp qua **SDK** để kéo ATTLOG mỗi 10 giây
và khi người dùng nhấn **Lấy qua SDK**. Hai luồng không tạo công trùng vì
database loại trùng theo thiết bị, mã nhân viên và thời gian quét.

Không cần tắt ADMS để dùng SDK lấy dữ liệu chấm công. Máy ZKTeco vẫn phải có
IP LAN truy cập được, cổng 4370 mở và ZKemKeeper đã đăng ký trên Windows.

### Bước 1: Kéy Log Công (Attendance Log)

1. Vào tab **📝 Chấm công**
2. Chọn **ngày**:
   - **Từ ngày**: 2024-07-01
   - **Đến ngày**: 2024-07-17
3. (Tuỳ chọn) Nhập **mã nhân viên** để lọc
4. Nhấn **Lấy qua SDK** để kết nối máy và kéo log mới. Nút **Áp dụng** chỉ lọc
   lại dữ liệu đã lưu theo khoảng ngày đã chọn.
5. Hệ thống sẽ:
   - Kết nối tới máy qua COM SDK
   - Kéy tất cả dữ liệu chấm công trong khoảng ngày
   - Lưu vào database
   - Hiển thị danh sách

### Bước 2: Xem Dữ Liệu Chấm Công

Bảng sẽ hiển thị:

| Nhân Viên | Thời Gian | Lần Quét | Thiết Bị |
|-----------|-----------|---------|---------|
| Nguyễn Văn A | 2024-07-17 08:15 | 1 (Vào) | ZKTeco Phòng Chính |
| Nguyễn Văn A | 2024-07-17 17:30 | 2 (Ra) | ZKTeco Phòng Chính |
| Trần Thị B | 2024-07-17 08:22 | 1 (Vào) | ZKTeco Phòng Chính |

### Bước 3: Tính Công

1. Nhấn **⚡ Tính công**
2. Hệ thống sẽ:
   - Phân tích giờ vào/ra
   - Tính đi muộn, về sớm
   - Tính tăng ca
   - Hiển thị tóm tắt

---

## 🆘 PHẦN 9: TROUBLESHOOTING - SỬA LỖI

### Lỗi 1: "zkemkeeper.ZKEM.1" Không Tìm Thấy

**Nguyên nhân:** DLL chưa đăng ký

**Giải pháp:**
```powershell
# 1. Kiểm tra đường dẫn
Test-Path "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"

# 2. Nếu không tìm thấy, cài lại SDK
# 3. Sau đó đăng ký DLL (quyền Admin)
regsvr32 "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"

# 4. Test lại
$obj = New-Object -ComObject zkemkeeper.ZKEM.1
Write-Host "✅ OK"
```

### Lỗi 2: "Kết Nối Máy Chấm Công Thất Bại"

**Nguyên nhân:** IP, port, hoặc mạng có vấn đề

**Giải pháp:**

```powershell
# 1. Kiểm tra mạng
ping 192.168.11.151

# 2. Kiểm tra port
Test-NetConnection -ComputerName 192.168.11.151 -Port 4370

# 3. Kiểm tra firewall
# Mở Windows Defender Firewall → Allow an app through firewall
# Thêm port 4370

# 4. Kiểm tra máy chấm công
# - Máy có bật không?
# - IP có đúng không?
# - Port 4370 có mở không?
# - TCP/IP có được bật không?
```

### Lỗi 3: "Timeout - Kết Nối Quá Lâu"

**Nguyên nhân:** Mạng chậm, máy chấm công không phản hồi

**Giải pháp:**
```powershell
# 1. Restart máy chấm công
# 2. Đợi 30 giây
# 3. Thử lại

# 4. Kiểm tra cấu hình mạng trên máy
# Settings → Network → TCP/IP
# - Xem có cài IP tĩnh không
# - Xem gateway có đúng không
```

### Lỗi 4: "Không Thể Quét Vân Tay"

**Nguyên nhân:** Máy không phản hồi lệnh quét

**Giải pháp:**
1. Kiểm tra máy có online không (nhấn nút bất kỳ)
2. Kiểm tra ID người dùng (PIN) có đúng không
3. Kiểm tra máy có hỗ trợ quét vân tay không
4. Thử restart máy

### Lỗi 5: "Mã Nhân Viên Đã Tồn Tại"

**Nguyên nhân:** Nhân viên này đã có trong hệ thống

**Giải pháp:**
1. Cập nhật thông tin nhân viên thay vì thêm mới
2. Hoặc xoá nhân viên cũ rồi thêm lại

---

## 📊 PHẦN 10: QUYẾT TRÌNH THỰC HIỆN TOÀN BỘ

### Quy Trình Hoàn Chỉnh

```
1️⃣ CÀI ĐẶT SDK
   ├─ Tải SDK từ ZKTeco
   ├─ Cài đặt
   └─ Đăng ký DLL (regsvr32)

2️⃣ KẾT NỐI MẠNG
   ├─ Tìm IP máy
   ├─ Ping kiểm tra
   └─ Test port 4370

3️⃣ CẤU HÌNH WEB
   ├─ Mở http://localhost:8085/
   ├─ Đăng nhập (admin/admin123)
   ├─ Thêm thiết bị
   └─ Test kết nối ✅

4️⃣ KÉY DỮ LIỆU TỪ MÁY
   ├─ 📥 Kéy nhân viên
   ├─ 🖐️ Kiểm tra vân tay
   └─ 📝 Kéy dữ liệu chấm công

5️⃣ ĐẨY DỮ LIỆU LÊN MÁY
   ├─ ➕ Thêm nhân viên mới
   ├─ 🖐️ Quét vân tay
   └─ 📤 Đẩy xuống máy

6️⃣ QUẢN LÝ HÀNG NGÀY
   ├─ Kiểm tra dữ liệu chấm công
   ├─ Tính công
   └─ Xuất báo cáo
```

---

## ✅ CHECKLIST HOÀN TẤT

```
☑️ Cài đặt SDK trên Windows
☑️ Đăng ký DLL thành công
☑️ Tìm được IP máy chấm công
☑️ Ping kết nối được máy
☑️ Test port 4370 thành công
☑️ Thêm thiết bị vào web
☑️ Test kết nối từ web thành công
☑️ Kéy danh sách nhân viên từ máy
☑️ Xem được trạng thái vân tay
☑️ Thêm nhân viên mới + quét vân tay
☑️ Kéy dữ liệu chấm công từ máy
☑️ Tính công thành công
```

---

## 📞 GỢI Ý VÀ MẸO

1. **Giữ SDK phiên bản mới nhất** - Tải từ trang chính thức
2. **Backup DLL file** - Lưu copy để dễ tái cài
3. **Kiểm tra IP định kỳ** - Nếu dùng DHCP, IP có thể thay đổi
4. **Bật log chi tiết** - Giúp debug lỗi dễ hơn
5. **Test kết nối mỗi tuần** - Đảm bảo hệ thống hoạt động

---

## 📚 TÀI LIỆU THAM KHẢO

- **ZKTeco SDK**: https://www.zkteco.com/en/download/
- **Hướng dẫn SDK**: File README trong thư mục SDK
- **Màn hình máy**: Xem hướng dẫn trên màn hình máy chấm công
- **Hỗ trợ**: Liên hệ nhà cung cấp ZKTeco

---

**🎉 Bạn đã hoàn thành cấu hình COM SDK! Hệ thống sẵn sàng kéy & đẩy dữ liệu.**
