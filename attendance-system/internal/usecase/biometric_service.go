package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

// sdkEnrollmentScanWindow is measured from the moment StartEnrollEx has
// successfully opened the terminal's native capture screen. No SDK read/cache
// operation is allowed during this window because those calls close the screen
// on several ZKTeco TFT firmwares.
const sdkEnrollmentScanWindow = 10 * time.Second

type BiometricService struct {
	fingerprintRepo port.FingerprintRepository
	deviceRepo      port.DeviceRepository
	commandRepo     port.DeviceCommandRepository
	employeeRepo    port.EmployeeRepository
	mappingRepo     port.EmployeeDeviceMappingRepository
	factory         port.DeviceAdapterFactory

	// A ZKTeco terminal can display and capture only one enrollment at a time.
	// Keep this state in the service so a second click cannot replace the
	// person currently shown on the terminal.
	sdkEnrollmentMu     sync.Mutex
	sdkEnrollmentActive map[string]bool
	sdkEnrollmentCancel map[string]context.CancelFunc
	sdkEnrollmentDone   map[string]chan struct{}
}

func NewBiometricService(
	fingerprintRepo port.FingerprintRepository,
	deviceRepo port.DeviceRepository,
	commandRepo port.DeviceCommandRepository,
	employeeRepo port.EmployeeRepository,
	mappingRepo port.EmployeeDeviceMappingRepository,
	factory port.DeviceAdapterFactory,
) *BiometricService {
	return &BiometricService{
		fingerprintRepo:     fingerprintRepo,
		deviceRepo:          deviceRepo,
		commandRepo:         commandRepo,
		employeeRepo:        employeeRepo,
		mappingRepo:         mappingRepo,
		factory:             factory,
		sdkEnrollmentActive: make(map[string]bool),
		sdkEnrollmentCancel: make(map[string]context.CancelFunc),
		sdkEnrollmentDone:   make(map[string]chan struct{}),
	}
}

// beginSDKEnrollment reserves the terminal for one interactive enrollment
// flow. It intentionally covers both the single-person and batch paths.
func (s *BiometricService) beginSDKEnrollment(deviceID string) bool {
	s.sdkEnrollmentMu.Lock()
	defer s.sdkEnrollmentMu.Unlock()
	if s.sdkEnrollmentActive == nil {
		s.sdkEnrollmentActive = make(map[string]bool)
	}
	if s.sdkEnrollmentDone == nil {
		s.sdkEnrollmentDone = make(map[string]chan struct{})
	}
	if s.sdkEnrollmentActive[deviceID] {
		return false
	}
	s.sdkEnrollmentActive[deviceID] = true
	s.sdkEnrollmentDone[deviceID] = make(chan struct{})
	return true
}

func (s *BiometricService) endSDKEnrollment(deviceID string) {
	s.sdkEnrollmentMu.Lock()
	defer s.sdkEnrollmentMu.Unlock()
	delete(s.sdkEnrollmentActive, deviceID)
	delete(s.sdkEnrollmentCancel, deviceID)
	if done := s.sdkEnrollmentDone[deviceID]; done != nil {
		close(done)
		delete(s.sdkEnrollmentDone, deviceID)
	}
}

func (s *BiometricService) setSDKEnrollmentCancel(deviceID string, cancel context.CancelFunc) {
	s.sdkEnrollmentMu.Lock()
	defer s.sdkEnrollmentMu.Unlock()
	if s.sdkEnrollmentCancel == nil {
		s.sdkEnrollmentCancel = make(map[string]context.CancelFunc)
	}
	if s.sdkEnrollmentActive[deviceID] {
		s.sdkEnrollmentCancel[deviceID] = cancel
	}
}

func (s *BiometricService) cancelSDKEnrollment(deviceID string) (<-chan struct{}, bool) {
	s.sdkEnrollmentMu.Lock()
	cancel := s.sdkEnrollmentCancel[deviceID]
	done := s.sdkEnrollmentDone[deviceID]
	s.sdkEnrollmentMu.Unlock()
	if cancel == nil {
		return nil, false
	}
	cancel()
	return done, true
}

// SaveFingerprint lưu hoặc cập nhật template vân tay và tự động đồng bộ
func (s *BiometricService) SaveFingerprint(ctx context.Context, fp *entity.EmployeeFingerprint) error {
	if err := s.fingerprintRepo.Upsert(ctx, fp); err != nil {
		return err
	}
	if emp, err := s.employeeRepo.GetByID(ctx, fp.EmployeeID); err != nil {
		return err
	} else if emp != nil && !emp.FingerprintEnrolled {
		emp.FingerprintEnrolled = true
		if err := s.employeeRepo.Update(ctx, emp); err != nil {
			return err
		}
	}

	// Kích hoạt đồng bộ ngầm sang các thiết bị khác
	go func(empID string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		_ = s.PropagateToAllDevices(bgCtx, empID)
	}(fp.EmployeeID)

	return nil
}

