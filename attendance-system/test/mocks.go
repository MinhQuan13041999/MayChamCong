package test

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// MockDeviceAdapter là mock cho port.DeviceAdapter, dùng testify/mock.
// Cho phép test Service layer mà không cần thiết bị thật.
type MockDeviceAdapter struct {
	mock.Mock
}

func (m *MockDeviceAdapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockDeviceAdapter) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(port.DeviceStatus), args.Error(1)
}

func (m *MockDeviceAdapter) SyncTime(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	args := m.Called(ctx)
	return args.Get(0).([]entity.Employee), args.Error(1)
}

func (m *MockDeviceAdapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	args := m.Called(ctx, emp)
	return args.Error(0)
}

func (m *MockDeviceAdapter) DeleteEmployee(ctx context.Context, employeeCode string) error {
	args := m.Called(ctx, employeeCode)
	return args.Error(0)
}

func (m *MockDeviceAdapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	args := m.Called(ctx, employeeCode, fp)
	return args.Error(0)
}

func (m *MockDeviceAdapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	args := m.Called(ctx, employeeCode, fingerIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.EmployeeFingerprint), args.Error(1)
}

func (m *MockDeviceAdapter) DeleteFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	args := m.Called(ctx, employeeCode, fingerIndex)
	return args.Error(0)
}

func (m *MockDeviceAdapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]entity.AttendanceLog), args.Error(1)
}

func (m *MockDeviceAdapter) ClearAttendanceLogs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) ClearEmployees(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) Reboot(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) Reset(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDeviceAdapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	args := m.Called(ctx, employeeCode, fingerIndex)
	return args.Error(0)
}

// MockDeviceAdapterFactory trả về 1 adapter mock cố định, bất kể device type nào.
type MockDeviceAdapterFactory struct {
	Adapter port.DeviceAdapter
	Err     error
}

func (f *MockDeviceAdapterFactory) NewAdapter(deviceType entity.DeviceType) (port.DeviceAdapter, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Adapter, nil
}

// ---- Repository mocks ----

type MockDeviceRepository struct {
	mock.Mock
}

func (m *MockDeviceRepository) Create(ctx context.Context, d *entity.Device) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}
func (m *MockDeviceRepository) Update(ctx context.Context, d *entity.Device) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}
func (m *MockDeviceRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockDeviceRepository) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Device), args.Error(1)
}
func (m *MockDeviceRepository) List(ctx context.Context) ([]entity.Device, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	devices := args.Get(0)
	switch v := devices.(type) {
	case []entity.Device:
		return v, args.Error(1)
	case []*entity.Device:
		out := make([]entity.Device, 0, len(v))
		for _, d := range v {
			if d != nil {
				out = append(out, *d)
			}
		}
		return out, args.Error(1)
	default:
		return nil, args.Error(1)
	}
}
func (m *MockDeviceRepository) UpdateStatus(ctx context.Context, id string, status string, checkedAt time.Time) error {
	args := m.Called(ctx, id, status, checkedAt)
	return args.Error(0)
}

type MockSyncHistoryRepository struct {
	mock.Mock
}

func (m *MockSyncHistoryRepository) Create(ctx context.Context, h *entity.SyncHistory) error {
	args := m.Called(ctx, h)
	return args.Error(0)
}
func (m *MockSyncHistoryRepository) Update(ctx context.Context, h *entity.SyncHistory) error {
	args := m.Called(ctx, h)
	return args.Error(0)
}
func (m *MockSyncHistoryRepository) List(ctx context.Context, deviceID, status string) ([]entity.SyncHistory, error) {
	args := m.Called(ctx, deviceID, status)
	return args.Get(0).([]entity.SyncHistory), args.Error(1)
}
func (m *MockSyncHistoryRepository) GetByID(ctx context.Context, id string) (*entity.SyncHistory, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.SyncHistory), args.Error(1)
}

