package port

import (
	"context"
	"time"

	"attendance-system/internal/domain/entity"
)

// DeviceRepository quản lý persistence cho Device
type DeviceRepository interface {
	Create(ctx context.Context, d *entity.Device) error
	Update(ctx context.Context, d *entity.Device) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*entity.Device, error)
	GetBySerialADMS(ctx context.Context, sn string) (*entity.Device, error) // Tìm thiết bị theo SN ADMS
	List(ctx context.Context) ([]entity.Device, error)
	UpdateStatus(ctx context.Context, id string, status string, checkedAt time.Time) error
	UpdateHeartbeat(ctx context.Context, id string, at time.Time) error // Cập nhật last_heartbeat_at
}

// DeviceCommandRepository quản lý hàng đợi lệnh ADMS
type DeviceCommandRepository interface {
	Enqueue(ctx context.Context, deviceID string, command string) (*entity.DeviceCommandQueue, error)
	GetPending(ctx context.Context, deviceID string) ([]entity.DeviceCommandQueue, error)
	GetByDeviceIDAndCommandID(ctx context.Context, deviceID string, commandID int64) (*entity.DeviceCommandQueue, error)
	MarkSent(ctx context.Context, commandID int64) error
	Ack(ctx context.Context, deviceID string, commandID int64) error
	MarkFailed(ctx context.Context, commandID int64) error
	MarkFailedByDeviceCmdID(ctx context.Context, deviceID string, commandID int64) error
	CancelPendingByDevice(ctx context.Context, deviceID string) (int, error)
}

// FingerprintRepository quản lý template vân tay tập trung
type FingerprintRepository interface {
	Upsert(ctx context.Context, fp *entity.EmployeeFingerprint) error
	ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeFingerprint, error)
	GetByEmployeeAndFinger(ctx context.Context, employeeID string, fingerIndex int) (*entity.EmployeeFingerprint, error)
	Delete(ctx context.Context, employeeID string, fingerIndex int) error
}

// EmployeeRepository quản lý persistence cho Employee
type EmployeeRepository interface {
	Create(ctx context.Context, e *entity.Employee) error
	Update(ctx context.Context, e *entity.Employee) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*entity.Employee, error)
	GetByCode(ctx context.Context, code string) (*entity.Employee, error)
	List(ctx context.Context) ([]entity.Employee, error)
	ListActive(ctx context.Context) ([]entity.Employee, error)
}

type EmployeeDeviceMappingRepository interface {
	Upsert(ctx context.Context, mapping *entity.EmployeeDeviceMapping) error
	ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeDeviceMapping, error)
	ListByDevice(ctx context.Context, deviceID string) ([]entity.EmployeeDeviceMapping, error)
	GetByEmployeeAndDevice(ctx context.Context, employeeID, deviceID string) (*entity.EmployeeDeviceMapping, error)
	GetByDeviceUserID(ctx context.Context, deviceID, deviceUserID string) (*entity.EmployeeDeviceMapping, error)
	MarkFingerprintEnrolled(ctx context.Context, employeeID, deviceID string, enrolledAt time.Time) error
}

// AttendanceLogRepository quản lý persistence cho AttendanceLog (append-only)
type AttendanceLogRepository interface {
	BulkInsert(ctx context.Context, logs []entity.AttendanceLog) (inserted int, err error)
	Query(ctx context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error)
}

// SyncHistoryRepository quản lý persistence cho SyncHistory
type SyncHistoryRepository interface {
	Create(ctx context.Context, h *entity.SyncHistory) error
	Update(ctx context.Context, h *entity.SyncHistory) error
	List(ctx context.Context, deviceID, status string) ([]entity.SyncHistory, error)
	GetByID(ctx context.Context, id string) (*entity.SyncHistory, error)
}

type SyncCursorRepository interface {
	GetAttendanceCursor(ctx context.Context, deviceID string) (*time.Time, error)
	SetAttendanceCursor(ctx context.Context, deviceID string, cursor time.Time) error
}

// UserRepository quản lý persistence cho User (tài khoản quản trị)
type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
}

// ShiftRepository quản lý ca làm việc
type ShiftRepository interface {
	Create(ctx context.Context, s *entity.Shift) error
	GetByID(ctx context.Context, id string) (*entity.Shift, error)
	List(ctx context.Context) ([]entity.Shift, error)
	Delete(ctx context.Context, id string) error
}

// EmployeeShiftRepository quản lý phân ca
type EmployeeShiftRepository interface {
	Create(ctx context.Context, es *entity.EmployeeShift) error
	GetActiveShiftForEmployee(ctx context.Context, employeeID string, date time.Time) (*entity.EmployeeShift, error)
	Delete(ctx context.Context, id string) error
}

// LeaveRequestRepository quản lý xin nghỉ phép
type LeaveRequestRepository interface {
	Create(ctx context.Context, lr *entity.LeaveRequest) error
	UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error
	List(ctx context.Context, employeeID string, status string) ([]entity.LeaveRequest, error)
	GetByID(ctx context.Context, id string) (*entity.LeaveRequest, error)
	CheckLeaveOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.LeaveRequest, error)
}

// DailyAttendanceRepository quản lý bảng công tổng hợp
type DailyAttendanceRepository interface {
	Upsert(ctx context.Context, da *entity.DailyAttendance) error
	Query(ctx context.Context, employeeID string, from, to time.Time) ([]entity.DailyAttendance, error)
}

// OvertimeRequestRepository quản lý làm thêm giờ
type OvertimeRequestRepository interface {
	Create(ctx context.Context, ot *entity.OvertimeRequest) error
	UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error
	List(ctx context.Context, employeeID string, status string) ([]entity.OvertimeRequest, error)
	GetByID(ctx context.Context, id string) (*entity.OvertimeRequest, error)
	GetApprovedOTOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.OvertimeRequest, error)
}

// AttendanceCorrectionRepository quản lý đơn sửa công
type AttendanceCorrectionRepository interface {
	Create(ctx context.Context, ac *entity.AttendanceCorrection) error
	UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error
	List(ctx context.Context, employeeID string, status string) ([]entity.AttendanceCorrection, error)
	GetByID(ctx context.Context, id string) (*entity.AttendanceCorrection, error)
}

// AuditLogRepository quản lý audit log
type AuditLogRepository interface {
	Create(ctx context.Context, al *entity.AuditLog) error
	List(ctx context.Context, limit int, offset int) ([]entity.AuditLog, error)
}
