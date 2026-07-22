# Báo cáo Chức năng: Đăng ký & Quản lý Nhiều Vân Tay (0-9)

Tài liệu này báo cáo chi tiết về việc nâng cấp hệ thống để hỗ trợ đăng ký và quản lý đồng thời lên đến **10 mẫu vân tay khác nhau** (tương ứng với các chỉ số ngón từ `0` đến `9`) cho mỗi nhân viên, khắc phục triệt để giới hạn chỉ lưu được duy nhất 1 ngón trước đây.

---

## 1. Tổng quan Tính năng
*   **Mục tiêu**: Giúp người quản trị có thể đăng ký nhiều ngón tay cho một nhân viên đề phòng trường hợp một ngón bị thương hoặc khó quét.
*   **Hỗ trợ đa giao thức**: Chức năng hoạt động tương thích hoàn toàn trên cả thiết bị chạy **ADMS Push** và **COM SDK** (Windows Pull).
*   **Tương thích ngược**: Các API cũ hoặc thiết bị chưa nâng cấp vẫn hoạt động bình thường nhờ cơ chế tự động gán chỉ số mặc định là `0` (Ngón cái phải).

---

## 2. Chi tiết Kiến trúc & Triển khai

### 2.1. Tầng Cơ sở dữ liệu (PostgreSQL)
Hệ thống sử dụng bảng `employee_fingerprint` đã được tối ưu hóa trước đó:
*   Mỗi bản ghi đại diện cho một mẫu vân tay của nhân viên.
*   Ràng buộc duy nhất `UNIQUE(employee_id, finger_index)` đảm bảo một nhân viên có thể lưu tối đa 10 mẫu vân tay tương ứng với 10 ngón tay độc lập.
*   Khi người dùng đăng ký lại một ngón tay cụ thể, hệ thống sẽ thực hiện `UPSERT` (cập nhật nếu đã tồn tại, chèn mới nếu chưa có) dựa trên cặp khóa này, đảm bảo không ảnh hưởng đến các ngón tay khác đã đăng ký.

### 2.2. Tầng Go Backend (API & Nghiệp vụ)
*   **API Endpoint**:
    *   `POST /api/v1/employees/{id}/fingerprints/enroll`: Hỗ trợ thêm trường `finger_index` trong JSON request body.
    *   `POST /api/v1/employees/{id}/fingerprints/re-enroll`: Hỗ trợ thêm trường `finger_index` trong JSON request body.
*   **Xử lý Logic (`BiometricService`)**:
    *   Tham số `fingerIndex int` được truyền qua tất cả các hàm dịch vụ.
    *   **ADMS**: Lệnh đăng ký từ xa (`ENROLL_FP`) được xây dựng động với tham số `FID` truyền chỉ số ngón chính xác (ví dụ: `FID=1`, `FID=2`...) thay vì cứng `FID=0`. Khi máy gửi dữ liệu vân tay về qua gói tin ADMS, server sẽ giải mã trường `FID` / `FingerID` tương ứng để lưu trữ chính xác.
    *   **COM SDK**: Truyền `fingerIndex` trực tiếp vào hàm `EnrollFingerprint` của Adapter để kích hoạt phiên quét ngón cụ thể trên máy tính chạy SDK.

### 2.3. Tầng Giao diện người dùng (Frontend Web UI & UX)
*   **Dropdown Chọn ngón tay trực quan**:
    *   Khi người quản trị mở modal quản lý vân tay của nhân viên, dropdown sẽ liệt kê toàn bộ 10 ngón tay kèm theo tên gọi tiếng Việt/tiếng Anh rõ ràng.
    *   Các ngón tay đã đăng ký trong DB sẽ tự động hiển thị trạng thái `(Đã đăng ký)` ngay trên danh sách chọn (Ví dụ: *Ngón trỏ phải (1) (Đã đăng ký)*) giúp người quản trị dễ dàng quan sát trước khi quyết định đăng ký.
*   **Hành động nhanh trực tiếp trên danh sách**:
    *   Hệ thống liệt kê toàn bộ các mẫu vân tay hiện có dưới dạng danh sách thân thiện.
    *   Mỗi ngón tay đã đăng ký sẽ đi kèm 2 nút thao tác nhanh:
        *   `🔁` **Đăng ký lại ngón này (Re-enroll)**: Kích hoạt phiên quét đè mẫu vân tay mới cho đúng ngón đó.
        *   `🗑️` **Xóa ngón này**: Xóa mẫu vân tay đó khỏi database và thu hồi lệnh trên thiết bị.

---

## 3. Hướng dẫn Vận hành & Sử dụng

### Bước 1: Mở Trình quản lý Vân tay
1. Vào tab **Nhân viên** trên Dashboard.
2. Tìm nhân viên cần quản lý và nhấp vào biểu tượng 🖐️ **Vân tay**.
3. Chọn thiết bị chấm công muốn dùng để quét vân tay.

### Bước 2: Đăng ký Ngón tay Mới
1. Tại dropdown chọn ngón, chọn ngón tay chưa đăng ký (Ví dụ: **Ngón trỏ phải (1)**).
2. Nhấn nút **Bắt đầu quét**.
3. Nhân viên đặt ngón tay lên máy chấm công để quét (3 lần).
4. Sau khi thiết bị quét thành công và gửi dữ liệu về, hệ thống sẽ tự động cập nhật danh sách và thêm ngón trỏ phải vừa quét vào.

### Bước 3: Đăng ký lại hoặc Xóa một ngón cụ thể
*   Để quét lại một ngón (ví dụ ngón cái bị mờ): bấm nút `🔁` ngay bên cạnh ngón đó trong danh sách, đặt ngón tay lên máy quét lại.
*   Để xóa bớt ngón tay dư thừa: bấm nút `🗑️` bên cạnh ngón đó, xác nhận xóa.
