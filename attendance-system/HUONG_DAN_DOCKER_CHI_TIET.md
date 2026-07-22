# HƯỚNG DẪN CHI TIẾT TRIỂN KHAI HỆ THỐNG TRÊN DOCKER

Tài liệu này hướng dẫn chi tiết từng bước xây dựng (build) và vận hành hệ thống Chấm công doanh nghiệp (Attendance System) bằng Docker và Docker Compose trên cả môi trường Windows và Linux.

---

## 📌 LƯU Ý KIẾN TRÚC QUAN TRỌNG (Dành cho IT & Cán bộ Kỹ thuật)

Trước khi thực hiện triển khai trên Docker, bạn cần hiểu rõ cơ chế giao tiếp phần cứng của dự án:

1. **Dòng máy hỗ trợ hoàn hảo trên Docker (Linux):**
   - **ZKTeco ADMS (Push):** Các thiết bị tự động đẩy log chấm công qua giao thức HTTP POST về server. Giao thức này chỉ yêu cầu cổng mạng Web thông thường nên Docker Linux chạy rất ổn định.
   - **Hikvision (ISAPI REST API):** Giao tiếp qua HTTP REST API.
   - **Sunbeam (REST API):** Giao tiếp qua HTTP REST API.

2. **Dòng máy ZKTeco Standalone cổ điển (Yêu cầu Windows COM SDK / `zkemkeeper.dll`):**
   - Thư viện COM SDK ActiveX của ZKTeco **không thể chạy** trong Docker Linux container do Docker Linux không có Windows API và Registry cần thiết để đăng ký tệp DLL.
   - **Phương án giải quyết:**
     - **Cách 1 (Khuyên dùng nếu chỉ dùng máy ADMS/Hikvision/Sunbeam):** Triển khai toàn bộ trên Docker (PostgreSQL, Go Server, Adminer) theo hướng dẫn dưới đây.
     - **Cách 2 (Nếu bắt buộc dùng máy ZKTeco Standalone kéo log):**
       - Khởi chạy Database PostgreSQL trên Docker (để quản lý dữ liệu tập trung, tối ưu hiệu năng).
       - Khởi chạy Go backend trực tiếp trên Windows Server / PC Windows (bằng file `server.exe` đã được đăng ký DCOM SDK).
       - Cấu hình file `config.yaml` trên Windows trỏ database kết nối tới IP của Docker Postgres.

---

## 🛠️ CHUẨN BỊ MÔI TRƯỜNG

