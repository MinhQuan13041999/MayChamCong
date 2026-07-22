package test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/usecase"
)

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_CreateDevice_Success
// Trường hợp: tạo thiết bị hợp lệ → repo.Create được gọi, trả về nil
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_CreateDevice_Success(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("Create", ctx, mock.Anything).Return(nil)

	factory := &MockDeviceAdapterFactory{}

	svc := usecase.NewDeviceService(deviceRepo, factory)

	d := &entity.Device{
		Name:       "Device Kho A",
		DeviceType: entity.DeviceTypeZKTeco,
		IPAddress:  "192.168.1.100",
		Port:       4370,
		Location:   "Kho A",
	}
	err := svc.CreateDevice(ctx, d)

	assert.NoError(t, err)
	assert.Equal(t, "offline", d.Status) // service phải set status=offline khi tạo mới
	deviceRepo.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_CreateDevice_MissingName
// Trường hợp: thiếu Name → validation fail, không gọi repo
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_CreateDevice_MissingName(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	factory := &MockDeviceAdapterFactory{}

	svc := usecase.NewDeviceService(deviceRepo, factory)

	d := &entity.Device{
		IPAddress: "192.168.1.100",
		Port:      4370,
	}
	err := svc.CreateDevice(ctx, d)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
	deviceRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_CreateDevice_MissingIP
// Trường hợp: thiếu IPAddress → validation fail
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_CreateDevice_MissingIP(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	factory := &MockDeviceAdapterFactory{}

	svc := usecase.NewDeviceService(deviceRepo, factory)

	d := &entity.Device{Name: "Device Kho B"}
	err := svc.CreateDevice(ctx, d)

	assert.Error(t, err)
	deviceRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_TestConnection_Online
// Trường hợp: kết nối thành công → UpdateStatus "online", trả DeviceStatus.Online=true
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_TestConnection_Online(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-online",
		DeviceType: entity.DeviceTypeZKTeco,
		IPAddress:  "192.168.1.100",
		Port:       4370,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("CheckStatus", ctx).Return(port.DeviceStatus{Online: true, FirmwareInfo: "Ver 6.60"}, nil)
	mockAdapter.On("Disconnect", ctx).Return(nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-online").Return(device, nil)
	deviceRepo.On("UpdateStatus", ctx, "dev-online", "online", mock.Anything).Return(nil)

	svc := usecase.NewDeviceService(deviceRepo, factory)

	status, err := svc.TestConnection(ctx, "dev-online")

	assert.NoError(t, err)
	assert.True(t, status.Online)
	assert.Equal(t, "Ver 6.60", status.FirmwareInfo)
	deviceRepo.AssertCalled(t, "UpdateStatus", ctx, "dev-online", "online", mock.Anything)
	mockAdapter.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_TestConnection_Offline
// Trường hợp: Connect thất bại → UpdateStatus "offline", trả error
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_TestConnection_Offline(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-offline",
		DeviceType: entity.DeviceTypeHikvision,
		IPAddress:  "10.0.0.99",
		Port:       80,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(errors.New("connection refused"))

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-offline").Return(device, nil)
	deviceRepo.On("UpdateStatus", ctx, "dev-offline", "offline", mock.Anything).Return(nil)

	svc := usecase.NewDeviceService(deviceRepo, factory)

	_, err := svc.TestConnection(ctx, "dev-offline")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	deviceRepo.AssertCalled(t, "UpdateStatus", ctx, "dev-offline", "offline", mock.Anything)
	mockAdapter.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_TestConnection_DeviceNotFound
// Trường hợp: deviceID không tồn tại → trả error ngay, không gọi adapter
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_TestConnection_DeviceNotFound(t *testing.T) {
	ctx := context.Background()

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "nonexistent").Return((*entity.Device)(nil), errors.New("not found"))

	factory := &MockDeviceAdapterFactory{}

	svc := usecase.NewDeviceService(deviceRepo, factory)

	_, err := svc.TestConnection(ctx, "nonexistent")

	assert.Error(t, err)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_RebootDevice_Success
// Trường hợp: reboot thành công → Connect → Reboot → Disconnect
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_RebootDevice_Success(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-reboot",
		DeviceType: entity.DeviceTypeSunbeam,
		IPAddress:  "192.168.2.10",
		Port:       80,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(nil)
	mockAdapter.On("Reboot", ctx).Return(nil)
	mockAdapter.On("Disconnect", ctx).Return(nil)

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-reboot").Return(device, nil)

	svc := usecase.NewDeviceService(deviceRepo, factory)

	err := svc.RebootDevice(ctx, "dev-reboot")

	assert.NoError(t, err)
	mockAdapter.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_RebootDevice_ConnectFail
// Trường hợp: Connect thất bại → Reboot không được gọi, trả error
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_RebootDevice_ConnectFail(t *testing.T) {
	ctx := context.Background()

	device := &entity.Device{
		ID:         "dev-noreboot",
		DeviceType: entity.DeviceTypeZKTeco,
		IPAddress:  "192.168.1.200",
		Port:       4370,
	}

	mockAdapter := new(MockDeviceAdapter)
	mockAdapter.On("Connect", ctx, mock.Anything).Return(errors.New("timeout"))

	factory := &MockDeviceAdapterFactory{Adapter: mockAdapter}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("GetByID", ctx, "dev-noreboot").Return(device, nil)

	svc := usecase.NewDeviceService(deviceRepo, factory)

	err := svc.RebootDevice(ctx, "dev-noreboot")

	assert.Error(t, err)
	mockAdapter.AssertNotCalled(t, "Reboot", mock.Anything)
}

// ──────────────────────────────────────────────────────────────────────────────
// TestDeviceService_ListDevices
// Trường hợp: list thành công → trả danh sách
// ──────────────────────────────────────────────────────────────────────────────
func TestDeviceService_ListDevices(t *testing.T) {
	ctx := context.Background()

	expected := []entity.Device{
		{ID: "d1", Name: "ZKTeco Kho A", DeviceType: entity.DeviceTypeZKTeco},
		{ID: "d2", Name: "Hikvision Cổng chính", DeviceType: entity.DeviceTypeHikvision},
	}

	deviceRepo := new(MockDeviceRepository)
	deviceRepo.On("List", ctx).Return(expected, nil)

	factory := &MockDeviceAdapterFactory{}
	svc := usecase.NewDeviceService(deviceRepo, factory)

	result, err := svc.ListDevices(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "ZKTeco Kho A", result[0].Name)
}


