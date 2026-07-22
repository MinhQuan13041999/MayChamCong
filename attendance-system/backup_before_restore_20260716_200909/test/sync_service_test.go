package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/usecase"
)

func TestSyncService_SyncAttendance_Success(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{ID: "dev-1", DeviceType: entity.DeviceTypeZKTeco, IPAddress: "192.168.1.100", Port: 4370}
	logs := []entity.AttendanceLog{
		{EmployeeCode: "EMP001", CheckTime: time.Now(), CheckType: entity.CheckTypeIn, VerifyMode: entity.VerifyModeFingerprint},
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("Disconnect", ctx).Return(nil)
	mockAdapter.On("GetAttendanceLogs", ctx, mock.Anything, mock.Anything).Return(logs, nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-1").Return(device, nil)
	deviceRepo.On("UpdateStatus", ctx, "dev-1", "online", mock.Anything).Return(nil)

	attendanceRepo := new(MockAttendanceLogRepository)
	attendanceRepo.On("BulkInsert", ctx, mock.Anything).Return(1, nil)

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewSyncService(deviceRepo, attendanceRepo, historyRepo, factory)

	hist, err := svc.SyncAttendance(ctx, "dev-1", time.Now().Add(-time.Hour), time.Now(), entity.SyncTriggerManual)

	assert.NoError(t, err)
	assert.Equal(t, entity.SyncStatusSuccess, hist.Status)
	assert.Equal(t, 1, hist.RecordCount)

	mockAdapter.AssertExpectations(t)
	deviceRepo.AssertExpectations(t)
	attendanceRepo.AssertExpectations(t)
}

func TestSyncService_SyncAttendance_ConnectFailure(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{ID: "dev-2", DeviceType: entity.DeviceTypeHikvision, IPAddress: "192.168.1.200", Port: 80}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(assertError("connection refused"))

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-2").Return(device, nil)
	deviceRepo.On("UpdateStatus", ctx, "dev-2", "offline", mock.Anything).Return(nil)

	attendanceRepo := new(MockAttendanceLogRepository)

	historyRepo := new(MockSyncHistoryRepository)
	historyRepo.On("Create", ctx, mock.Anything).Return(nil)
	historyRepo.On("Update", ctx, mock.Anything).Return(nil)

	svc := usecase.NewSyncService(deviceRepo, attendanceRepo, historyRepo, factory)

	hist, err := svc.SyncAttendance(ctx, "dev-2", time.Now().Add(-time.Hour), time.Now(), entity.SyncTriggerScheduled)

	assert.Error(t, err)
	assert.Equal(t, entity.SyncStatusFailed, hist.Status)
}

// --- helpers ---

func assertError(msg string) error {
	return &simpleError{msg}
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

var _ port.DeviceAdapter = (*MockDeviceAdapter)(nil)
