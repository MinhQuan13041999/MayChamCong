package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

type syncDeviceRepoStub struct {
	device       *entity.Device
	updatedID    string
	updatedState string
}

func (s *syncDeviceRepoStub) Create(context.Context, *entity.Device) error { return nil }
func (s *syncDeviceRepoStub) Update(context.Context, *entity.Device) error { return nil }
func (s *syncDeviceRepoStub) Delete(context.Context, string) error         { return nil }
func (s *syncDeviceRepoStub) GetByID(context.Context, string) (*entity.Device, error) {
	return s.device, nil
}
func (s *syncDeviceRepoStub) GetBySerialADMS(context.Context, string) (*entity.Device, error) {
	return nil, nil
}
func (s *syncDeviceRepoStub) List(context.Context) ([]entity.Device, error) {
	if s.device == nil {
		return nil, nil
	}
	return []entity.Device{*s.device}, nil
}
func (s *syncDeviceRepoStub) UpdateStatus(_ context.Context, id, status string, _ time.Time) error {
	s.updatedID = id
	s.updatedState = status
	return nil
}
func (s *syncDeviceRepoStub) UpdateHeartbeat(context.Context, string, time.Time) error { return nil }

var _ port.DeviceRepository = (*syncDeviceRepoStub)(nil)

type syncAttendanceRepoStub struct {
	logs []entity.AttendanceLog
}

func (s *syncAttendanceRepoStub) BulkInsert(_ context.Context, logs []entity.AttendanceLog) (int, error) {
	s.logs = append(s.logs, logs...)
	return len(logs), nil
}
func (s *syncAttendanceRepoStub) Query(context.Context, time.Time, time.Time, string, string) ([]entity.AttendanceLog, error) {
	return nil, nil
}
func (s *syncAttendanceRepoStub) UpdateValidity(context.Context, int64, bool, string) error {
	return nil
}
func (s *syncAttendanceRepoStub) UpdateClassification(context.Context, int64, bool, string, string, *string) error {
	return nil
}

var _ port.AttendanceLogRepository = (*syncAttendanceRepoStub)(nil)

type syncHistoryRepoStub struct {
	created *entity.SyncHistory
	updated *entity.SyncHistory
}

func (s *syncHistoryRepoStub) Create(_ context.Context, h *entity.SyncHistory) error {
	s.created = h
	return nil
}
func (s *syncHistoryRepoStub) Update(_ context.Context, h *entity.SyncHistory) error {
	copy := *h
	s.updated = &copy
	return nil
}
func (s *syncHistoryRepoStub) List(context.Context, string, string) ([]entity.SyncHistory, error) {
	return nil, nil
}
func (s *syncHistoryRepoStub) GetByID(context.Context, string) (*entity.SyncHistory, error) {
	return nil, nil
}

var _ port.SyncHistoryRepository = (*syncHistoryRepoStub)(nil)

type syncMappingRepoStub struct {
	mapping *entity.EmployeeDeviceMapping
}

func (s *syncMappingRepoStub) Upsert(context.Context, *entity.EmployeeDeviceMapping) error {
	return nil
}
func (s *syncMappingRepoStub) ListByEmployee(context.Context, string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *syncMappingRepoStub) ListByDevice(context.Context, string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *syncMappingRepoStub) GetByEmployeeAndDevice(context.Context, string, string) (*entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *syncMappingRepoStub) GetByDeviceUserID(_ context.Context, _, deviceUserID string) (*entity.EmployeeDeviceMapping, error) {
	if s.mapping != nil && s.mapping.DeviceUserID == deviceUserID {
		return s.mapping, nil
	}
	return nil, nil
}
func (s *syncMappingRepoStub) MarkFingerprintEnrolled(context.Context, string, string, time.Time) error {
	return nil
}

var _ port.EmployeeDeviceMappingRepository = (*syncMappingRepoStub)(nil)

type syncAdapterStub struct {
	connected    bool
	disconnected bool
	getLogCalls  int
	logs         []entity.AttendanceLog
}