// BackupDeviceTemplates copies every source-device account and all available
// fingerprint templates to each selected target. SDK targets are written in a
// single connection; ADMS targets receive the equivalent queued commands.
func (s *BiometricService) BackupDeviceTemplates(ctx context.Context, srcDevID string, targetDevIDs []string) error {
	if s.deviceRepo == nil || s.mappingRepo == nil || s.employeeRepo == nil || s.fingerprintRepo == nil {
		return fmt.Errorf("backup repositories are not configured")
	}
	srcDev, err := s.deviceRepo.GetByID(ctx, srcDevID)
	if err != nil {
		return fmt.Errorf("load source device: %w", err)
	}
	if srcDev == nil {
		return fmt.Errorf("source device not found: %s", srcDevID)
	}

	mappings, err := s.mappingRepo.ListByDevice(ctx, srcDevID)
	if err != nil {
		return fmt.Errorf("load source device employees: %w", err)
	}
	if len(mappings) == 0 {
		return fmt.Errorf("source device has no employee mappings")
	}

	type sourceRecord struct {
		employee entity.Employee
		mapping  entity.EmployeeDeviceMapping
		fps      []entity.EmployeeFingerprint
	}
	records := make([]sourceRecord, 0, len(mappings))

	var sourceAdapter port.DeviceAdapter
	if !srcDev.ADMSEnabled && s.factory != nil {
		sourceAdapter, err = s.factory.NewAdapter(srcDev.DeviceType)
		if err != nil {
			return fmt.Errorf("create source SDK adapter: %w", err)
		}
		if err = sourceAdapter.Connect(ctx, port.DeviceConfig{
			IPAddress: srcDev.IPAddress,
			Port:      srcDev.Port,
			Username:  srcDev.Username,
			Password:  srcDev.Password,
			Timeout:   10 * time.Second,
		}); err != nil {
			return fmt.Errorf("connect source SDK device: %w", err)
		}
		defer sourceAdapter.Disconnect(ctx)
	}

	for _, mapping := range mappings {
		emp, getErr := s.employeeRepo.GetByID(ctx, mapping.EmployeeID)
		if getErr != nil {
			return fmt.Errorf("load employee %s: %w", mapping.EmployeeID, getErr)
		}
		if emp == nil {
			continue
		}
		fps, fpErr := s.fingerprintRepo.ListByEmployee(ctx, mapping.EmployeeID)
		if fpErr != nil {
			return fmt.Errorf("load fingerprints for employee %s: %w", mapping.EmployeeID, fpErr)
		}
		// A standalone device may contain templates that have not been pulled
		// into the central table yet. Read them once through the SDK and persist
		// them so every target receives the same complete set.
		if len(fps) == 0 && sourceAdapter != nil {
			if reader, ok := sourceAdapter.(interface {
				GetEmployeeFingerprints(context.Context, string) ([]entity.EmployeeFingerprint, error)
			}); ok {
				fps, fpErr = reader.GetEmployeeFingerprints(ctx, mapping.DeviceUserID)
			} else {
				for fingerIndex := 0; fingerIndex <= 9; fingerIndex++ {
					fp, readErr := sourceAdapter.GetFingerprint(ctx, mapping.DeviceUserID, fingerIndex)
					if readErr == nil && fp != nil && fp.TemplateData != "" {
						fps = append(fps, *fp)
					}
				}
			}
			if fpErr != nil {
				return fmt.Errorf("read source fingerprints for employee %s: %w", mapping.EmployeeID, fpErr)
			}
			for index := range fps {
				fps[index].EmployeeID = mapping.EmployeeID
				fps[index].SourceDeviceID = srcDevID
				if upsertErr := s.fingerprintRepo.Upsert(ctx, &fps[index]); upsertErr != nil {
					return fmt.Errorf("save source fingerprint for employee %s: %w", mapping.EmployeeID, upsertErr)
				}
			}
		}
		records = append(records, sourceRecord{employee: *emp, mapping: mapping, fps: fps})
	}
	if len(records) == 0 {
		return fmt.Errorf("source device has no valid employees")
	}

	seenTargets := make(map[string]struct{}, len(targetDevIDs))
	for _, targetID := range targetDevIDs {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" || targetID == srcDevID {
			continue
		}
		if _, exists := seenTargets[targetID]; exists {
			continue
		}
		seenTargets[targetID] = struct{}{}

		targetDev, getErr := s.deviceRepo.GetByID(ctx, targetID)
		if getErr != nil {
			return fmt.Errorf("load target device %s: %w", targetID, getErr)
		}
		if targetDev == nil {
			return fmt.Errorf("target device not found: %s", targetID)
		}

		var targetAdapter port.DeviceAdapter
		if !targetDev.ADMSEnabled {
			if s.factory == nil {
				return fmt.Errorf("device adapter factory is not configured")
			}
			targetAdapter, err = s.factory.NewAdapter(targetDev.DeviceType)
			if err != nil {
				return fmt.Errorf("create target SDK adapter %s: %w", targetID, err)
			}
			if err = targetAdapter.Connect(ctx, port.DeviceConfig{
				IPAddress: targetDev.IPAddress,
				Port:      targetDev.Port,
				Username:  targetDev.Username,
				Password:  targetDev.Password,
				Timeout:   10 * time.Second,
			}); err != nil {
				return fmt.Errorf("connect target SDK device %s: %w", targetDev.Name, err)
			}
		}

		copyErr := func() error {
			if targetDev.ADMSEnabled && s.commandRepo == nil {
				return fmt.Errorf("ADMS command repository is not configured")
			}
			for _, record := range records {
				targetMapping, mapErr := s.mappingRepo.GetByEmployeeAndDevice(ctx, record.employee.ID, targetID)
				if mapErr != nil {
					return fmt.Errorf("load target mapping for employee %s: %w", record.employee.ID, mapErr)
				}
				pin := record.mapping.DeviceUserID
				if targetMapping != nil && strings.TrimSpace(targetMapping.DeviceUserID) != "" {
					pin = targetMapping.DeviceUserID
				}
				if strings.TrimSpace(pin) == "" {
					pin = record.employee.EmployeeCode
				}
				if targetMapping == nil {
					targetMapping = &entity.EmployeeDeviceMapping{EmployeeID: record.employee.ID, DeviceID: targetID, DeviceUserID: pin, SyncStatus: "pending"}
				} else {
					targetMapping.DeviceUserID = pin
					targetMapping.SyncStatus = "pending"
				}
				if err := s.mappingRepo.Upsert(ctx, targetMapping); err != nil {
					return fmt.Errorf("save target mapping for employee %s: %w", record.employee.ID, err)
				}

				if targetDev.ADMSEnabled {
					if _, err := s.commandRepo.Enqueue(ctx, targetID, buildADMSUserCommand(pin, record.employee.FullName, record.employee.CardNo)); err != nil {
						return fmt.Errorf("queue employee %s to ADMS: %w", record.employee.ID, err)
					}
					for _, fp := range record.fps {
						if _, err := s.commandRepo.Enqueue(ctx, targetID, buildADMSFingerprintCommand(pin, fp.FingerIndex, fp.TemplateSize, fp.TemplateData)); err != nil {
							return fmt.Errorf("queue fingerprint %d for employee %s: %w", fp.FingerIndex, record.employee.ID, err)
						}
					}
					targetMapping.SyncStatus = "pending"
				} else {
					deviceEmployee := record.employee
					deviceEmployee.EmployeeCode = pin
					if err := targetAdapter.PushEmployee(ctx, deviceEmployee); err != nil {
						return fmt.Errorf("push employee %s to SDK: %w", record.employee.ID, err)
					}
					for _, fp := range record.fps {
						if err := targetAdapter.PushFingerprint(ctx, pin, fp); err != nil {
							return fmt.Errorf("push fingerprint %d for employee %s to SDK: %w", fp.FingerIndex, record.employee.ID, err)
						}
					}
					targetMapping.SyncStatus = "synced"
				}
				now := time.Now()
				targetMapping.LastSyncedAt = &now
				if err := s.mappingRepo.Upsert(ctx, targetMapping); err != nil {
					return fmt.Errorf("update target mapping for employee %s: %w", record.employee.ID, err)
				}
			}
			return nil
		}()
		if targetAdapter != nil {
			_ = targetAdapter.Disconnect(ctx)
		}
		if copyErr != nil {
			return fmt.Errorf("backup to %s failed: %w", targetDev.Name, copyErr)
		}
	}

	broadcast.Global.Broadcast("device_backup_completed", map[string]any{
		"source_device_id":  srcDevID,
		"target_device_ids": targetDevIDs,
		"employee_count":    len(records),
	})
	return nil
}

