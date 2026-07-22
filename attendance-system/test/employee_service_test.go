package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/usecase"
)

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_CreateEmployee_Success
// Trường hợp: dữ liệu hợp lệ → gọi repo.Create, status tự động set "active"
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_CreateEmployee_Success(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("Create", ctx, mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		new(MockDeviceRepository),
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		new(MockEmployeeDeviceMappingRepository),
		new(MockDeviceCommandRepository),
	)

	e := &entity.Employee{
		EmployeeCode: "EMP001",
		FullName:     "Nguyễn Văn A",
	}
	err := svc.CreateEmployee(ctx, e)

	assert.NoError(t, err)
	assert.Equal(t, "active", e.Status) // service phải set status=active mặc định
	empRepo.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_CreateEmployee_MissingCode
// Trường hợp: thiếu EmployeeCode → validation fail, không gọi repo
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_CreateEmployee_MissingCode(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	svc := usecase.NewEmployeeService(
		empRepo,
		new(MockDeviceRepository),
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		new(MockEmployeeDeviceMappingRepository),
		new(MockDeviceCommandRepository),
	)

	e := &entity.Employee{FullName: "Nguyễn Văn B"}
	err := svc.CreateEmployee(ctx, e)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
	empRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_DeleteAllEmployees_DeletesDatabaseAndQueuesDeviceCommand
// Trường hợp: xóa toàn bộ DB và gửi đúng một lệnh xóa collection tới mỗi máy ADMS.
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_DeleteAllEmployees_DeletesDatabaseAndQueuesDeviceCommand(t *testing.T) {
	ctx := context.Background()
	empRepo := new(MockEmployeeRepository)
	empRepo.On("DeleteAll", ctx).Return(int64(3), nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("List", ctx).Return([]entity.Device{
		{ID: "dev-adms", Name: "ADMS", ADMSEnabled: true, SerialNumberADMS: "SN001"},
		{ID: "dev-sdk", Name: "SDK", DeviceType: entity.DeviceTypeZKTeco},
	}, nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-adms", "DATA DELETE USER").Return(&entity.DeviceCommandQueue{}, nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		deviceRepo,
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		new(MockEmployeeDeviceMappingRepository),
		commandRepo,
	)

	deleted, err := svc.DeleteAllEmployees(ctx)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), deleted)
	empRepo.AssertExpectations(t)
	deviceRepo.AssertExpectations(t)
	commandRepo.AssertExpectations(t)
}

// TestEmployeeService_CreateEmployee_MissingFullName
// Trường hợp: thiếu FullName → validation fail
func TestEmployeeService_CreateEmployee_MissingFullName(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	svc := usecase.NewEmployeeService(
		empRepo,
		new(MockDeviceRepository),
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		new(MockEmployeeDeviceMappingRepository),
		new(MockDeviceCommandRepository),
	)

	e := &entity.Employee{EmployeeCode: "EMP002"}
	err := svc.CreateEmployee(ctx, e)

	assert.Error(t, err)
	empRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_CreateEmployee_RepoError
// Trường hợp: repo.Create trả lỗi (ví dụ duplicate key) → propagate lỗi đó
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_CreateEmployee_RepoError(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("Create", ctx, mock.Anything).Return(errors.New("duplicate key: employee_code"))

	svc := usecase.NewEmployeeService(
		empRepo,
		new(MockDeviceRepository),
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		new(MockEmployeeDeviceMappingRepository),
		new(MockDeviceCommandRepository),
	)

	err := svc.CreateEmployee(ctx, &entity.Employee{EmployeeCode: "EMP003", FullName: "Lê Thị C"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestEmployeeService_CreateEmployeeWithEnrollment_SyncsToDevice(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		emp := args.Get(1).(*entity.Employee)
		emp.ID = "emp-1"
	}).Return(nil)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}, nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-1").Return(&entity.Device{ID: "dev-1", ADMSEnabled: true}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("Upsert", ctx, mock.Anything).Return(nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-1", mock.AnythingOfType("string")).Return(&entity.DeviceCommandQueue{}, nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		deviceRepo,
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		mappingRepo,
		commandRepo,
	)

	err := svc.CreateEmployeeWithEnrollment(ctx, &entity.Employee{EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}, true, "dev-1", "NV001")

	assert.NoError(t, err)
	mappingRepo.AssertExpectations(t)
	commandRepo.AssertExpectations(t)
}

func TestEmployeeService_PushEmployeeToAllDevices_QueuesEnrollAndFingerprintCommands(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}, nil)
	empRepo.On("List", ctx).Return([]entity.Employee{{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}}, nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("List", ctx).Return([]*entity.Device{{ID: "dev-1", Name: "dev-1", ADMSEnabled: true, LastHeartbeatAt: &[]time.Time{time.Now()}[0]}}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("ListByDevice", ctx, "dev-1").Return([]entity.EmployeeDeviceMapping{}, nil)
	mappingRepo.On("Upsert", ctx, mock.Anything).Return(nil)

	fingerprintRepo := new(MockFingerprintRepository)
	fingerprintRepo.On("ListByEmployee", ctx, "emp-1").Return([]entity.EmployeeFingerprint{{EmployeeID: "emp-1", FingerIndex: 1, TemplateData: "abc", TemplateSize: 64}}, nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-1", mock.AnythingOfType("string")).Return(&entity.DeviceCommandQueue{}, nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		deviceRepo,
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		mappingRepo,
		commandRepo,
	)
	svc.SetFingerprintRepo(fingerprintRepo)

	count, errs, err := svc.PushEmployeeToAllDevices(ctx, "emp-1")

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, errs)
	commandRepo.AssertNumberOfCalls(t, "Enqueue", 2)
}