type MockAttendanceLogRepository struct {
	mock.Mock
}

func (m *MockAttendanceLogRepository) BulkInsert(ctx context.Context, logs []entity.AttendanceLog) (int, error) {
	args := m.Called(ctx, logs)
	return args.Int(0), args.Error(1)
}
func (m *MockAttendanceLogRepository) Query(ctx context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error) {
	args := m.Called(ctx, from, to, employeeCode, deviceID)
	return args.Get(0).([]entity.AttendanceLog), args.Error(1)
}
func (m *MockAttendanceLogRepository) UpdateValidity(ctx context.Context, id int64, isValid bool, reason string) error {
	return m.Called(ctx, id, isValid, reason).Error(0)
}
func (m *MockAttendanceLogRepository) UpdateClassification(ctx context.Context, id int64, isValid bool, reason, workSegment string, overtimeRequestID *string) error {
	return m.Called(ctx, id, isValid, reason, workSegment, overtimeRequestID).Error(0)
}

// ---- MockEmployeeRepository ----

type MockEmployeeRepository struct {
	mock.Mock
}

func (m *MockEmployeeRepository) Create(ctx context.Context, e *entity.Employee) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}
func (m *MockEmployeeRepository) Update(ctx context.Context, e *entity.Employee) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}
func (m *MockEmployeeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockEmployeeRepository) DeleteAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockEmployeeRepository) GetByID(ctx context.Context, id string) (*entity.Employee, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Employee), args.Error(1)
}
func (m *MockEmployeeRepository) GetByCode(ctx context.Context, code string) (*entity.Employee, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Employee), args.Error(1)
}
func (m *MockEmployeeRepository) List(ctx context.Context) ([]entity.Employee, error) {
	args := m.Called(ctx)
	return args.Get(0).([]entity.Employee), args.Error(1)
}
func (m *MockEmployeeRepository) ListActive(ctx context.Context) ([]entity.Employee, error) {
	args := m.Called(ctx)
	return args.Get(0).([]entity.Employee), args.Error(1)
}

// MockAttendanceCorrectionRepository là mock cho port.AttendanceCorrectionRepository
type MockAttendanceCorrectionRepository struct {
	mock.Mock
}

func (m *MockAttendanceCorrectionRepository) Create(ctx context.Context, ac *entity.AttendanceCorrection) error {
	args := m.Called(ctx, ac)
	return args.Error(0)
}

func (m *MockAttendanceCorrectionRepository) UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error {
	args := m.Called(ctx, id, status, approvedBy)
	return args.Error(0)
}

func (m *MockAttendanceCorrectionRepository) List(ctx context.Context, employeeID string, status string) ([]entity.AttendanceCorrection, error) {
	args := m.Called(ctx, employeeID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.AttendanceCorrection), args.Error(1)
}

func (m *MockAttendanceCorrectionRepository) GetByID(ctx context.Context, id string) (*entity.AttendanceCorrection, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.AttendanceCorrection), args.Error(1)
}

func (m *MockDeviceRepository) GetBySerialADMS(ctx context.Context, sn string) (*entity.Device, error) {
	args := m.Called(ctx, sn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Device), args.Error(1)
}

func (m *MockDeviceRepository) UpdateHeartbeat(ctx context.Context, id string, at time.Time) error {
	args := m.Called(ctx, id, at)
	return args.Error(0)
}

type MockDeviceCommandRepository struct {
	mock.Mock
}