// ListFingerprints lấy danh sách vân tay của nhân viên
func (s *BiometricService) ListFingerprints(ctx context.Context, employeeID string) ([]entity.EmployeeFingerprint, error) {
	return s.fingerprintRepo.ListByEmployee(ctx, employeeID)
}

// DeleteFingerprint xóa vân tay của nhân viên và gửi lệnh xóa xuống các thiết bị
func (s *BiometricService) DeleteFingerprint(ctx context.Context, employeeID string, fingerIndex int) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}

	var devices []entity.Device
	if s.deviceRepo != nil {
		devices, err = s.deviceRepo.List(ctx)
		if err != nil {
			return err
		}
	}
	// Standalone SDK devices must be updated before removing the central
	// template, so a connection failure does not create a silent mismatch.
	for _, d := range devices {
		if isADMSDevice(d) || s.factory == nil {
			continue
		}
		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(ctx, employeeID, d.ID)
			if mapping != nil && mapping.DeviceUserID != "" {
				pin = mapping.DeviceUserID
			}
		}
		adapter, adapterErr := s.factory.NewAdapter(d.DeviceType)
		if adapterErr != nil {
			return adapterErr
		}
		cfg := port.DeviceConfig{
			IPAddress: d.IPAddress,
			Port:      d.Port,
			Username:  d.Username,
			Password:  d.Password,
			Timeout:   10 * time.Second,
		}
		if adapterErr = adapter.Connect(ctx, cfg); adapterErr != nil {
			return fmt.Errorf("connect SDK device %s: %w", d.Name, adapterErr)
		}
		deleteErr := adapter.DeleteFingerprint(ctx, pin, fingerIndex)
		_ = adapter.Disconnect(ctx)
		if deleteErr != nil {
			return fmt.Errorf("delete fingerprint on SDK device %s: %w", d.Name, deleteErr)
		}
	}

	if err := s.fingerprintRepo.Delete(ctx, employeeID, fingerIndex); err != nil {
		return err
	}
	// The employee badge represents whether at least one template remains.
	// Do not leave it true after the last fingerprint has been deleted.
	remaining, err := s.fingerprintRepo.ListByEmployee(ctx, employeeID)
	if err != nil {
		return err
	}
	if len(remaining) == 0 && emp.FingerprintEnrolled {
		emp.FingerprintEnrolled = false
		if err := s.employeeRepo.Update(ctx, emp); err != nil {
			return err
		}
	}

	// Gửi lệnh xóa xuống các thiết bị ADMS
	for _, d := range devices {
		if !isADMSDevice(d) || s.commandRepo == nil || d.LastHeartbeatAt == nil {
			continue
		}
		// Bỏ qua nếu thiết bị đã offline lâu (> 10 phút)
		if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
			continue
		}

		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(ctx, employeeID, d.ID)
			if mapping != nil {
				pin = mapping.DeviceUserID
			}
		}

		// Lệnh xóa vân tay: DATA DELETE FINGERTEMPLATE Pin=123\tFingerID=0
		cmdStr := fmt.Sprintf("DATA DELETE FINGERTEMPLATE Pin=%s\tFingerID=%d", pin, fingerIndex)
		_, _ = s.commandRepo.Enqueue(ctx, d.ID, cmdStr)
	}

	// Broadcast update
	broadcast.Global.Broadcast("fingerprint_updated", map[string]any{
		"employee_id":  employeeID,
		"finger_index": fingerIndex,
		"action":       "delete",
	})

	return nil
}

// PropagateToAllDevices đẩy toàn bộ vân tay của nhân viên sang các thiết bị ADMS khác
func (s *BiometricService) PropagateToAllDevices(ctx context.Context, employeeID string) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}

	fps, err := s.fingerprintRepo.ListByEmployee(ctx, employeeID)
	if err != nil {
		return err
	}
	if len(fps) == 0 {
		return nil
	}

	devices, err := s.deviceRepo.List(ctx)
	if err != nil {
		return err
	}

	enqueuedCount := 0
	for _, d := range devices {
		if !isADMSDevice(d) {
			if err := s.propagateToStandaloneDevice(ctx, d, *emp, fps); err != nil {
				return err
			}
			continue
		}
		if d.LastHeartbeatAt == nil {
			continue
		}
		// Thiết bị phải hoạt động trong 10 phút qua
		if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
			continue
		}

		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(ctx, employeeID, d.ID)
			if mapping != nil {
				pin = mapping.DeviceUserID
			} else {
				// Tạo mapping mặc định nếu chưa có và giữ trạng thái pending cho đến khi có ACK/BIODATA
				_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
					EmployeeID:   employeeID,
					DeviceID:     d.ID,
					DeviceUserID: pin,
					SyncStatus:   "pending",
				})
			}
		}

		// 1. Tạo lệnh cập nhật USER trước
		userCmd := buildADMSUserCommand(pin, emp.FullName, emp.CardNo)
		_, err = s.commandRepo.Enqueue(ctx, d.ID, userCmd)
		if err != nil {
			zap.L().Error("failed to enqueue user cmd", zap.Error(err))
			continue
		}

		// 2. Tạo lệnh cập nhật các FINGERTEMPLATE
		for _, fp := range fps {
			fpCmd := buildADMSFingerprintCommand(pin, fp.FingerIndex, fp.TemplateSize, fp.TemplateData)
			_, err = s.commandRepo.Enqueue(ctx, d.ID, fpCmd)
			if err != nil {
				zap.L().Error("failed to enqueue fp cmd", zap.Error(err))
				continue
			}
			enqueuedCount++
		}
	}

	if enqueuedCount > 0 {
		broadcast.Global.Broadcast("fingerprint_synced", map[string]any{
			"employee_id": employeeID,
			"commands":    enqueuedCount,
		})
	}

	return nil
}