1. **Cài đặt Docker Desktop:**
   - Trên **Windows/macOS:** Tải và cài đặt [Docker Desktop](https://www.docker.com/products/docker-desktop/).
   - Trên **Linux (Ubuntu/CentOS):** Cài đặt Docker Engine và Docker Compose v2.
     ```bash
     sudo apt-get update
     sudo apt-get install docker-ce docker-ce-cli containerd.io docker-compose-plugin
     ```
2. **Kiểm tra cài đặt:**
   Mở Terminal/PowerShell và kiểm tra phiên bản:
   ```bash
   docker --version
   docker compose version
   ```

---

## 🚀 CÁC BƯỚC TRIỂN KHAI CHI TIẾT

### BƯỚC 1: Kiểm tra cấu hình kết nối Database

Mở file cấu hình Docker Compose tại đường dẫn [docker-compose.yml](file:///e:/Project/attendance-system/docker/docker-compose.yml):
```yaml
      ATTENDANCE_POSTGRES_DSN: "postgres://attendance:attendance@postgres:5432/attendance_db?sslmode=disable"
```
*Lưu ý:* Biến `postgres` trong chuỗi kết nối chính là tên dịch vụ container cơ sở dữ liệu được định nghĩa trong Docker Compose. Docker sẽ tự động phân giải tên miền nội bộ này sang IP của container. Bạn **không cần** sửa đổi chuỗi này trừ khi muốn đổi tên DB, Username hoặc Password.

---

### BƯỚC 2: Khởi chạy và Biên dịch (Build) bằng Docker Compose

Mở PowerShell (trên Windows) hoặc Terminal (trên Linux), điều hướng đến thư mục dự án và thực hiện các lệnh sau:

```powershell
# 1. Di chuyển vào thư mục docker chứa cấu hình
cd docker

# 2. Khởi chạy Docker Compose (Tự động tải DB, build mã nguồn Go và chạy ngầm)
docker compose up -d --build
```

**Giải thích các tham số:**
- `up`: Khởi chạy các container được định nghĩa trong file `docker-compose.yml`.
- `-d` (detached): Chạy các tiến trình ngầm trong background, giải phóng terminal.
- `--build`: Ép buộc Docker biên dịch lại mã nguồn Go mới nhất từ máy của bạn vào Image (giúp cập nhật các bản sửa lỗi code mới nhất).

---

### BƯỚC 3: Kiểm tra trạng thái hoạt động của Container

Kiểm tra xem các container đã khởi động thành công và ở trạng thái khỏe mạnh (healthy) chưa:
```powershell
docker compose ps
```

Kết quả mong đợi hiển thị trên màn hình:
```text
NAME                  IMAGE                COMMAND                  SERVICE    CREATED          STATUS                    PORTS
attendance-adminer    adminer:latest       "entrypoint.sh php -…"   adminer    30 seconds ago   Up 30 seconds             0.0.0.0:8081->8080/tcp
attendance-app        docker-app           "/app/server"            app        30 seconds ago   Up 30 seconds             0.0.0.0:8080->8080/tcp
attendance-postgres   postgres:16-alpine   "docker-entrypoint.s…"   postgres   30 seconds ago   Up 30 seconds (healthy)   0.0.0.0:5432->5432/tcp
```
*(Cột STATUS của `attendance-postgres` phải hiển thị `healthy` - tức là database đã sẵn sàng nhận kết nối).*

---

### BƯỚC 4: Theo dõi Log khởi động hệ thống

Nếu bạn muốn kiểm tra xem Go backend có kết nối thành công với database và chạy migrations để tạo bảng tự động hay không, hãy chạy lệnh xem log:
```powershell
docker compose logs -f app
```
*Nhấn `Ctrl + C` để thoát khỏi chế độ xem log.*

---

### BƯỚC 5: Truy cập Giao diện Web

Sau khi cả 3 container đã ở trạng thái `Up`, bạn mở trình duyệt web lên và truy cập:

1. **Giao diện Dashboard Chấm công:** [http://localhost:8080](http://localhost:8080)
   - Đầy đủ tính năng: Giám sát Log real-time, Quản lý Nhân sự, Ca làm việc, Phê duyệt Đơn từ ESS, Báo cáo tháng.
2. **Trình quản trị Database Adminer:** [http://localhost:8081](http://localhost:8081)
   - Nhập thông tin đăng nhập sau để truy cập bảng DB trực tiếp:
     - **Hệ quản trị (System):** PostgreSQL
     - **Máy chủ (Server):** postgres
     - **Người dùng (Username):** attendance
     - **Mật khẩu (Password):** attendance
     - **Cơ sở dữ liệu (Database):** attendance_db

---

## 🛠️ MỘT SỐ LỆNH VẬN HÀNH THƯỜNG DÙNG

### 1. Dừng hệ thống tạm thời
Khi muốn dừng hệ thống mà không xóa dữ liệu:
```powershell
docker compose stop
```

### 2. Khởi chạy lại hệ thống đã dừng
```powershell
docker compose start
```

### 3. Tắt và dọn dẹp hệ thống (Xóa Container, giữ lại Data)
Khi muốn tắt hẳn hệ thống và giải phóng tài nguyên CPU/RAM:
```powershell
docker compose down
```
*Lưu ý:* Cơ sở dữ liệu của bạn được lưu an toàn tại volume `attendance_pgdata` nên khi chạy `docker compose down` dữ liệu sẽ **không bị mất**.

### 4. Xóa sạch toàn bộ để cài đặt lại từ đầu (Xóa cả database cũ)
> [!CAUTION]
> Lệnh này sẽ xóa sạch toàn bộ dữ liệu trong Database. Chỉ sử dụng khi kiểm thử hoặc muốn dọn sạch dữ liệu cũ làm lại từ đầu.
```powershell
docker compose down -v
```
*(Tham số `-v` sẽ xóa volume lưu trữ dữ liệu database `attendance_pgdata`).*