func (s *syncAdapterStub) Connect(context.Context, port.DeviceConfig) error {
	s.connected = true
	return nil
}
func (s *syncAdapterStub) Disconnect(context.Context) error {
	s.disconnected = true
	return nil
}
func (s *syncAdapterStub) CheckStatus(context.Context) (port.DeviceStatus, error) {
	return port.DeviceStatus{}, nil
}
func (s *syncAdapterStub) SyncTime(context.Context) error { return nil }
func (s *syncAdapterStub) GetEmployees(context.Context) ([]entity.Employee, error) {
	return nil, nil
}
func (s *syncAdapterStub) PushEmployee(context.Context, entity.Employee) error { return nil }
func (s *syncAdapterStub) DeleteEmployee(context.Context, string) error        { return nil }
func (s *syncAdapterStub) PushFingerprint(context.Context, string, entity.EmployeeFingerprint) error {
	return nil
}
func (s *syncAdapterStub) GetFingerprint(context.Context, string, int) (*entity.EmployeeFingerprint, error) {
	return nil, nil
}
func (s *syncAdapterStub) DeleteFingerprint(context.Context, string, int) error { return nil }
func (s *syncAdapterStub) GetAttendanceLogs(context.Context, time.Time, time.Time) ([]entity.AttendanceLog, error) {
	s.getLogCalls++
	return s.logs, nil
}
func (s *syncAdapterStub) ClearAttendanceLogs(context.Context) error { return nil }
func (s *syncAdapterStub) ClearEmployees(context.Context) error      { return nil }
func (s *syncAdapterStub) Reboot(context.Context) error              { return nil }
func (s *syncAdapterStub) Reset(context.Context) error               { return nil }
func (s *syncAdapterStub) EnrollFingerprint(context.Context, string, int) error {
	return nil
}

var _ port.DeviceAdapter = (*syncAdapterStub)(nil)

type syncAdapterFactoryStub struct {
	adapter port.DeviceAdapter
	calls   int
	err     error
}

func (s *syncAdapterFactoryStub) NewAdapter(entity.DeviceType) (port.DeviceAdapter, error) {
	s.calls++
	return s.adapter, s.err
}

func TestSyncAttendanceSDKFailureKeepsRecentADMSDeviceOnline(t *testing.T) {
	heartbeat := time.Now()
	device := &entity.Device{
		ID:              "dev-adms-online",
		DeviceType:      entity.DeviceTypeZKTeco,
		ADMSEnabled:     true,
		LastHeartbeatAt: &heartbeat,
	}
	deviceRepo := &syncDeviceRepoStub{device: device}
	service := NewSyncService(
		deviceRepo,
		&syncAttendanceRepoStub{},
		&syncHistoryRepoStub{},
		&syncAdapterFactoryStub{err: errors.New("SDK unavailable")},
	)

	_, err := service.SyncAttendance(
		context.Background(), device.ID, time.Now().Add(-time.Hour), time.Now(), entity.SyncTriggerManual,
	)
	if err == nil {
		t.Fatal("SyncAttendance returned nil error, want SDK failure")
	}
	if deviceRepo.updatedState == "offline" {
		t.Fatal("recent ADMS heartbeat was overwritten to offline by an SDK-only failure")
	}
}

var _ port.DeviceAdapterFactory = (*syncAdapterFactoryStub)(nil)

