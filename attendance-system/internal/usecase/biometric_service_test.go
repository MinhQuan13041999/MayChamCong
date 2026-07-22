package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

type stubCommandRepo struct {
	queued []string
}

func (s *stubCommandRepo) Enqueue(_ context.Context, _ string, command string) (*entity.DeviceCommandQueue, error) {
	s.queued = append(s.queued, command)
	return &entity.DeviceCommandQueue{Command: command}, nil
}

func (s *stubCommandRepo) GetPending(_ context.Context, _ string) ([]entity.DeviceCommandQueue, error) {
	return nil, nil
}

func (s *stubCommandRepo) GetByDeviceIDAndCommandID(_ context.Context, _ string, _ int64) (*entity.DeviceCommandQueue, error) {
	return nil, nil
}

func (s *stubCommandRepo) MarkSent(_ context.Context, _ int64) error      { return nil }
func (s *stubCommandRepo) Ack(_ context.Context, _ string, _ int64) error { return nil }
func (s *stubCommandRepo) MarkFailed(_ context.Context, _ int64) error    { return nil }
func (s *stubCommandRepo) MarkFailedByDeviceCmdID(_ context.Context, _ string, _ int64) error {
	return nil
}
func (s *stubCommandRepo) CancelPendingByDevice(_ context.Context, _ string) (int, error) {
	return 0, nil
}

var _ port.DeviceCommandRepository = (*stubCommandRepo)(nil)

type stubDeviceRepo struct {
	device *entity.Device
}

func (s *stubDeviceRepo) Create(_ context.Context, _ *entity.Device) error { return nil }
func (s *stubDeviceRepo) Update(_ context.Context, _ *entity.Device) error { return nil }
func (s *stubDeviceRepo) Delete(_ context.Context, _ string) error         { return nil }
func (s *stubDeviceRepo) GetByID(_ context.Context, _ string) (*entity.Device, error) {
	return s.device, nil
}
func (s *stubDeviceRepo) GetBySerialADMS(_ context.Context, _ string) (*entity.Device, error) {
	return s.device, nil
}
func (s *stubDeviceRepo) List(_ context.Context) ([]entity.Device, error) {
	return []entity.Device{*s.device}, nil
}
func (s *stubDeviceRepo) UpdateStatus(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}
func (s *stubDeviceRepo) UpdateHeartbeat(_ context.Context, _ string, _ time.Time) error { return nil }

var _ port.DeviceRepository = (*stubDeviceRepo)(nil)

type biometricStubEmployeeRepo struct {
	employee *entity.Employee
}

func (s *biometricStubEmployeeRepo) Create(_ context.Context, _ *entity.Employee) error { return nil }
func (s *biometricStubEmployeeRepo) Update(_ context.Context, _ *entity.Employee) error { return nil }
func (s *biometricStubEmployeeRepo) Delete(_ context.Context, _ string) error           { return nil }
func (s *biometricStubEmployeeRepo) DeleteAll(_ context.Context) (int64, error)         { return 0, nil }
func (s *biometricStubEmployeeRepo) GetByID(_ context.Context, _ string) (*entity.Employee, error) {
	return s.employee, nil
}
func (s *biometricStubEmployeeRepo) GetByCode(_ context.Context, _ string) (*entity.Employee, error) {
	return nil, nil
}
func (s *biometricStubEmployeeRepo) List(_ context.Context) ([]entity.Employee, error) {
	return nil, nil
}
func (s *biometricStubEmployeeRepo) ListActive(_ context.Context) ([]entity.Employee, error) {
	return nil, nil
}

var _ port.EmployeeRepository = (*biometricStubEmployeeRepo)(nil)

type biometricStubMappingRepo struct{}