// EnrollFingerprint kích hoạt màn hình đăng ký vân tay từ xa trên máy ADMS được chọn
// propagateToStandaloneDevice writes through the vendor SDK. Standalone devices
// do not poll the ADMS command queue, so they must never be skipped here.
func (s *BiometricService) propagateToStandaloneDevice(ctx context.Context, d entity.Device, emp entity.Employee, fps []entity.EmployeeFingerprint) error {
	if s.factory == nil {
		return fmt.Errorf("device %s: adapter factory is not configured", d.Name)
	}

	pin := emp.EmployeeCode
	if s.mappingRepo != nil {
		mapping, err := s.mappingRepo.GetByEmployeeAndDevice(ctx, emp.ID, d.ID)
		if err != nil {
			return fmt.Errorf("device %s: get employee mapping: %w", d.Name, err)
		}
		if mapping != nil && mapping.DeviceUserID != "" {
			pin = mapping.DeviceUserID
		}
	}
	if pin == "" {
		return fmt.Errorf("device %s: employee has no device user ID", d.Name)
	}

	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return fmt.Errorf("device %s: create adapter: %w", d.Name, err)
	}
	cfg := port.DeviceConfig{
		IPAddress: d.IPAddress,
		Port:      d.Port,
		Username:  d.Username,
		Password:  d.Password,
		Timeout:   10 * time.Second,
	}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return fmt.Errorf("device %s: connect through SDK: %w", d.Name, err)
	}
	defer func() { _ = adapter.Disconnect(ctx) }()

	deviceEmployee := emp
	deviceEmployee.EmployeeCode = pin
	if err := adapter.PushEmployee(ctx, deviceEmployee); err != nil {
		return fmt.Errorf("device %s: push employee %s: %w", d.Name, pin, err)
	}
	for _, fp := range fps {
		if err := adapter.PushFingerprint(ctx, pin, fp); err != nil {
			return fmt.Errorf("device %s: push fingerprint %d for %s: %w", d.Name, fp.FingerIndex, pin, err)
		}
	}
	return nil
}

func (s *BiometricService) EnrollFingerprint(ctx context.Context, employeeID string, deviceID string, fingerIndex int) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}

	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	pin := emp.EmployeeCode
	if s.mappingRepo != nil {
		mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(ctx, employeeID, d.ID)
		if mapping != nil {
			pin = mapping.DeviceUserID
		} else {
			// Tạo mapping mặc định nếu chưa có và giữ trạng thái pending
			_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
				EmployeeID:   employeeID,
				DeviceID:     d.ID,
				DeviceUserID: pin,
				SyncStatus:   "pending",
			})
		}
	}

	if !isADMSDevice(d) {
		captureCtx, captureCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if !s.beginSDKEnrollment(d.ID) {
			captureCancel()
			return fmt.Errorf("SDK fingerprint enrollment is already active on device %s; finish or wait for the current employee before starting another enrollment", d.Name)
		}
		s.setSDKEnrollmentCancel(d.ID, captureCancel)
		// Keep the same COM session open from StartEnrollEx through template
		// retrieval. Disconnecting immediately after StartEnrollEx cancels the
		// terminal's capture screen before the employee can place a finger.
		go func(device entity.Device, employee entity.Employee, userID string, captureCtx context.Context, captureCancel context.CancelFunc) {
			defer captureCancel()
			defer s.endSDKEnrollment(device.ID)
			if err := s.enrollAndCaptureSDKFingerprint(captureCtx, device, employee, userID, fingerIndex, sdkEnrollmentScanWindow); err != nil {
				zap.L().Warn("SDK fingerprint enrollment did not produce a template", zap.String("device_id", device.ID), zap.String("employee_id", employee.ID), zap.Error(err))
				broadcast.Global.Broadcast("fingerprint_enroll_skipped", map[string]any{"employee_id": employee.ID, "device_id": device.ID, "error": err.Error(), "batch": false, "reason": "timeout_or_cancelled"})
			}
		}(*d, *emp, pin, captureCtx, captureCancel)

		broadcast.Global.Broadcast("fingerprint_enroll_triggered", map[string]any{
			"employee_id": employeeID,
			"device_id":   deviceID,
			"pin":         pin,
		})
		return nil

		if s.factory == nil {
			return fmt.Errorf("device adapter factory not configured")
		}
		adapter, err := s.factory.NewAdapter(d.DeviceType)
		if err != nil {
			return err
		}
		cfg := port.DeviceConfig{
			IPAddress: d.IPAddress,
			Port:      d.Port,
			Username:  d.Username,
			Password:  d.Password,
			Timeout:   10 * time.Second,
		}
		if err := adapter.Connect(ctx, cfg); err != nil {
			return fmt.Errorf("không thể kết nối thiết bị: %w", err)
		}
		defer adapter.Disconnect(ctx)

		// Đảm bảo nhân viên đã tồn tại trên thiết bị Standalone trước khi gọi Enroll
		if err := adapter.PushEmployee(ctx, *emp); err != nil {
			zap.L().Warn("failed to push employee details before enroll, attempting to continue", zap.Error(err))
		}

		// Gọi đăng ký vân tay qua SDK/COM
		if err := adapter.EnrollFingerprint(ctx, pin, fingerIndex); err != nil {
			return fmt.Errorf("lỗi kích hoạt quét vân tay qua COM SDK: %w", err)
		}

		broadcast.Global.Broadcast("fingerprint_enroll_triggered", map[string]any{
			"employee_id": employeeID,
			"device_id":   deviceID,
			"pin":         pin,
		})
		return nil
	}

	if d.SerialNumberADMS == "" {
		return fmt.Errorf("device %s does not have ADMS serial configured", d.Name)
	}

	if s.commandRepo == nil {
		return fmt.Errorf("ADMS command repository is not configured")
	}

	// 1. Tạo lệnh USER cập nhật thông tin trước (đảm bảo User tồn tại trên thiết bị)
	userCmd := buildADMSUserCommand(pin, emp.FullName, emp.CardNo)
	if _, err := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err != nil {
		return fmt.Errorf("failed to enqueue ADMS user command: %w", err)
	}

	// 2. Gửi lệnh enroll ADMS để thiết bị bước vào màn hình đăng ký vân tay.
	enrollCmd := buildADMSEnrollCommand(pin, fingerIndex)
	if _, err := s.commandRepo.Enqueue(ctx, d.ID, enrollCmd); err != nil {
		return fmt.Errorf("failed to enqueue ADMS enroll command: %w", err)
	}

	zap.L().Info("queued ADMS user update and enroll command", zap.String("device_id", d.ID), zap.String("device_name", d.Name), zap.String("employee_id", employeeID), zap.String("pin", pin), zap.String("enroll_command", enrollCmd))

	broadcast.Global.Broadcast("fingerprint_enroll_pending", map[string]any{
		"employee_id": employeeID,
		"device_id":   deviceID,
		"pin":         pin,
		"reason":      "adms_enroll_command_queued",
	})

	return nil
}

// ReEnrollFingerprint starts a fresh capture for finger index and replaces
// the existing device template. EnrollFingerprint already uses OVERWRITE=1 for
// ADMS devices and SSR_DelUserTmpExt before StartEnrollEx for COM SDK devices.
// The central template is intentionally kept until the new capture arrives, so
// a timeout does not destroy the employee's last known-good template.
func (s *BiometricService) ReEnrollFingerprint(ctx context.Context, employeeID string, deviceID string, fingerIndex int) error {
	if err := s.EnrollFingerprint(ctx, employeeID, deviceID, fingerIndex); err != nil {
		return err
	}

	broadcast.Global.Broadcast("fingerprint_reenroll_requested", map[string]any{
		"employee_id":  employeeID,
		"device_id":    deviceID,
		"finger_index": 0,
	})
	return nil
}

