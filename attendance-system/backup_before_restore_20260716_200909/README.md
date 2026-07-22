# Attendance System (Golang + PostgreSQL)

Hệ thống chấm công web, tích hợp nhiều dòng máy chấm công (ZKTeco, Sunbeam/Timmy, Hikvision)
qua Clean Architecture. Hệ thống chỉ lưu trữ **raw log chấm công** và đồng bộ nhân viên
hai chiều — **không tính công**; dữ liệu được cung cấp qua API cho các hệ thống khác
(payroll, HRM...) sử dụng.

## Kiến trúc

Clean Architecture 4 lớp, dependency luôn đi vào trong:

```
Interfaces (HTTP handler)  →  Application (usecase/service)  →  Domain (entity/port)  ←  Infrastructure (postgres, adapter, scheduler, logger)
```

- **Domain** định nghĩa `port.DeviceAdapter` — interface chung mọi hãng máy phải implement.
- **Application/usecase** chỉ phụ thuộc vào `port.DeviceAdapter`, không bao giờ gọi SDK hãng máy trực tiếp.
- **Infrastructure/adapter** chứa implementation cụ thể cho từng hãng: `zkteco/`, `sunbeam/`, `hikvision/`.
- Thêm hãng máy mới = thêm 1 package trong `internal/infrastructure/adapter/`, implement interface, đăng ký vào `factory.go`. Không sửa Service/Controller.

## Cấu trúc thư mục

```
attendance-system/
├── cmd/server/main.go              # composition root, khởi động HTTP server + scheduler
├── internal/
│   ├── domain/entity/              # Employee, Device, AttendanceLog, SyncHistory...
│   ├── domain/port/                # DeviceAdapter interface, Repository interfaces
│   ├── usecase/                    # DeviceService, EmployeeService, SyncService
│   ├── interface/http/             # handler, router, middleware
│   ├── interface/dto/              # request/response models
│   ├── infrastructure/postgres/    # repository implementation
│   ├── infrastructure/adapter/     # zkteco/, sunbeam/, hikvision/ + factory
│   ├── infrastructure/scheduler/   # cron job đồng bộ tự động
│   ├── infrastructure/logger/      # zap structured logger
│   └── config/                     # load config.yaml + env var
├── docs/swagger.yaml               # OpenAPI spec
├── migrations/                     # SQL migration (golang-migrate compatible)
├── docker/                         # Dockerfile + docker-compose.yml
├── test/                           # unit test + mocks cho DeviceAdapter
└── go.mod
```

## Chạy thử với Docker

```bash
cd docker
docker compose up --build
```

- API: http://localhost:8085/api/v1
- Health check: http://localhost:8085/healthz
- Adminer (quản lý DB): http://localhost:8081 (server: postgres, user/pass: attendance/attendance, db: attendance_db)

## Chạy local (không Docker)

1. Cài PostgreSQL, tạo database `attendance_db`.
2. Copy `config.example.yaml` → `config.yaml`, chỉnh `postgres_dsn`.
3. Chạy migration (dùng [golang-migrate](https://github.com/golang-migrate/migrate) hoặc atlas):
   ```bash
   migrate -path migrations -database "$POSTGRES_DSN" up
   ```
4. Chạy server:
   ```bash
   go run ./cmd/server
   ```

## Chạy test

```bash
go test ./test/...
```

Unit test dùng mock cho `port.DeviceAdapter` (testify/mock) — không cần thiết bị thật.

## Việc cần hoàn thiện tiếp (TODO)

Đây là bộ khung kiến trúc hoàn chỉnh theo đúng kế hoạch thiết kế. Các phần sau cần
tích hợp SDK chính hãng thật (đã đánh dấu `TODO` trong code):

- `internal/infrastructure/adapter/zkteco/` — cắm ZKTeco SDK (TCP/IP protocol hoặc zkemsdk qua cgo).
- `internal/infrastructure/adapter/sunbeam/` — cắm Sunbeam (Timmy) SDK/API.
- `internal/infrastructure/adapter/hikvision/` — cắm Hikvision ISAPI hoặc HCNetSDK.
- `internal/interface/http/employee_handler.go` — import nhân viên từ Excel.
- `internal/interface/http/auth_handler.go` — login (bcrypt) + JWT + RBAC đầy đủ.
- Retry/backoff nâng cao và circuit breaker (`sony/gobreaker`) cho giao tiếp thiết bị hay timeout.

## API Documentation

Xem `docs/swagger.yaml` (OpenAPI 3.0) — có thể import vào Swagger UI hoặc Postman.
