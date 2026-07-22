# HƯỚNG DẪN TẠO BỘ CÀI ĐẶT TỰ ĐỘNG (.EXE INSTALLER) CHO WINDOWS

Tài liệu này hướng dẫn chi tiết từng bước đóng gói toàn bộ hệ thống Chấm công thành một tệp cài đặt `.exe` duy nhất (ví dụ: `AttendanceSystem_Setup.exe`) giúp người dùng khác có thể cài đặt dễ dàng chỉ bằng vài cú click chuột.

---

## 🛠️ BƯỚC 1: BIÊN DỊCH MÃ NGUỒN GO
Mở Terminal/PowerShell tại thư mục gốc của dự án và chạy lệnh sau để biên dịch ứng dụng Go thành file thực thi chạy độc lập trên Windows:

```powershell
# Biên dịch code tối ưu (ẩn cửa sổ console dòng lệnh và giảm kích thước tệp)
go build -ldflags="-s -w -H=windowsgui" -o dist_package/server.exe ./cmd/server
```
*(Tham số `-H=windowsgui` giúp phần mềm chạy ngầm không hiện cửa sổ cmd màu đen khi chạy trực tiếp).*

---

## 📁 BƯỚC 2: CHUẨN BỊ THƯ MỤC ĐÓNG GÓI (`dist_package`)
Bạn cần tạo cấu trúc thư mục `dist_package` tại thư mục gốc dự án như sau:

```text
attendance-system/
  └── dist_package/
        ├── server.exe         (Tệp thực thi Go vừa biên dịch ở Bước 1)
        ├── config.yaml        (Copy từ config.example.yaml và sửa cấu hình mặc định)
        ├── nssm.exe           (Công cụ đăng ký Service chạy ngầm)
        ├── web/               (Copy toàn bộ thư mục giao diện web từ dự án)
        ├── migrations/        (Copy toàn bộ thư mục chứa các file SQL database)
        └── sdk/               (Chứa các file DLL kết nối máy chấm công ZKTeco)
              ├── zkemkeeper.dll
              ├── commpro.dll
              ├── comms.dll
              ├── rscomm.dll
              ├── tcpcomm.dll
              └── usbcomm.dll
```

> [!TIP]
> * Tải công cụ **NSSM** (phiên bản Windows 64-bit) miễn phí tại: https://nssm.cc/download
> * Các file DLL SDK của ZKTeco có thể tìm thấy trong thư mục cài đặt phần mềm chấm công cũ của hãng hoặc trong gói SDK ZKTeco.

---

## ⚙️ BƯỚC 3: CÀI ĐẶT VÀ BIÊN DỊCH BỘ CÀI ĐẶT VỚI INNO SETUP
1. Tải công cụ đóng gói phần mềm chuyên nghiệp **Inno Setup** tại: https://jrsoftware.org/isdl.php và cài đặt trên máy tính của bạn.
2. Mở phần mềm **Inno Setup Compiler** vừa cài đặt.
3. Chọn **Open an existing script file** và mở tệp [setup_script.iss](file:///e:/Project/attendance-system/setup_script.iss) đã được tạo sẵn ở thư mục gốc của dự án.
4. Nhấn nút **Compile** (Biểu tượng nút Play màu xanh lá cây hoặc nhấn phím `F9`).
5. Quá trình biên dịch sẽ hoàn tất trong vòng vài giây. Tệp cài đặt duy nhất của bạn sẽ được tạo ra tại thư mục: `Output/AttendanceSystem_Setup.exe`.

---

## 🚀 BƯỚC 4: TIẾN HÀNH CÀI ĐẶT TRÊN MÁY TÍNH MỚI
Chuyển tệp `AttendanceSystem_Setup.exe` sang máy tính khách hàng/máy tính mới và thực hiện:

1. **Chạy file cài đặt:** Click đúp chuột vào file `.exe`, nhấn **Next -> Next -> Install -> Finish** (yêu cầu quyền Admin).
2. **Cơ chế hoạt động tự động:**
   * Trình cài đặt sẽ tự động copy toàn bộ code và giao diện vào ổ `C:\Program Files\AttendanceSystem`.
   * Tự động đăng ký các file DLL của ZKTeco vào System32 và gọi `regsvr32` ngầm.
   * Tự động tạo và kích hoạt một dịch vụ Windows Service tên là `AttendanceService` chạy ngầm (bạn có thể kiểm tra trong ứng dụng `Services` của Windows).
3. **Sử dụng:**
   * Mở trình duyệt web và truy cập `http://localhost:8085` để bắt đầu quản lý.
   * Để gỡ cài đặt phần mềm sạch sẽ, chỉ cần vào Control Panel -> Uninstall a program và gỡ cài đặt **Hệ Thống Chấm Công Doanh Nghiệp**.