// BatchEnroll gửi lệnh ENROLL_INFO hàng loạt cho danh sách nhân viên xuống 1 thiết bị ADMS.
// Máy sẽ tự xử lý lần lượt từng người theo thứ tự trong command queue.
// Trả về số lệnh đã enqueue thành công và danh sách lỗi từng nhân viên (nếu có).
func (s *BiometricService) startSDKEnrollment(ctx context.Context, device entity.Device, emp entity.Employee, pin string, fingerIndex int) error {
	if s.factory == nil {
		return fmt.Errorf("device adapter factory not configured")
	}
	adapter, err := s.factory.NewAdapter(device.DeviceType)
	if err != nil {
		return fmt.Errorf("create SDK adapter: %w", err)
	}
	cfg := port.DeviceConfig{
		IPAddress: device.IPAddress,
		Port:      device.Port,
		Username:  device.Username,
		Password:  device.Password,
		Timeout:   10 * time.Second,
	}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return fmt.Errorf("connect SDK device %s: %w", device.Name, err)
	}
	defer func() { _ = adapter.Disconnect(ctx) }()

	deviceEmployee := emp
	deviceEmployee.EmployeeCode = pin
	if err := adapter.PushEmployee(ctx, deviceEmployee); err != nil {
		return fmt.Errorf("push employee %s before enrollment: %w", pin, err)
	}
	if err := adapter.EnrollFingerprint(ctx, pin, fingerIndex); err != nil {
		return fmt.Errorf("start fingerprint enrollment for %s: %w", pin, err)
	}
	return nil
}