func TestEmployeeService_CreateEmployeeWithEnrollment_UsesSerialBasedADMSDevice(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		emp := args.Get(1).(*entity.Employee)
		emp.ID = "emp-1"
	}).Return(nil)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}, nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-1").Return(&entity.Device{ID: "dev-1", Name: "dev-1", SerialNumberADMS: "SN123", ADMSEnabled: true}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("Upsert", ctx, mock.Anything).Return(nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-1", mock.AnythingOfType("string")).Return(&entity.DeviceCommandQueue{}, nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		deviceRepo,
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		mappingRepo,
		commandRepo,
	)

	err := svc.CreateEmployeeWithEnrollment(ctx, &entity.Employee{EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}, true, "dev-1", "NV001")

	assert.NoError(t, err)
	commandRepo.AssertExpectations(t)
}

func TestEmployeeService_ConfirmFingerprintEnrolled_DeviceNotFound(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1"}, nil)
	empRepo.On("Update", ctx, mock.Anything).Return(nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-404").Return((*entity.Device)(nil), nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("MarkFingerprintEnrolled", ctx, "emp-1", "dev-404", mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(
		empRepo,
		deviceRepo,
		new(MockSyncHistoryRepository),
		&MockDeviceAdapterFactory{},
		mappingRepo,
		new(MockDeviceCommandRepository),
	)

	err := svc.ConfirmFingerprintEnrolled(ctx, "emp-1", "dev-404")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "device not found")
}

func TestEmployeeService_PullEmployeesFromDevice_ADMSWithoutMappingRepo(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-adms").Return(&entity.Device{ID: "dev-adms", ADMSEnabled: true}, nil)

	empRepo := new(MockEmployeeRepository)
	svc := usecase.NewEmployeeService(empRepo, deviceRepo, new(MockSyncHistoryRepository), nil, nil, new(MockDeviceCommandRepository))

	assert.NotPanics(t, func() {
		imported, existing, errs, err := svc.PullEmployeesFromDevice(ctx, "dev-adms")
		assert.NoError(t, err)
		assert.Equal(t, 0, imported)
		assert.Equal(t, 0, existing)
		assert.Empty(t, errs)
	})
}

func TestEmployeeService_PullEmployeesFromDevice_ADMSMarksFingerprintStatus(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-adms").Return(&entity.Device{ID: "dev-adms", ADMSEnabled: true}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("ListByDevice", ctx, "dev-adms").Return([]entity.EmployeeDeviceMapping{{
		EmployeeID:          "emp-1",
		DeviceID:            "dev-adms",
		DeviceUserID:        "NV001",
		FingerprintEnrolled: true,
	}}, nil)

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A", FingerprintEnrolled: false}, nil)
	empRepo.On("Update", ctx, mock.Anything).Run(func(args mock.Arguments) {
		updated := args.Get(1).(*entity.Employee)
		updated.FingerprintEnrolled = true
	}).Return(nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, new(MockSyncHistoryRepository), nil, mappingRepo, new(MockDeviceCommandRepository))

	imported, existing, errs, err := svc.PullEmployeesFromDevice(ctx, "dev-adms")

	assert.NoError(t, err)
	assert.Equal(t, 0, imported)
	assert.Equal(t, 1, existing)
	assert.Empty(t, errs)
	empRepo.AssertExpectations(t)
}

