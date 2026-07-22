package port

import (
	"context"
	"time"

	"attendance-system/internal/domain/entity"
)

// DeviceConfig cấu hình kết nối tới thiết bị chấm công
type DeviceConfig struct {
	IPAddress string
	Port      int
	// CommKey / password dùng cho SDK ZKTeco, tuỳ hãng có thể mở rộng thêm field
	CommKey  string
	Username string
	Password string
	Timeout  time.Duration
}

// DeviceStatus trạng thái hiện tại của thiết bị
type DeviceStatus struct {
	Online       bool   `json:"online"`
	FirmwareInfo string `json:"firmware_info"`
	UserCount    int    `json:"user_count"`
	LogCount     int    `json:"log_count"`
}

// DeviceAdapter là interface chung mà mọi hãng máy chấm công (ZKTeco, Sunbeam/Timmy,
// Hikvision...) phải implement. Service layer (usecase) CHỈ phụ thuộc vào interface
// này, không bao giờ gọi trực tiếp SDK của hãng nào.
type DeviceAdapter interface {
	Connect(ctx context.Context, cfg DeviceConfig) error
	Disconnect(ctx context.Context) error
	CheckStatus(ctx context.Context) (DeviceStatus, error)
	SyncTime(ctx context.Context) error

	GetEmployees(ctx context.Context) ([]entity.Employee, error)
	PushEmployee(ctx context.Context, emp entity.Employee) error
	PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error
	GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error)

	GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error)
	ClearAttendanceLogs(ctx context.Context) error // optional, tuỳ hãng hỗ trợ

	Reboot(ctx context.Context) error // optional, tuỳ hãng hỗ trợ
	Reset(ctx context.Context) error  // optional, tuỳ hãng hỗ trợ
	EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error // optional, tuỳ hãng hỗ trợ
}

// DeviceAdapterFactory tạo ra DeviceAdapter tương ứng theo loại thiết bị
type DeviceAdapterFactory interface {
	NewAdapter(deviceType entity.DeviceType) (DeviceAdapter, error)
}