func (s *BiometricService) captureSDKFingerprint(ctx context.Context, device entity.Device, emp entity.Employee, pin string, fingerIndex int) error {
	if s.factory == nil || s.fingerprintRepo == nil {
		return fmt.Errorf("SDK fingerprint capture is not configured")
	}
	for {
		adapter, err := s.factory.NewAdapter(device.DeviceType)
		if err == nil {
			cfg := port.DeviceConfig{
				IPAddress: device.IPAddress,
				Port:      device.Port,
				Username:  device.Username,
				Password:  device.Password,
				Timeout:   10 * time.Second,
			}
			if err = adapter.Connect(ctx, cfg); err == nil {
				fp, readErr := adapter.GetFingerprint(ctx, pin, fingerIndex)
				_ = adapter.Disconnect(ctx)
				if readErr == nil && fp != nil && fp.TemplateData != "" {
					fp.EmployeeID = emp.ID
					fp.SourceDeviceID = device.ID
					if err = s.fingerprintRepo.Upsert(ctx, fp); err != nil {
						return fmt.Errorf("save SDK fingerprint: %w", err)
					}
					if !emp.FingerprintEnrolled {
						emp.FingerprintEnrolled = true
						if err = s.employeeRepo.Update(ctx, &emp); err != nil {
							return fmt.Errorf("mark employee fingerprint enrolled: %w", err)
						}
					}
					if s.mappingRepo != nil {
						_ = s.mappingRepo.MarkFingerprintEnrolled(ctx, emp.ID, device.ID, time.Now())
					}
					broadcast.Global.Broadcast("fingerprint_updated", map[string]any{
						"employee_id":  emp.ID,
						"device_id":    device.ID,
						"finger_index": fingerIndex,
						"action":       "captured_via_sdk",
					})
					return nil
				}
			} else {
				_ = adapter.Disconnect(ctx)
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for fingerprint %d of user %s: %w", fingerIndex, pin, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

// enrollAndCaptureSDKFingerprint keeps one COM session open from StartEnrollEx
// through template retrieval. Some terminals discard the interactive state when
// the initiating SDK connection is closed, which left a batch stuck at person 1.
func (s *BiometricService) enrollAndCaptureSDKFingerprint(ctx context.Context, device entity.Device, emp entity.Employee, pin string, fingerIndex int, scanWindow time.Duration) error {
	if s.factory == nil || s.fingerprintRepo == nil {
		return fmt.Errorf("SDK fingerprint enrollment is not configured")
	}
	adapter, err := s.factory.NewAdapter(device.DeviceType)
	if err != nil {
		return fmt.Errorf("create SDK adapter: %w", err)
	}
	cfg := port.DeviceConfig{
		IPAddress: device.IPAddress,
		Port:      device.Port,
		Username:  device.Username,
		Password:  device.Password,
		Timeout:   10 * time.Second,
	}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return fmt.Errorf("connect SDK device %s: %w", device.Name, err)
	}
	defer func() { _ = adapter.Disconnect(ctx) }()

	deviceEmployee := emp
	deviceEmployee.EmployeeCode = pin
	if err := adapter.PushEmployee(ctx, deviceEmployee); err != nil {
		return fmt.Errorf("push employee %s before enrollment: %w", pin, err)
	}
	if err := adapter.EnrollFingerprint(ctx, pin, fingerIndex); err != nil {
		return fmt.Errorf("start fingerprint enrollment for %s: %w", pin, err)
	}
	zap.L().Info("SDK batch enrollment started; waiting for template", zap.String("device_id", device.ID), zap.String("employee_id", emp.ID), zap.String("pin", pin), zap.Int("finger_index", fingerIndex))
	broadcast.Global.Broadcast("fingerprint_enroll_triggered", map[string]any{"employee_id": emp.ID, "device_id": device.ID, "pin": pin, "batch": true})

	if scanWindow <= 0 {
		scanWindow = sdkEnrollmentScanWindow
	}
	scanTimer := time.NewTimer(scanWindow)
	defer scanTimer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("fingerprint enrollment cancelled for user %s: %w", pin, ctx.Err())
	case <-scanTimer.C:
	}

	// This is deliberately the first SDK call after StartEnrollEx. Polling
	// GetUserTmpExStr while the employee is scanning makes affected terminals
	// flash the enrollment page and return to the idle screen immediately.
	fp, readErr := adapter.GetFingerprint(ctx, pin, fingerIndex)
	if readErr != nil || fp == nil || fp.TemplateData == "" {
		if readErr != nil {
			return fmt.Errorf("fingerprint %d was not captured for user %s after %s: %w", fingerIndex, pin, scanWindow, readErr)
		}
		return fmt.Errorf("fingerprint %d was not captured for user %s after %s", fingerIndex, pin, scanWindow)
	}

	fp.EmployeeID = emp.ID
	fp.SourceDeviceID = device.ID
	if err := s.fingerprintRepo.Upsert(ctx, fp); err != nil {
		return fmt.Errorf("save SDK fingerprint: %w", err)
	}
	if !emp.FingerprintEnrolled {
		emp.FingerprintEnrolled = true
		if err := s.employeeRepo.Update(ctx, &emp); err != nil {
			return fmt.Errorf("mark employee fingerprint enrolled: %w", err)
		}
	}
	if s.mappingRepo != nil {
		_ = s.mappingRepo.MarkFingerprintEnrolled(ctx, emp.ID, device.ID, time.Now())
	}
	zap.L().Info("SDK batch enrollment captured; advancing to next employee", zap.String("device_id", device.ID), zap.String("employee_id", emp.ID), zap.String("pin", pin), zap.Int("finger_index", fingerIndex))
	broadcast.Global.Broadcast("fingerprint_updated", map[string]any{"employee_id": emp.ID, "device_id": device.ID, "finger_index": fingerIndex, "action": "captured_via_sdk"})
	return nil
}

func (s *BiometricService) batchEnrollViaSDK(ctx context.Context, device entity.Device, employeeIDs []string) (int, []string, error) {
	seen := make(map[string]struct{}, len(employeeIDs))
	type enrollment struct {
		employee entity.Employee
		pin      string
	}
	pending := make([]enrollment, 0, len(employeeIDs))
	var errList []string
	for _, employeeID := range employeeIDs {
		if _, exists := seen[employeeID]; exists {
			continue
		}
		seen[employeeID] = struct{}{}
		emp, err := s.employeeRepo.GetByID(ctx, employeeID)
		if err != nil {
			errList = append(errList, fmt.Sprintf("employee %s: lookup error: %v", employeeID, err))
			continue
		}
		if emp == nil {
			emp, _ = s.employeeRepo.GetByCode(ctx, employeeID)
		}
		if emp == nil {
			errList = append(errList, fmt.Sprintf("employee %s: not found", employeeID))
			continue
		}
		// The selected SDK batch is an explicit enrollment/re-enrollment action.
		// Do not silently skip a selected employee just because a previous
		// template or badge exists in the web database.
		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			if mapping, err := s.mappingRepo.GetByEmployeeAndDevice(ctx, emp.ID, device.ID); err == nil && mapping != nil && mapping.DeviceUserID != "" {
				pin = mapping.DeviceUserID
			}
		}
		if pin == "" {
			errList = append(errList, fmt.Sprintf("employee %s: device user ID is empty", emp.ID))
			continue
		}
		pending = append(pending, enrollment{employee: *emp, pin: pin})
	}

	if len(pending) == 0 {
		return 0, errList, nil
	}
	batchCtx, batchCancel := context.WithCancel(context.Background())
	if !s.beginSDKEnrollment(device.ID) {
		batchCancel()
		return 0, errList, fmt.Errorf("SDK fingerprint enrollment is already active on device %s; do not start batch enrollment again", device.Name)
	}
	s.setSDKEnrollmentCancel(device.ID, batchCancel)
	// The SDK exposes one interactive enrollment screen only. Process the
	// selected employees one by one, waiting until each template is captured
	// before showing the next person on the terminal.
	go func() {
		defer batchCancel()
		defer s.endSDKEnrollment(device.ID)
		for _, item := range pending {
			if batchCtx.Err() != nil {
				return
			}
			err := s.enrollAndCaptureSDKFingerprint(batchCtx, device, item.employee, item.pin, 0, sdkEnrollmentScanWindow)
			if err != nil {
				if batchCtx.Err() != nil {
					zap.L().Info("SDK batch fingerprint enrollment stopped", zap.String("device_id", device.ID), zap.String("employee_id", item.employee.ID), zap.String("pin", item.pin))
					broadcast.Global.Broadcast("fingerprint_enroll_stopped", map[string]any{"employee_id": item.employee.ID, "device_id": device.ID, "batch": true})
					return
				}
				zap.L().Warn("SDK batch fingerprint enrollment skipped; advancing to next employee", zap.String("device_id", device.ID), zap.String("employee_id", item.employee.ID), zap.String("pin", item.pin), zap.Error(err))
				broadcast.Global.Broadcast("fingerprint_enroll_skipped", map[string]any{"employee_id": item.employee.ID, "device_id": device.ID, "error": err.Error(), "batch": true, "reason": "timeout_or_cancelled"})
			}
		}
	}()

	return len(pending), errList, nil
}

func (s *BiometricService) BatchEnroll(ctx context.Context, employeeIDs []string, deviceID string) (int, []string, error) {
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return 0, nil, err
	}
	if d == nil {
		return 0, nil, fmt.Errorf("device not found: %s", deviceID)
	}
	if !isADMSDevice(d) {
		return s.batchEnrollViaSDK(ctx, *d, employeeIDs)
	}
	if s.commandRepo == nil {
		return 0, nil, fmt.Errorf("ADMS command repository is not configured")
	}

	enqueuedCount := 0
	var errList []string
	seenEmployees := make(map[string]struct{}, len(employeeIDs))

	for _, empID := range employeeIDs {
		// A duplicated id from the browser must not create two consecutive
		// enrollment prompts for the same person on the physical device.
		if _, seen := seenEmployees[empID]; seen {
			continue
		}
		seenEmployees[empID] = struct{}{}
		emp, err := s.employeeRepo.GetByID(ctx, empID)
		if err != nil {
			errList = append(errList, fmt.Sprintf("employee %s: lookup error: %v", empID, err))
			continue
		}
		// If not found by ID, try by employee code to be tolerant of frontends
		// that may send employee codes instead of internal IDs.
		if emp == nil {
			if s.employeeRepo != nil {
				if byCode, codeErr := s.employeeRepo.GetByCode(ctx, empID); codeErr == nil && byCode != nil {
					emp = byCode
				}
			}
		}
		if emp == nil {
			errList = append(errList, fmt.Sprintf("employee %s: not found", empID))
			continue
		}
		if emp.FingerprintEnrolled {
			continue
		}
		if s.fingerprintRepo != nil {
			if fps, fpErr := s.fingerprintRepo.ListByEmployee(ctx, empID); fpErr == nil && len(fps) > 0 {
				continue
			}
		}

		// Xác định PIN trên thiết bị (dùng mapping nếu có, fallback về employee_code)
		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(ctx, empID, d.ID)
			if mapping != nil {
				pin = mapping.DeviceUserID
			} else {
				// Tạo mapping mặc định nếu chưa có và giữ trạng thái pending
				_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
					EmployeeID:   empID,
					DeviceID:     d.ID,
					DeviceUserID: pin,
					SyncStatus:   "pending",
				})
			}
		}

		// 1. Đảm bảo user tồn tại trên thiết bị
		userCmd := buildADMSUserCommand(pin, emp.FullName, emp.CardNo)
		if _, err := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err != nil {
			errList = append(errList, fmt.Sprintf("employee %s (%s): enqueue USER failed: %v", emp.EmployeeCode, emp.FullName, err))
			continue
		}

		// 2. Put the device into fingerprint-enrollment mode. Keeping this
		// immediately after USER is important: ADMS returns commands in queue
		// order, so the PIN is present on the device before it receives ENROLL.
		enrollCmd := buildADMSEnrollCommand(pin, 0)
		if _, err := s.commandRepo.Enqueue(ctx, d.ID, enrollCmd); err != nil {
			errList = append(errList, fmt.Sprintf("employee %s (%s): enqueue fingerprint enrollment failed: %v", emp.EmployeeCode, emp.FullName, err))
			continue
		}

		enqueuedCount++
	}

	if enqueuedCount > 0 {
		broadcast.Global.Broadcast("batch_enroll_queued", map[string]any{
			"device_id":       deviceID,
			"device_name":     d.Name,
			"enqueued":        enqueuedCount,
			"total_requested": len(employeeIDs),
		})
	}

	return enqueuedCount, errList, nil
}