func TestEmployeeService_PullEmployeesFromDevice_UsesStoredFingerprintsToMarkStatus(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-adms").Return(&entity.Device{ID: "dev-adms", ADMSEnabled: true}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("ListByDevice", ctx, "dev-adms").Return([]entity.EmployeeDeviceMapping{{
		EmployeeID:   "emp-1",
		DeviceID:     "dev-adms",
		DeviceUserID: "NV001",
	}}, nil)
	mappingRepo.On("MarkFingerprintEnrolled", ctx, "emp-1", "dev-adms", mock.Anything).Return(nil)

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A", FingerprintEnrolled: false}, nil)
	empRepo.On("Update", ctx, mock.Anything).Run(func(args mock.Arguments) {
		updated := args.Get(1).(*entity.Employee)
		updated.FingerprintEnrolled = true
	}).Return(nil)

	fingerprintRepo := new(MockFingerprintRepository)
	fingerprintRepo.On("ListByEmployee", ctx, "emp-1").Return([]entity.EmployeeFingerprint{{EmployeeID: "emp-1"}}, nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, new(MockSyncHistoryRepository), nil, mappingRepo, new(MockDeviceCommandRepository))
	svc.SetFingerprintRepo(fingerprintRepo)

	imported, existing, errs, err := svc.PullEmployeesFromDevice(ctx, "dev-adms")

	assert.NoError(t, err)
	assert.Equal(t, 0, imported)
	assert.Equal(t, 1, existing)
	assert.Empty(t, errs)
	empRepo.AssertExpectations(t)
	fingerprintRepo.AssertExpectations(t)
}

