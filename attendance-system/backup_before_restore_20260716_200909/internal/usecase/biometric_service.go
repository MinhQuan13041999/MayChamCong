package usecase

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

type BiometricService struct {
	fingerprintRepo port.FingerprintRepository
	deviceRepo      port.DeviceRepository
	commandRepo     port.DeviceCommandRepository
	employeeRepo    port.EmployeeRepository
	mappingRepo     port.EmployeeDeviceMappingRepository
	factory         port.DeviceAdapterFactory
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
		fingerprintRepo: fingerprintRepo,
		deviceRepo:      deviceRepo,
		commandRepo:     commandRepo,
		employeeRepo:    employeeRepo,
		mappingRepo:     mappingRepo,
		factory:         factory,
	}
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
	devices, err := s.deviceRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, d := range devices {
		if !isADMSDevice(d) || d.LastHeartbeatAt == nil {
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
		if !isADMSDevice(d) || d.LastHeartbeatAt == nil {
			continue
		}
		// Thiết bị phải hoạt động trong 10 phút qua
		if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
			continue
		}

		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByDeviceUserID(ctx, d.ID, emp.EmployeeCode)
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
func (s *BiometricService) EnrollFingerprint(ctx context.Context, employeeID string, deviceID string) error {
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

		// Gọi đăng ký vân tay ngón 0 qua SDK/COM
		if err := adapter.EnrollFingerprint(ctx, pin, 0); err != nil {
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

	// 2. Chỉ gửi user update; không kích hoạt enroll trên máy chấm công.
	// Với ADMS, việc bắt quét thực tế phụ thuộc vào firmware/SDK của thiết bị.
	// Vì vậy không coi đây là enrollment thành công.
	zap.L().Info("queued ADMS user update without triggering fingerprint enrollment", zap.String("device_id", d.ID), zap.String("device_name", d.Name), zap.String("employee_id", employeeID), zap.String("pin", pin))

	broadcast.Global.Broadcast("fingerprint_enroll_pending", map[string]any{
		"employee_id": employeeID,
		"device_id":   deviceID,
		"pin":         pin,
		"reason":      "adms_user_sync_only",
	})

	return nil
}

// BatchEnroll gửi lệnh ENROLL_INFO hàng loạt cho danh sách nhân viên xuống 1 thiết bị ADMS.
// Máy sẽ tự xử lý lần lượt từng người theo thứ tự trong command queue.
// Trả về số lệnh đã enqueue thành công và danh sách lỗi từng nhân viên (nếu có).
func (s *BiometricService) BatchEnroll(ctx context.Context, employeeIDs []string, deviceID string) (int, []string, error) {
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return 0, nil, err
	}
	if d == nil {
		return 0, nil, fmt.Errorf("device not found: %s", deviceID)
	}
	if !isADMSDevice(d) {
		return 0, nil, fmt.Errorf("device %s is not ADMS-enabled", d.Name)
	}
	if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
		return 0, nil, fmt.Errorf("device %s is offline or has not contacted ADMS in the last 10 minutes", d.Name)
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

		// 2. Chỉ gửi user update; không kích hoạt quét vân tay trên máy.
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
func (s *BiometricService) BackupDeviceTemplates(ctx context.Context, srcDevID string, targetDevIDs []string) error {
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
					srcCfg := port.DeviceConfig{IPAddress: srcDev.IPAddress, Port: srcDev.Port, Timeout: 10 * time.Second}
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

				cfg := port.DeviceConfig{IPAddress: targetDev.IPAddress, Port: targetDev.Port, Timeout: 10 * time.Second}
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
		cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}
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
		cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}
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