// BackupDeviceTemplates đồng bộ tất cả người dùng và vân tay từ máy nguồn (srcDevID) sang các máy đích (targetDevIDs).
func (s *BiometricService) backupDeviceTemplatesLegacy(ctx context.Context, srcDevID string, targetDevIDs []string) error {
	srcDev, err := s.deviceRepo.GetByID(ctx, srcDevID)
	if err != nil {
		return fmt.Errorf("không tìm thấy thiết bị nguồn: %w", err)
	}
	if srcDev == nil {
		return fmt.Errorf("thiết bị nguồn không tồn tại")
	}

	// Lấy tất cả mapping từ thiết bị nguồn để biết những người dùng nào đang ở trên thiết bị nguồn
	mappings, err := s.mappingRepo.ListByDevice(ctx, srcDevID)
	if err != nil {
		return fmt.Errorf("lỗi lấy danh sách nhân viên từ thiết bị nguồn: %w", err)
	}

	if len(mappings) == 0 {
		return fmt.Errorf("thiết bị nguồn không có nhân viên nào để backup")
	}

	// Duyệt qua từng thiết bị đích
	for _, targetID := range targetDevIDs {
		if targetID == srcDevID {
			continue // không backup cho chính nó
		}

		targetDev, err := s.deviceRepo.GetByID(ctx, targetID)
		if err != nil || targetDev == nil {
			zap.L().Warn("bỏ qua thiết bị đích không tồn tại hoặc lỗi", zap.String("device_id", targetID), zap.Error(err))
			continue
		}

		// Với mỗi nhân viên trên thiết bị nguồn, đồng bộ sang thiết bị đích
		for _, m := range mappings {
			emp, err := s.employeeRepo.GetByID(ctx, m.EmployeeID)
			if err != nil || emp == nil {
				continue
			}

			// Lấy các vân tay của nhân viên này
			fps, err := s.fingerprintRepo.ListByEmployee(ctx, m.EmployeeID)
			if err != nil {
				continue
			}

			// Nếu DB chưa có vân tay và thiết bị nguồn là Pull SDK, kết nối để lấy vân tay trực tiếp
			if len(fps) == 0 && !srcDev.ADMSEnabled && s.factory != nil {
				srcAdapter, err := s.factory.NewAdapter(srcDev.DeviceType)
				if err == nil {
					srcCfg := port.DeviceConfig{
						IPAddress: srcDev.IPAddress,
						Port:      srcDev.Port,
						Username:  srcDev.Username,
						Password:  srcDev.Password,
						Timeout:   10 * time.Second,
					}
					if err := srcAdapter.Connect(ctx, srcCfg); err == nil {
						// Thử quét lấy vân tay từ ngón 0 đến 9
						for idx := 0; idx <= 9; idx++ {
							fp, err := srcAdapter.GetFingerprint(ctx, m.DeviceUserID, idx)
							if err == nil && fp != nil {
								fp.EmployeeID = m.EmployeeID
								fp.SourceDeviceID = srcDevID
								_ = s.fingerprintRepo.Upsert(ctx, fp)
								fps = append(fps, *fp)
								fmt.Printf("[BiometricService] Backup: Successfully pulled template index %d for user %s from source device\n", idx, m.EmployeeID)
							}
						}
						_ = srcAdapter.Disconnect(ctx)
					}
				}
			}

			// Đảm bảo có mapping cho thiết bị đích
			targetMapping, err := s.mappingRepo.GetByDeviceUserID(ctx, targetID, m.DeviceUserID)
			if err != nil {
				continue
			}
			if targetMapping == nil {
				targetMapping = &entity.EmployeeDeviceMapping{
					EmployeeID:   m.EmployeeID,
					DeviceID:     targetID,
					DeviceUserID: m.DeviceUserID,
					SyncStatus:   "synced",
				}
				_ = s.mappingRepo.Upsert(ctx, targetMapping)
			}

			if targetDev.ADMSEnabled {
				// 1. Tạo lệnh USER
				userCmd := buildADMSUserCommand(m.DeviceUserID, emp.FullName, emp.CardNo)
				_, _ = s.commandRepo.Enqueue(ctx, targetID, userCmd)
				// 2. Tạo lệnh FINGERTEMPLATE cho từng vân tay
				for _, f := range fps {
					fpCmd := buildADMSFingerprintCommand(m.DeviceUserID, f.FingerIndex, f.TemplateSize, f.TemplateData)
					_, _ = s.commandRepo.Enqueue(ctx, targetID, fpCmd)
				}
			} else {
				// Thiết bị Pull SDK
				if s.factory == nil {
					continue
				}
				adapter, err := s.factory.NewAdapter(targetDev.DeviceType)
				if err != nil {
					zap.L().Error("failed to create adapter for target device", zap.String("device_id", targetID), zap.Error(err))
					continue
				}

				cfg := port.DeviceConfig{
					IPAddress: targetDev.IPAddress,
					Port:      targetDev.Port,
					Username:  targetDev.Username,
					Password:  targetDev.Password,
					Timeout:   10 * time.Second,
				}
				if err := adapter.Connect(ctx, cfg); err != nil {
					zap.L().Warn("failed to connect to target device", zap.String("device_id", targetID), zap.Error(err))
					continue
				}

				// Push user info
				deviceEmployee := *emp
				deviceEmployee.EmployeeCode = m.DeviceUserID
				if err := adapter.PushEmployee(ctx, deviceEmployee); err != nil {
					zap.L().Warn("failed to push employee to target device", zap.String("device_id", targetID), zap.Error(err))
				} else {
					// Push fingerprints
					for _, f := range fps {
						if err := adapter.PushFingerprint(ctx, m.DeviceUserID, f); err != nil {
							zap.L().Warn("failed to push fingerprint to target device", zap.String("device_id", targetID), zap.Error(err))
						}
					}
				}
				_ = adapter.Disconnect(ctx)
			}
		}
		// Không xếp lại người đã có template. Điều này đảm bảo lệnh hàng loạt
		// thực sự chỉ lần lượt yêu cầu các nhân viên còn thiếu vân tay.
	}

	return nil
}

