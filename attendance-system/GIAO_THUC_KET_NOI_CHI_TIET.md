# Hướng dẫn Chi Tiết: 3 Giao Thức Kết Nối Thiết Bị Chấm Công

Hệ thống Attendance hỗ trợ **3 giao thức chính** để kết nối với thiết bị ZKTeco. Mỗi giao thức có ưu nhược điểm khác nhau.

---

## Tóm Tắt So Sánh 3 Giao Thức

| Giao Thức | Hướng Dữ Liệu | Chạy Trên | Độ Trễ | Ưu Điểm | Nhược Điểm |
|-----------|---------------|----------|-------|--------|-----------|
| **ADMS Push** | Máy → Web (tự động) | Bất kỳ | Thấp (real-time) | Không cần máy tính chạy liên tục, máy tự gửi | Cần cấu hình máy chấm công, không kéo được dữ liệu cũ |
| **COM SDK** | Web ← Máy (chủ động kéo) | Windows + DLL | Cao (phải cấu hình) | Đẩy nhân viên & vân tay trực tiếp | Yêu cầu Windows, cần đăng ký DLL, chỉ chạy trên máy tính có SDK |
| **TCP/IP** | Web ← Máy (chủ động kéo) | Bất kỳ | Cao | Đa nền tảng, không cần DLL đặc biệt | Phải cấu hình lệnh TCP/IP trên máy |

---

## 1️⃣ ADMS PUSH PROTOCOL - Máy Chủ Động Gửi Dữ Liệu

### 📋 Mô Tả
- **Máy chấm công tự động push dữ liệu** định kỳ tới web server
- Máy sẽ gọi HTTP POST tới `http://your-server:8085/iclock/cdata`
- Web server lưu dữ liệu công chấm vào database
- Máy cũng có thể kéo lệnh từ server thông qua `GET /iclock/getrequest`

### 🔧 Cách Cấu Hình Máy Chấm Công (ZKTeco)

#### Bước 1: Kết Nối Máy vào Mạng LAN
1. Chạm vào Settings / Advanced Settings trên máy
2. Chọn **Network** → **TCP/IP**
3. Nhập địa chỉ IP tĩnh (ví dụ: `192.168.11.151`) hoặc DHCP
4. Nhập IP Gateway (ví dụ: `192.168.11.1`)
5. **Lưu lại**, máy sẽ khởi động lại

#### Bước 2: Cấu Hình ADMS Server
Trên màn hình máy chấm công:
1. Vào **Settings** → **System Setup** → **ADMS**
2. Cấu hình **Server Address / Host**: Nhập IP hoặc tên miền của server
   - Ví dụ: `192.168.11.122:8085` hoặc `attendance.company.local:8085`
3. Cấu hình **Port**: `8085`
4. Cấu hình **Device Serial (SN)**: Nhập mã serial của máy (thường có sẵn)
   - Có thể xem ở **System Info** trên máy
5. Bật chế độ **ADMS Push**
6. **Lưu và khởi động lại máy**

#### Bước 3: Xác Nhận Trong Web UI
1. Mở http://localhost:8085/
2. Đăng nhập với `admin` / `admin123`
3. Vào tab **🖥 Thiết Bị**
4. Nhấp **➕ Thêm thiết bị**
5. Điền thông tin:
   - **Tên thiết bị**: ZKTeco Phòng Chính
   - **Loại thiết bị**: ZKTeco
   - **Địa chỉ IP**: 192.168.11.151
   - **Port**: 4370
   - **Số serial (SDK Pull)**: Bỏ trống (ADMS không dùng)
   - **Số serial ADMS (Push)**: `ZK-239102` (mã serial máy)
   - ✅ **Bật chế độ ADMS Push** (máy chấm công tự push dữ liệu)
6. Nhấn **Lưu cấu hình**

#### Bước 4: Kiểm Tra Kết Nối
Máy sẽ tự động push dữ liệu mỗi 10-30 giây. Bạn sẽ thấy:
- Dữ liệu chấm công tự động xuất hiện trong tab **📝 Chấm công**
- Lịch sử sync được cập nhật trong tab **🔄 Đồng bộ**
- Không cần bất cứ hành động gì từ web (hoàn toàn tự động)

### ✅ Ưu Điểm ADMS
- Máy tự gửi dữ liệu → không cần máy tính chạy 24/7 kéo dữ liệu
- Real-time: dữ liệu cập nhật nhanh
- Đơn giản: cấu hình máy 1 lần xong là chạy tự động
- An toàn: dữ liệu được ghi nhận khi máy gửi