func TestEmployeeService_PushEmployeeToAllDevices_ResolvesByEmployeeCode(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "26").Return((*entity.Employee)(nil), nil)
	empRepo.On("GetByCode", ctx, "26").Return(&entity.Employee{ID: "emp-26", EmployeeCode: "26", FullName: "Nguyễn Văn 26", CardNo: "123"}, nil)
	empRepo.On("List", ctx).Return([]entity.Employee{{ID: "emp-26", EmployeeCode: "26", FullName: "Nguyễn Văn 26", CardNo: "123"}}, nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("List", ctx).Return([]*entity.Device{{ID: "dev-1", Name: "dev-1", ADMSEnabled: true, LastHeartbeatAt: &[]time.Time{time.Now()}[0]}}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("ListByDevice", ctx, "dev-1").Return([]entity.EmployeeDeviceMapping{}, nil)
	mappingRepo.On("Upsert", ctx, mock.Anything).Return(nil)

	fingerprintRepo := new(MockFingerprintRepository)
	fingerprintRepo.On("ListByEmployee", ctx, "emp-26").Return([]entity.EmployeeFingerprint{{EmployeeID: "emp-26", FingerIndex: 1, TemplateData: "abc", TemplateSize: 10}}, nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-1", mock.AnythingOfType("string")).Return(&entity.DeviceCommandQueue{}, nil).Times(2)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, new(MockSyncHistoryRepository), nil, mappingRepo, commandRepo)
	svc.SetFingerprintRepo(fingerprintRepo)

	count, errs, err := svc.PushEmployeeToAllDevices(ctx, "26")

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, errs)
	commandRepo.AssertExpectations(t)
}

func TestEmployeeService_PushEmployeeToAllDevices_QueuesFingerprintCommands(t *testing.T) {
	ctx := context.Background()

	empRepo := new(MockEmployeeRepository)
	empRepo.On("GetByID", ctx, "emp-1").Return(&entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nguyễn Văn A", CardNo: "123"}, nil)

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("List", ctx).Return([]*entity.Device{{ID: "dev-1", Name: "dev-1", ADMSEnabled: true, LastHeartbeatAt: &[]time.Time{time.Now()}[0]}}, nil)

	mappingRepo := new(MockEmployeeDeviceMappingRepository)
	mappingRepo.On("ListByDevice", ctx, "dev-1").Return([]entity.EmployeeDeviceMapping{}, nil)
	mappingRepo.On("Upsert", ctx, mock.Anything).Return(nil)

	fingerprintRepo := new(MockFingerprintRepository)
	fingerprintRepo.On("ListByEmployee", ctx, "emp-1").Return([]entity.EmployeeFingerprint{{EmployeeID: "emp-1", FingerIndex: 1, TemplateData: "abc", TemplateSize: 10}}, nil)

	commandRepo := new(MockDeviceCommandRepository)
	commandRepo.On("Enqueue", ctx, "dev-1", mock.AnythingOfType("string")).Return(&entity.DeviceCommandQueue{}, nil).Times(2)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, new(MockSyncHistoryRepository), nil, mappingRepo, commandRepo)
	svc.SetFingerprintRepo(fingerprintRepo)

	count, errs, err := svc.PushEmployeeToAllDevices(ctx, "emp-1")

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, errs)
	commandRepo.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_PushEmployeesToDevice_Success
// Trường hợp: đẩy nhân viên thành công → status=success, record_count=2
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_PushEmployeesToDevice_Success(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-1",
		DeviceType: entity.DeviceTypeZKTeco,
		IPAddress:  "192.168.1.100",
		Port:       4370,
	}
	employees := []entity.Employee{
		{EmployeeCode: "EMP001", FullName: "Nguyễn Văn A", Status: "active"},
		{EmployeeCode: "EMP002", FullName: "Trần Thị B", Status: "active"},
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("PushEmployee", ctx, employees[0]).Return(nil)
	mockAdapter.On("PushEmployee", ctx, employees[1]).Return(nil)
	mockAdapter.On("Disconnect", ctx).Return(nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-1").Return(device, nil)

	empRepo := new(MockEmployeeRepository)
	empRepo.On("ListActive", ctx).Return(employees, nil)

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, historyRepo, factory, new(MockEmployeeDeviceMappingRepository), new(MockDeviceCommandRepository))

	hist, err := svc.PushEmployeesToDevice(ctx, "dev-1", entity.SyncTriggerManual)

	assert.NoError(t, err)
	assert.Equal(t, entity.SyncStatusSuccess, hist.Status)
	assert.Equal(t, 2, hist.RecordCount)
	mockAdapter.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_PushEmployeesToDevice_NoActiveEmployees
// Trường hợp: không có nhân viên active → 0 record đẩy, status=success (empty success)
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_PushEmployeesToDevice_NoActiveEmployees(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-2",
		DeviceType: entity.DeviceTypeSunbeam,
		IPAddress:  "192.168.2.10",
		Port:       80,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("Disconnect", ctx).Return(nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-2").Return(device, nil)

	empRepo := new(MockEmployeeRepository)
	empRepo.On("ListActive", ctx).Return([]entity.Employee{}, nil) // danh sách rỗng

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, historyRepo, factory, new(MockEmployeeDeviceMappingRepository), new(MockDeviceCommandRepository))

	hist, err := svc.PushEmployeesToDevice(ctx, "dev-2", entity.SyncTriggerManual)

	assert.NoError(t, err)
	assert.Equal(t, 0, hist.RecordCount)
	assert.Equal(t, entity.SyncStatusSuccess, hist.Status)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_PushEmployeesToDevice_ConnectFail
// Trường hợp: không kết nối được thiết bị → status=failed, error được trả về
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_PushEmployeesToDevice_ConnectFail(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:          "dev-3",
		DeviceType:  entity.DeviceTypeHikvision,
		IPAddress:   "10.0.0.50",
		Port:        80,
		ADMSEnabled: false,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(errors.New("network unreachable"))

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-3").Return(device, nil)

	empRepo := new(MockEmployeeRepository)

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, historyRepo, factory, new(MockEmployeeDeviceMappingRepository), new(MockDeviceCommandRepository))

	hist, err := svc.PushEmployeesToDevice(ctx, "dev-3", entity.SyncTriggerManual)

	assert.Error(t, err)
	assert.Equal(t, entity.SyncStatusFailed, hist.Status)
	assert.Contains(t, hist.ErrorMessage, "network unreachable")
	// ListActive không được gọi vì kết nối thất bại trước đó
	empRepo.AssertNotCalled(t, "ListActive", mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestEmployeeService_PushEmployeesToDevice_PartialSuccess
// Trường hợp: 1 nhân viên đẩy thất bại, 1 thành công → status=partial
// ──────────────────────────────────────────────────────────────────────────────
func TestEmployeeService_PushEmployeesToDevice_PartialSuccess(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-4",
		DeviceType: entity.DeviceTypeZKTeco,
		IPAddress:  "192.168.1.101",
		Port:       4370,
	}
	employees := []entity.Employee{
		{EmployeeCode: "EMP010", FullName: "Phạm Văn D", Status: "active"},
		{EmployeeCode: "EMP011", FullName: "Hoàng Thị E", Status: "active"},
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("PushEmployee", ctx, employees[0]).Return(nil)                        // thành công
	mockAdapter.On("PushEmployee", ctx, employees[1]).Return(errors.New("card invalid")) // thất bại
	mockAdapter.On("Disconnect", ctx).Return(nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-4").Return(device, nil)

	empRepo := new(MockEmployeeRepository)
	empRepo.On("ListActive", ctx).Return(employees, nil)

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewEmployeeService(empRepo, deviceRepo, historyRepo, factory, new(MockEmployeeDeviceMappingRepository), new(MockDeviceCommandRepository))

	hist, err := svc.PushEmployeesToDevice(ctx, "dev-4", entity.SyncTriggerManual)

	assert.NoError(t, err) // partial không trả error ở level service
	assert.Equal(t, entity.SyncStatusPartial, hist.Status)
	assert.Equal(t, 1, hist.RecordCount) // chỉ 1 thành công
	assert.NotEmpty(t, hist.ErrorMessage)
}