// ClearEnrollDataForEmployee gửi lệnh xóa toàn bộ enroll data của một nhân viên trên tất cả thiết bị ADMS
func (s *BiometricService) CancelPendingBatchEnroll(ctx context.Context, deviceID string) (int, error) {
	if s.deviceRepo != nil {
		device, err := s.deviceRepo.GetByID(ctx, deviceID)
		if err != nil {
			return 0, err
		}
		if device != nil && !isADMSDevice(device) {
			if done, cancelled := s.cancelSDKEnrollment(deviceID); cancelled {
				select {
				case <-done:
					return 1, nil
				case <-time.After(3 * time.Second):
					return 0, fmt.Errorf("SDK enrollment is still stopping; wait a moment and try again")
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			}
			return 0, nil
		}
	}
	if s.commandRepo == nil {
		return 0, fmt.Errorf("command repository is not configured")
	}
	return s.commandRepo.CancelPendingByDevice(ctx, deviceID)
}

func (s *BiometricService) ClearEnrollDataForEmployee(ctx context.Context, employeeID string) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return fmt.Errorf("employee not found: %w", err)
	}
	if emp == nil {
		return fmt.Errorf("employee %s not found", employeeID)
	}

	devices, err := s.deviceRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, d := range devices {
		if !isADMSDevice(d) || d.LastHeartbeatAt == nil {
			continue
		}
		if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
			continue
		}

		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByDeviceUserID(ctx, d.ID, emp.EmployeeCode)
			if mapping != nil {
				pin = mapping.DeviceUserID
			}
		}

		// Lệnh xóa toàn bộ enroll data: DATA DELETE USER Pin=123
		cmdStr := fmt.Sprintf("DATA DELETE USER Pin=%s", pin)
		_, _ = s.commandRepo.Enqueue(ctx, d.ID, cmdStr)
	}

	// The requested operation removes the enrollment record from devices, so
	// clear the local templates and badge as well. This keeps the UI from
	// incorrectly showing "enrolled" after a successful clear request.
	if s.fingerprintRepo != nil {
		for fingerIndex := 0; fingerIndex <= 9; fingerIndex++ {
			if err := s.fingerprintRepo.Delete(ctx, employeeID, fingerIndex); err != nil {
				return err
			}
		}
	}
	if emp.FingerprintEnrolled {
		emp.FingerprintEnrolled = false
		if err := s.employeeRepo.Update(ctx, emp); err != nil {
			return err
		}
	}

	broadcast.Global.Broadcast("enroll_data_cleared", map[string]any{
		"employee_id":   employeeID,
		"employee_code": emp.EmployeeCode,
	})

	return nil
}

// ClearDeviceLog gửi lệnh xóa log chấm công trên thiết bị (ADMS hoặc Standalone)
func (s *BiometricService) ClearDeviceLog(ctx context.Context, deviceID string) error {
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil || d == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	if !isADMSDevice(d) {
		// Gọi adapter cho thiết bị Standalone (PULL)
		adapter, err := s.factory.NewAdapter(d.DeviceType)
		if err != nil {
			return err
		}
		cfg := port.DeviceConfig{
			IPAddress: d.IPAddress,
			Port:      d.Port,
			Username:  d.Username,
			Password:  d.Password,
			Timeout:   10 * time.Second,
		}
		if err := adapter.Connect(ctx, cfg); err != nil {
			return fmt.Errorf("không thể kết nối thiết bị: %w", err)
		}
		defer adapter.Disconnect(ctx)

		if err := adapter.ClearAttendanceLogs(ctx); err != nil {
			return fmt.Errorf("lỗi khi xóa log chấm công qua COM SDK: %w", err)
		}

		broadcast.Global.Broadcast("device_log_cleared", map[string]any{
			"device_id":   deviceID,
			"device_name": d.Name,
		})
		return nil
	}

	// Lệnh xóa log: DATA DELETE ATTLOG cho ADMS
	cmdStr := "DATA DELETE ATTLOG"
	_, err = s.commandRepo.Enqueue(ctx, deviceID, cmdStr)
	if err != nil {
		return fmt.Errorf("failed to enqueue clear log command: %w", err)
	}

	broadcast.Global.Broadcast("device_log_cleared", map[string]any{
		"device_id":   deviceID,
		"device_name": d.Name,
	})

	return nil
}

// ResetDevice gửi lệnh reset toàn bộ thiết bị (xóa tất cả dữ liệu người dùng, vân tay, log)
func (s *BiometricService) ResetDevice(ctx context.Context, deviceID string) error {
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil || d == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	if !isADMSDevice(d) {
		// Gọi adapter cho thiết bị Standalone (PULL)
		adapter, err := s.factory.NewAdapter(d.DeviceType)
		if err != nil {
			return err
		}
		cfg := port.DeviceConfig{
			IPAddress: d.IPAddress,
			Port:      d.Port,
			Username:  d.Username,
			Password:  d.Password,
			Timeout:   10 * time.Second,
		}
		if err := adapter.Connect(ctx, cfg); err != nil {
			return fmt.Errorf("không thể kết nối thiết bị: %w", err)
		}
		defer adapter.Disconnect(ctx)

		if err := adapter.Reset(ctx); err != nil {
			return fmt.Errorf("lỗi khi reset thiết bị qua COM SDK: %w", err)
		}

		broadcast.Global.Broadcast("device_reset", map[string]any{
			"device_id":   deviceID,
			"device_name": d.Name,
		})
		return nil
	}

	// Gửi các lệnh xóa dữ liệu cho ADMS
	// 1. Xóa tất cả USER
	cmdStr1 := "DATA DELETE USER"
	_, _ = s.commandRepo.Enqueue(ctx, deviceID, cmdStr1)

	// 2. Xóa tất cả FINGERTEMPLATE
	cmdStr2 := "DATA DELETE FINGERTEMPLATE"
	_, _ = s.commandRepo.Enqueue(ctx, deviceID, cmdStr2)

	// 3. Xóa tất cả ATTLOG
	cmdStr3 := "DATA DELETE ATTLOG"
	_, _ = s.commandRepo.Enqueue(ctx, deviceID, cmdStr3)

	broadcast.Global.Broadcast("device_reset", map[string]any{
		"device_id":   deviceID,
		"device_name": d.Name,
	})

	return nil
}