func (s *biometricStubMappingRepo) Upsert(_ context.Context, _ *entity.EmployeeDeviceMapping) error {
	return nil
}
func (s *biometricStubMappingRepo) ListByEmployee(_ context.Context, _ string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *biometricStubMappingRepo) ListByDevice(_ context.Context, _ string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *biometricStubMappingRepo) GetByEmployeeAndDevice(_ context.Context, _, _ string) (*entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *biometricStubMappingRepo) GetByDeviceUserID(_ context.Context, _, _ string) (*entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *biometricStubMappingRepo) MarkFingerprintEnrolled(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

var _ port.EmployeeDeviceMappingRepository = (*biometricStubMappingRepo)(nil)

type biometricStubFingerprintRepo struct {
	fingerprints []entity.EmployeeFingerprint
}

func (s *biometricStubFingerprintRepo) Upsert(_ context.Context, _ *entity.EmployeeFingerprint) error {
	return nil
}
func (s *biometricStubFingerprintRepo) ListByEmployee(_ context.Context, _ string) ([]entity.EmployeeFingerprint, error) {
	return s.fingerprints, nil
}
func (s *biometricStubFingerprintRepo) GetByEmployeeAndFinger(_ context.Context, _ string, _ int) (*entity.EmployeeFingerprint, error) {
	return nil, nil
}
func (s *biometricStubFingerprintRepo) Delete(_ context.Context, _ string, _ int) error { return nil }

var _ port.FingerprintRepository = (*biometricStubFingerprintRepo)(nil)

type sdkPushAdapterStub struct {
	connected      bool
	disconnected   bool
	pushedEmployee entity.Employee
	pushedFingers  []entity.EmployeeFingerprint
	pushedPin      string
}

func (s *sdkPushAdapterStub) Connect(_ context.Context, _ port.DeviceConfig) error {
	s.connected = true
	return nil
}
func (s *sdkPushAdapterStub) Disconnect(_ context.Context) error { s.disconnected = true; return nil }
func (s *sdkPushAdapterStub) CheckStatus(_ context.Context) (port.DeviceStatus, error) {
	return port.DeviceStatus{}, nil
}
func (s *sdkPushAdapterStub) SyncTime(_ context.Context) error { return nil }
func (s *sdkPushAdapterStub) GetEmployees(_ context.Context) ([]entity.Employee, error) {
	return nil, nil
}
func (s *sdkPushAdapterStub) PushEmployee(_ context.Context, employee entity.Employee) error {
	s.pushedEmployee = employee
	return nil
}
func (s *sdkPushAdapterStub) DeleteEmployee(_ context.Context, _ string) error { return nil }
func (s *sdkPushAdapterStub) PushFingerprint(_ context.Context, pin string, fp entity.EmployeeFingerprint) error {
	s.pushedPin = pin
	s.pushedFingers = append(s.pushedFingers, fp)
	return nil
}
func (s *sdkPushAdapterStub) GetFingerprint(_ context.Context, _ string, _ int) (*entity.EmployeeFingerprint, error) {
	return nil, nil
}
func (s *sdkPushAdapterStub) DeleteFingerprint(_ context.Context, _ string, _ int) error { return nil }
func (s *sdkPushAdapterStub) GetAttendanceLogs(_ context.Context, _, _ time.Time) ([]entity.AttendanceLog, error) {
	return nil, nil
}
func (s *sdkPushAdapterStub) ClearAttendanceLogs(_ context.Context) error { return nil }
func (s *sdkPushAdapterStub) ClearEmployees(_ context.Context) error      { return nil }
func (s *sdkPushAdapterStub) Reboot(_ context.Context) error              { return nil }
func (s *sdkPushAdapterStub) Reset(_ context.Context) error               { return nil }
func (s *sdkPushAdapterStub) EnrollFingerprint(_ context.Context, _ string, _ int) error {
	return nil
}

var _ port.DeviceAdapter = (*sdkPushAdapterStub)(nil)

type sdkPushFactoryStub struct{ adapter port.DeviceAdapter }

func (s *sdkPushFactoryStub) NewAdapter(_ entity.DeviceType) (port.DeviceAdapter, error) {
	return s.adapter, nil
}

var _ port.DeviceAdapterFactory = (*sdkPushFactoryStub)(nil)

func TestEnrollFingerprintQueuesEnrollCommandForADMSDevice(t *testing.T) {
	cmdRepo := &stubCommandRepo{}
	devRepo := &stubDeviceRepo{device: &entity.Device{ID: "dev1", Name: "ADMS", SerialNumberADMS: "SN1", ADMSEnabled: true, LastHeartbeatAt: &[]time.Time{time.Now()}[0]}}
	employeeRepo := &biometricStubEmployeeRepo{employee: &entity.Employee{ID: "emp1", EmployeeCode: "001", FullName: "Test", CardNo: "123"}}

	svc := NewBiometricService(nil, devRepo, cmdRepo, employeeRepo, &biometricStubMappingRepo{}, nil)
	if err := svc.EnrollFingerprint(context.Background(), "emp1", "dev1", 0); err != nil {
		t.Fatalf("EnrollFingerprint returned error: %v", err)
	}

	if len(cmdRepo.queued) == 0 {
		t.Fatalf("expected at least one queued command")
	}
	foundEnroll := false
	for _, cmd := range cmdRepo.queued {
		if cmd == buildADMSEnrollCommand("001", 0) {
			foundEnroll = true
			break
		}
	}
	if !foundEnroll {
		t.Fatalf("expected queued command to include enroll command, got %v", cmdRepo.queued)
	}
}

func TestReEnrollFingerprintQueuesOverwriteAndKeepsSavedTemplate(t *testing.T) {
	cmdRepo := &stubCommandRepo{}
	devRepo := &stubDeviceRepo{device: &entity.Device{ID: "dev1", Name: "ADMS", SerialNumberADMS: "SN1", ADMSEnabled: true}}
	employeeRepo := &biometricStubEmployeeRepo{employee: &entity.Employee{ID: "emp1", EmployeeCode: "001", FullName: "Test", FingerprintEnrolled: true}}
	fingerprintRepo := &biometricStubFingerprintRepo{fingerprints: []entity.EmployeeFingerprint{{EmployeeID: "emp1", FingerIndex: 0, TemplateData: "last-known-good"}}}

	svc := NewBiometricService(fingerprintRepo, devRepo, cmdRepo, employeeRepo, &biometricStubMappingRepo{}, nil)
	if err := svc.ReEnrollFingerprint(context.Background(), "emp1", "dev1", 0); err != nil {
		t.Fatalf("ReEnrollFingerprint returned error: %v", err)
	}

	if len(cmdRepo.queued) != 2 {
		t.Fatalf("expected USER and overwrite enrollment commands, got %v", cmdRepo.queued)
	}
	if !strings.Contains(cmdRepo.queued[1], "ENROLL_FP") || !strings.Contains(cmdRepo.queued[1], "OVERWRITE=1") {
		t.Fatalf("re-enroll command must overwrite finger 0, got %q", cmdRepo.queued[1])
	}
	if len(fingerprintRepo.fingerprints) != 1 || fingerprintRepo.fingerprints[0].TemplateData != "last-known-good" {
		t.Fatalf("saved template must remain until the replacement arrives, got %+v", fingerprintRepo.fingerprints)
	}
}

func TestPropagateToAllDevicesPushesTemplateViaSDKForStandaloneDevice(t *testing.T) {
	adapter := &sdkPushAdapterStub{}
	device := &entity.Device{ID: "dev-sdk", Name: "SDK device", DeviceType: entity.DeviceTypeZKTeco, IPAddress: "192.168.1.10", Port: 4370}
	service := NewBiometricService(
		&biometricStubFingerprintRepo{fingerprints: []entity.EmployeeFingerprint{{EmployeeID: "emp1", FingerIndex: 2, TemplateData: "template"}}},
		&stubDeviceRepo{device: device},
		nil,
		&biometricStubEmployeeRepo{employee: &entity.Employee{ID: "emp1", EmployeeCode: "1001", FullName: "Test"}},
		&biometricStubMappingRepo{},
		&sdkPushFactoryStub{adapter: adapter},
	)

	if err := service.PropagateToAllDevices(context.Background(), "emp1"); err != nil {
		t.Fatalf("PropagateToAllDevices returned error: %v", err)
	}
	if !adapter.connected || !adapter.disconnected {
		t.Fatalf("expected SDK adapter Connect and Disconnect, got connected=%t disconnected=%t", adapter.connected, adapter.disconnected)
	}
	if adapter.pushedEmployee.EmployeeCode != "1001" {
		t.Fatalf("employee pushed with code %q, want device user ID 1001", adapter.pushedEmployee.EmployeeCode)
	}
	if adapter.pushedPin != "1001" || len(adapter.pushedFingers) != 1 || adapter.pushedFingers[0].FingerIndex != 2 {
		t.Fatalf("unexpected fingerprint SDK calls: pin=%q fingerprints=%+v", adapter.pushedPin, adapter.pushedFingers)
	}
}

func TestBatchEnrollQueuesUserThenEnrollCommandForEachEmployee(t *testing.T) {
	cmdRepo := &stubCommandRepo{}
	// ADMS commands stay in the queue until the terminal polls them. A stale
	// heartbeat must not prevent a batch request while single enrollment works.
	devRepo := &stubDeviceRepo{device: &entity.Device{ID: "dev1", Name: "ADMS", SerialNumberADMS: "SN1", ADMSEnabled: true}}
	employeeRepo := &biometricStubEmployeeRepo{employee: &entity.Employee{ID: "emp1", EmployeeCode: "001", FullName: "Test"}}

	svc := NewBiometricService(nil, devRepo, cmdRepo, employeeRepo, &biometricStubMappingRepo{}, nil)
	enqueued, errors, err := svc.BatchEnroll(context.Background(), []string{"emp1"}, "dev1")
	if err != nil {
		t.Fatalf("BatchEnroll returned error: %v", err)
	}
	if len(errors) != 0 || enqueued != 1 {
		t.Fatalf("BatchEnroll = (%d, %v), want (1, no errors)", enqueued, errors)
	}
	if len(cmdRepo.queued) != 2 {
		t.Fatalf("expected USER and ENROLL_FP commands, got %v", cmdRepo.queued)
	}
	if cmdRepo.queued[0] != buildADMSUserCommand("001", "Test", "") {
		t.Fatalf("first command = %q, want USER update", cmdRepo.queued[0])
	}
	if cmdRepo.queued[1] != buildADMSEnrollCommand("001", 0) {
		t.Fatalf("second command = %q, want ENROLL_FP", cmdRepo.queued[1])
	}
}