### ❌ Nhược Điểm ADMS
- Chỉ gửi dữ liệu mới từ khi bật ADMS
- Không thể kéo dữ liệu lịch sử cũ
- Cần phải cấu hình trực tiếp trên máy (không qua web)
- Nếu máy chủ down khi máy gửi, có thể mất dữ liệu

---

## 2️⃣ COM SDK - Kéo Dữ Liệu Qua SDK Chính Hãng (Windows)

### 📋 Mô Tả
- **Máy tính (Windows) chủ động kéo dữ liệu** từ máy chấm công
- Sử dụng **ZKemKeeper SDK** (file DLL của ZKTeco)
- Có khả năng **đẩy nhân viên & vân tay** trực tiếp xuống máy
- Chỉ chạy trên Windows, yêu cầu DLL được đăng ký

### 🔧 Cách Cấu Hình

#### Bước 1: Cài Đặt SDK Trên Windows
1. **Tải ZKemKeeper SDK** từ trang chính thức ZKTeco
   - Link: https://www.zkteco.com/en/download/
   - Tìm bản "ZKemkeeper SDK" hoặc "ZKemkeeper Setup"
2. **Cài đặt SDK**
   - Chạy installer, chọn Install
   - Mặc định cài vào `C:\Program Files\ZKTeco\ZKemkeeper\`
3. **Đăng ký DLL** (chạy lệnh với quyền Admin)
   ```powershell
   regsvr32 "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"
   ```
   - Nếu không có đường dẫn này, tìm file `zkemkeeper.dll` trong thư mục cài đặt
   - Thành công khi thấy: "DllRegisterServer in ... succeeded"

#### Bước 2: Kết Nối Máy Chấm Công Vào Mạng
Xem phần **Bước 1** trong **ADMS Push Protocol** ở trên

#### Bước 3: Cấu Hình Trong Web UI
1. Mở http://localhost:8085/
2. Đăng nhập
3. Vào tab **🖥 Thiết Bị** → **➕ Thêm thiết bị**
4. Điền thông tin:
   - **Tên thiết bị**: ZKTeco Phòng Chính
   - **Loại thiết bị**: ZKTeco
   - **Địa chỉ IP**: 192.168.11.151
   - **Port**: 4370
   - **Số serial (SDK Pull)**: `ZK-239102` (mã serial máy)
   - **Số serial ADMS (Push)**: Bỏ trống (không dùng ADMS)
   - ❌ **Tắt chế độ ADMS Push** (chúng ta sẽ dùng SDK kéy dữ liệu)
5. Nhấn **Lưu cấu hình**

#### Bước 4: Kéo Dữ Liệu Từ Máy
**Kéo nhân viên từ máy:**
1. Vào tab **👤 Nhân viên**
2. Nhấn **📥 Kéo NV từ máy**
3. Chọn thiết bị
4. Nhấn **📥 Kéo dữ liệu**
5. Hệ thống sẽ:
   - Kết nối tới máy qua SDK
   - Đọc danh sách nhân viên
   - Đọc trạng thái vân tay của mỗi người
   - Tạo nhân viên mới nếu chưa có
   - Hiển thị kết quả

**Kéo dữ liệu chấm công:**
1. Vào tab **📝 Chấm công**
2. Nhấn **Lấy dữ liệu** (hoặc chọn ngày)
3. Nhấn **Áp dụng**
4. Dữ liệu sẽ được kéy từ máy

#### Bước 5: Đẩy Nhân Viên Xuống Máy
**Đẩy 1 nhân viên mới & quét vân tay:**
1. Vào tab **👤 Nhân viên**
2. Nhấn **➕ Thêm nhân viên**
3. Điền thông tin:
   - Mã NV: 001
   - Họ tên: Nguyễn Văn A
   - Phòng ban: Kỹ thuật
   - ...
4. ✅ Tích **Đồng bộ và yêu cầu quét vân tay trên máy ngay**
5. Chọn thiết bị & ID người dùng (PIN): 1
6. Nhấn **Lưu nhân viên**
7. Máy sẽ hiển thị **"Vui lòng quét vân tay"** → đặt ngón tay 3 lần

**Đẩy nhiều nhân viên cùng lúc:**
1. Vào tab **👤 Nhân viên**
2. Nhấn **📤 Đẩy tất cả xuống máy**
3. Chọn thiết bị đích
4. Nhấn **📤 Đẩy xuống máy**
5. Hệ thống sẽ đẩy tất cả nhân viên active

### ✅ Ưu Điểm COM SDK
- Có thể **đẩy nhân viên & vân tay** trực tiếp xuống máy
- Có thể **kéy dữ liệu lịch sử cũ** từ máy
- Kết nối nhanh, hiệu suất cao
- Hỗ trợ quét vân tay trực tiếp trên máy từ web

### ❌ Nhược Điểm COM SDK
- ⚠️ **Chỉ chạy trên Windows** (không hỗ trợ Mac, Linux)
- Yêu cầu cài đặt SDK + đăng ký DLL
- Phải chạy từ máy tính có SDK cài sẵn
- Nếu máy tính tắt, không kéy được dữ liệu
- Hỗ trợ 1 kết nối tại 1 thời điểm (lock OS thread)

---

## 3️⃣ TCP/IP PROTOCOL - Kéo Dữ Liệu Qua TCP/IP Thuần (Đa Nền Tảng)

### 📋 Mô Tả
- **Máy tính chủ động kéy dữ liệu** từ máy chấm công thông qua **TCP/IP trực tiếp**
- Sử dụng thư viện **gozk** (thuần Go, không cần DLL)
- Hoạt động trên **bất kỳ hệ điều hành nào** (Windows, Linux, macOS)
- Không cần cài SDK đặc biệt

### 🔧 Cách Cấu Hình

#### Bước 1: Kết Nối Máy Chấm Công Vào Mạng
Xem phần **Bước 1** trong **ADMS Push Protocol** ở trên

#### Bước 2: Cấu Hình Trong Web UI
1. Mở http://localhost:8085/
2. Đăng nhập
3. Vào tab **🖥 Thiết Bị** → **➕ Thêm thiết bị**
4. Điền thông tin:
   - **Tên thiết bị**: ZKTeco Phòng Chính
   - **Loại thiết bị**: ZKTeco
   - **Địa chỉ IP**: 192.168.11.151
   - **Port**: 4370 (cổng mặc định)
   - **Số serial (SDK Pull)**: Bỏ trống hoặc để trống
   - ❌ **Tắt chế độ ADMS Push**
5. Nhấn **Lưu cấu hình**

#### Bước 3: Test Kết Nối TCP/IP
1. Vào tab **🖥 Thiết Bị**
2. Tìm thiết bị vừa thêm
3. Nhấn nút **"⚙️ Chi tiết"** hoặc **"Test"**
4. Hệ thống sẽ:
   - Kết nối tới máy qua TCP/IP port 4370
   - Đọc firmware version
   - Hiển thị trạng thái online/offline
   - Báo số nhân viên & log trên máy

#### Bước 4: Kéo Dữ Liệu Từ Máy
**Kéo nhân viên từ máy:**
1. Vào tab **👤 Nhân viên**
2. Nhấn **📥 Kéo NV từ máy**
3. Chọn thiết bị
4. Nhấn **📥 Kéo dữ liệu**
5. Hệ thống sẽ:
   - Kết nối qua TCP/IP
   - Đọc danh sách nhân viên
   - ⚠️ Không thể đọc vân tay (TCP/IP không hỗ trợ)
   - Tạo nhân viên mới

**Kéo dữ liệu chấm công:**
1. Vào tab **📝 Chấm công**
2. Chọn ngày (ví dụ: từ hôm nay)
3. Nhấn **Áp dụng**
4. Dữ liệu sẽ được kéy từ máy

#### Bước 5: Lưu Ý TCP/IP
- TCP/IP **không hỗ trợ**:
  - ❌ Đẩy nhân viên xuống máy
  - ❌ Đẩy vân tay xuống máy
  - ❌ Quét vân tay từ xa
  - ❌ Gửi lệnh điều khiển khác
- ✅ Chỉ hỗ trợ:
  - ✅ Kéy danh sách nhân viên
  - ✅ Kéy dữ liệu chấm công
  - ✅ Kiểm tra trạng thái máy
  - ✅ Đồng bộ thời gian

### ✅ Ưu Điểm TCP/IP
- Đa nền tảng: chạy trên Windows, Linux, macOS
- Không cần cài SDK hay DLL
- Đơn giản, không phụ thuộc vào COM
- Có thể kéy dữ liệu lịch sử

### ❌ Nhược Điểm TCP/IP
- Không thể đẩy nhân viên hay vân tay xuống máy
- Không thể quét vân tay từ xa
- Hiệu suất có thể thấp hơn COM SDK

---

## 📊 Bảng So Sánh Chi Tiết

### Khả Năng Kéy Dữ Liệu
| Giao Thức | Kéy Nhân Viên | Kéy Vân Tay | Kéy Log Công |
|-----------|:---:|:---:|:---:|
| ADMS Push | ❌ Không | ❌ Không | ✅ Có |
| COM SDK | ✅ Có | ✅ Có | ✅ Có |
| TCP/IP | ✅ Có | ❌ Không | ✅ Có |

### Khả Năng Đẩy Dữ Liệu
| Giao Thức | Đẩy Nhân Viên | Đẩy Vân Tay | Quét VT Từ Xa |
|-----------|:---:|:---:|:---:|
| ADMS Push | ❌ Không | ❌ Không | ✅ Có (máy nhận lệnh) |
| COM SDK | ✅ Có | ✅ Có | ✅ Có |
| TCP/IP | ❌ Không | ❌ Không | ❌ Không |

> **💡 Lưu ý về Đăng ký Vân tay (Cập nhật mới):**
> Hệ thống hỗ trợ đăng ký và quản lý đồng thời lên tới **10 dấu vân tay độc lập (chỉ số ngón từ 0 đến 9)** cho mỗi nhân viên.
> - **Đối với ADMS Push**: Khi người dùng chọn một ngón tay cụ thể trên giao diện Web và nhấn "Bắt đầu quét", server sẽ phát hành lệnh đăng ký với mã ngón (`FID`) tương ứng xuống thiết bị. Khi thiết bị gửi kết quả quét về, server tự động giải mã và lưu đúng ngón đó.
> - **Đối với COM SDK**: Hệ thống truyền chỉ số ngón (`fingerIndex`) trực tiếp vào hàm SDK để kích hoạt phiên quét chính xác trên thiết bị Windows.

---

## 🎯 Khuyến Nghị Sử Dụng

### Trường Hợp 1: Máy Chấm Công Ổn Định, Muốn Tự Động
→ **Dùng ADMS Push**
- Cấu hình máy 1 lần
- Máy tự động gửi dữ liệu
- Không cần máy tính chạy 24/7

### Trường Hợp 2: Cần Kiểm Soát Toàn Bộ, Chạy Trên Windows
→ **Dùng COM SDK**
- Có thể kéy/đẩy dữ liệu linh hoạt
- Quét vân tay trực tiếp từ web
- Yêu cầu máy tính Windows chạy thường xuyên

### Trường Hợp 3: Chạy Trên Server Linux/Không Có Windows
→ **Dùng TCP/IP**
- Không cần SDK
- Đa nền tảng
- Kéy được nhân viên & log công
- Nhưng không thể đẩy dữ liệu hay quét vân tay

### Trường Hợp 4: Hỗ Trợ Cả 3 (Tối Ưu)
→ **Kết Hợp Cả 3 Giao Thức**
- ADMS Push: máy tự gửi log công real-time
- COM SDK: quản lý nhân viên & vân tay từ Windows
- TCP/IP: backup kéy dữ liệu từ Linux server

---

## 🛠️ Troubleshooting

### ❌ ADMS Push: Máy Không Gửi Dữ Liệu

**Kiểm tra:**
```
1. Ping máy từ máy tính: ping 192.168.11.151
2. Xem terminal server: tail -f /path/to/logs
3. Kiểm tra tab "🔄 Đồng bộ": có request từ máy không?
```

**Giải pháp:**
- Kiểm tra IP, port cấu hình trên máy
- Kiểm tra firewall cho phép port 8085
- Kiểm tra serial number (SN) có đúng không

### ❌ COM SDK: Lỗi "zkemkeeper.ZKEM.1"

**Nguyên nhân:** DLL chưa đăng ký

**Giải pháp:**
```powershell
# Chạy với quyền Admin
regsvr32 "C:\Program Files\ZKTeco\ZKemkeeper\zkemkeeper.dll"
```

### ❌ TCP/IP: Kết Nối Timeout

**Kiểm tra:**
```powershell
# Ping máy
ping 192.168.11.151

# Test port 4370
Test-NetConnection -ComputerName 192.168.11.151 -Port 4370
```

**Giải pháp:**
- Kiểm tra máy có bật không
- Kiểm tra IP đúng không
- Kiểm tra port 4370 mở không

---

## 📞 Hỗ Trợ Thêm

Nếu gặp vấn đề, hãy cung cấp:
1. Mã lỗi cụ thể
2. Loại máy & firmware version
3. Giao thức đang dùng
4. Kết quả lệnh test kết nối