func (m *MockDeviceCommandRepository) Enqueue(ctx context.Context, deviceID string, command string) (*entity.DeviceCommandQueue, error) {
	args := m.Called(ctx, deviceID, command)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.DeviceCommandQueue), args.Error(1)
}
func (m *MockDeviceCommandRepository) GetPending(ctx context.Context, deviceID string) ([]entity.DeviceCommandQueue, error) {
	args := m.Called(ctx, deviceID)
	return args.Get(0).([]entity.DeviceCommandQueue), args.Error(1)
}
func (m *MockDeviceCommandRepository) GetByDeviceIDAndCommandID(ctx context.Context, deviceID string, commandID int64) (*entity.DeviceCommandQueue, error) {
	args := m.Called(ctx, deviceID, commandID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.DeviceCommandQueue), args.Error(1)
}
func (m *MockDeviceCommandRepository) MarkSent(ctx context.Context, commandID int64) error {
	args := m.Called(ctx, commandID)
	return args.Error(0)
}
func (m *MockDeviceCommandRepository) Ack(ctx context.Context, deviceID string, commandID int64) error {
	args := m.Called(ctx, deviceID, commandID)
	return args.Error(0)
}
func (m *MockDeviceCommandRepository) MarkFailed(ctx context.Context, commandID int64) error {
	args := m.Called(ctx, commandID)
	return args.Error(0)
}
func (m *MockDeviceCommandRepository) MarkFailedByDeviceCmdID(ctx context.Context, deviceID string, commandID int64) error {
	args := m.Called(ctx, deviceID, commandID)
	return args.Error(0)
}
func (m *MockDeviceCommandRepository) CancelPendingByDevice(ctx context.Context, deviceID string) (int, error) {
	args := m.Called(ctx, deviceID)
	return args.Int(0), args.Error(1)
}

type MockFingerprintRepository struct {
	mock.Mock
}

func (m *MockFingerprintRepository) Upsert(ctx context.Context, fp *entity.EmployeeFingerprint) error {
	args := m.Called(ctx, fp)
	return args.Error(0)
}
func (m *MockFingerprintRepository) ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeFingerprint, error) {
	args := m.Called(ctx, employeeID)
	return args.Get(0).([]entity.EmployeeFingerprint), args.Error(1)
}
func (m *MockFingerprintRepository) GetByEmployeeAndFinger(ctx context.Context, employeeID string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	args := m.Called(ctx, employeeID, fingerIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.EmployeeFingerprint), args.Error(1)
}
func (m *MockFingerprintRepository) Delete(ctx context.Context, employeeID string, fingerIndex int) error {
	args := m.Called(ctx, employeeID, fingerIndex)
	return args.Error(0)
}

type MockEmployeeDeviceMappingRepository struct {
	mock.Mock
}

func (m *MockEmployeeDeviceMappingRepository) Upsert(ctx context.Context, mapping *entity.EmployeeDeviceMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}
func (m *MockEmployeeDeviceMappingRepository) ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeDeviceMapping, error) {
	args := m.Called(ctx, employeeID)
	return args.Get(0).([]entity.EmployeeDeviceMapping), args.Error(1)
}
func (m *MockEmployeeDeviceMappingRepository) ListByDevice(ctx context.Context, deviceID string) ([]entity.EmployeeDeviceMapping, error) {
	args := m.Called(ctx, deviceID)
	return args.Get(0).([]entity.EmployeeDeviceMapping), args.Error(1)
}
func (m *MockEmployeeDeviceMappingRepository) GetByEmployeeAndDevice(ctx context.Context, employeeID, deviceID string) (*entity.EmployeeDeviceMapping, error) {
	args := m.Called(ctx, employeeID, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.EmployeeDeviceMapping), args.Error(1)
}
func (m *MockEmployeeDeviceMappingRepository) GetByDeviceUserID(ctx context.Context, deviceID, deviceUserID string) (*entity.EmployeeDeviceMapping, error) {
	args := m.Called(ctx, deviceID, deviceUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.EmployeeDeviceMapping), args.Error(1)
}
func (m *MockEmployeeDeviceMappingRepository) MarkFingerprintEnrolled(ctx context.Context, employeeID, deviceID string, enrolledAt time.Time) error {
	args := m.Called(ctx, employeeID, deviceID, enrolledAt)
	return args.Error(0)
}