func TestSyncAttendancePullsSDKLogsWhenADMSIsAlsoEnabled(t *testing.T) {
	checkTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	device := &entity.Device{
		ID:          "dev-adms-sdk",
		Name:        "ZKTeco Phong Chinh",
		DeviceType:  entity.DeviceTypeZKTeco,
		IPAddress:   "192.168.11.151",
		Port:        4370,
		ADMSEnabled: true,
	}
	deviceRepo := &syncDeviceRepoStub{device: device}
	attendanceRepo := &syncAttendanceRepoStub{}
	historyRepo := &syncHistoryRepoStub{}
	adapter := &syncAdapterStub{logs: []entity.AttendanceLog{{
		EmployeeCode: "12",
		CheckTime:    checkTime,
		CheckType:    entity.CheckTypeIn,
		VerifyMode:   entity.VerifyModeFingerprint,
	}}}
	factory := &syncAdapterFactoryStub{adapter: adapter}
	mappingRepo := &syncMappingRepoStub{mapping: &entity.EmployeeDeviceMapping{
		DeviceID:     device.ID,
		DeviceUserID: "12",
		EmployeeCode: "NV012",
	}}
	service := NewSyncServiceWithCursor(deviceRepo, attendanceRepo, historyRepo, nil, factory, mappingRepo)

	history, err := service.SyncAttendance(
		context.Background(),
		device.ID,
		checkTime.Add(-time.Hour),
		checkTime.Add(time.Hour),
		entity.SyncTriggerManual,
	)
	if err != nil {
		t.Fatalf("SyncAttendance returned error: %v", err)
	}
	if factory.calls != 1 || !adapter.connected || !adapter.disconnected || adapter.getLogCalls != 1 {
		t.Fatalf("SDK adapter calls = factory:%d connected:%t disconnected:%t getLogs:%d, want 1/true/true/1",
			factory.calls, adapter.connected, adapter.disconnected, adapter.getLogCalls)
	}
	if len(attendanceRepo.logs) != 1 {
		t.Fatalf("stored logs = %d, want 1", len(attendanceRepo.logs))
	}
	stored := attendanceRepo.logs[0]
	if stored.DeviceID != device.ID || stored.EmployeeCode != "NV012" || !stored.CheckTime.Equal(checkTime) {
		t.Fatalf("stored log = %+v, want mapped employee NV012 on device %s", stored, device.ID)
	}
	if history.Status != entity.SyncStatusSuccess || history.RecordCount != 1 {
		t.Fatalf("history = %+v, want success with one record", history)
	}
	if historyRepo.updated == nil || historyRepo.updated.Status != entity.SyncStatusSuccess {
		t.Fatalf("sync history was not updated to success: %+v", historyRepo.updated)
	}
	if deviceRepo.updatedID != device.ID || deviceRepo.updatedState != "online" {
		t.Fatalf("device status = %s/%s, want %s/online", deviceRepo.updatedID, deviceRepo.updatedState, device.ID)
	}
}

func TestSyncAttendanceResolvesZeroPaddedSDKPin(t *testing.T) {
	checkTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	device := &entity.Device{ID: "dev-sdk-pin", DeviceType: entity.DeviceTypeZKTeco, ADMSEnabled: false}
	deviceRepo := &syncDeviceRepoStub{device: device}
	attendanceRepo := &syncAttendanceRepoStub{}
	historyRepo := &syncHistoryRepoStub{}
	adapter := &syncAdapterStub{logs: []entity.AttendanceLog{{
		EmployeeCode: "8",
		CheckTime:    checkTime,
		CheckType:    entity.CheckTypeIn,
		VerifyMode:   entity.VerifyModeFingerprint,
	}}}
	mappingRepo := &syncMappingRepoStub{mapping: &entity.EmployeeDeviceMapping{
		DeviceID: device.ID, DeviceUserID: "08", EmployeeCode: "NV008",
	}}
	service := NewSyncServiceWithCursor(deviceRepo, attendanceRepo, historyRepo, nil, &syncAdapterFactoryStub{adapter: adapter}, mappingRepo)

	history, err := service.SyncAttendance(context.Background(), device.ID, checkTime.Add(-time.Hour), checkTime.Add(time.Hour), entity.SyncTriggerManual)
	if err != nil {
		t.Fatalf("SyncAttendance returned error: %v", err)
	}
	if len(attendanceRepo.logs) != 1 || attendanceRepo.logs[0].EmployeeCode != "NV008" {
		t.Fatalf("stored logs = %+v, want zero-padded SDK PIN mapped to NV008", attendanceRepo.logs)
	}
	if history.Status != entity.SyncStatusSuccess {
		t.Fatalf("history = %+v, want success", history)
	}
}
