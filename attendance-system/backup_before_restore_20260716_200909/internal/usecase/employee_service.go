package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"

	"go.uber.org/zap"
)

// EmployeeService xử lý business logic liên quan tới nhân viên,
// bao gồm đồng bộ hai chiều xuống thiết bị chấm công.
type EmployeeService struct {
	employeeRepo    port.EmployeeRepository
	deviceRepo      port.DeviceRepository
	syncHistoryRepo port.SyncHistoryRepository
	factory         port.DeviceAdapterFactory
	mappingRepo     port.EmployeeDeviceMappingRepository
	commandRepo     port.DeviceCommandRepository
	fingerprintRepo port.FingerprintRepository
}

func (s *EmployeeService) SetFingerprintRepo(repo port.FingerprintRepository) {
	s.fingerprintRepo = repo
}

func NewEmployeeService(
	employeeRepo port.EmployeeRepository,
	deviceRepo port.DeviceRepository,
	syncHistoryRepo port.SyncHistoryRepository,
	factory port.DeviceAdapterFactory,
	mappingRepo port.EmployeeDeviceMappingRepository,
	commandRepo port.DeviceCommandRepository,
) *EmployeeService {
	return &EmployeeService{
		employeeRepo:    employeeRepo,
		deviceRepo:      deviceRepo,
		syncHistoryRepo: syncHistoryRepo,
		factory:         factory,
		mappingRepo:     mappingRepo,
		commandRepo:     commandRepo,
	}
}

// SyncEmployeeToDevice creates/updates one device account using deviceUserID.
// It intentionally does not transfer biometric templates; enrollment is done on-device.
func isADMSDevice(d any) bool {
	switch v := d.(type) {
	case nil:
		return false
	case *entity.Device:
		if v == nil {
			return false
		}
		return v.ADMSEnabled || strings.TrimSpace(v.SerialNumberADMS) != ""
	case entity.Device:
		return v.ADMSEnabled || strings.TrimSpace(v.SerialNumberADMS) != ""
	default:
		return false
	}
}

func (s *EmployeeService) SyncEmployeeToDevice(ctx context.Context, employeeID, deviceID, deviceUserID string) (*entity.EmployeeDeviceMapping, error) {
	if s.mappingRepo == nil {
		return nil, fmt.Errorf("employee-device mapping is not configured")
	}
	if deviceUserID == "" {
		return nil, fmt.Errorf("device_user_id is required")
	}
	emp, err := s.resolveEmployeeByIdentifier(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, fmt.Errorf("employee not found")
	}
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("device not found")
	}
	mapping := &entity.EmployeeDeviceMapping{EmployeeID: employeeID, DeviceID: deviceID, DeviceUserID: deviceUserID, SyncStatus: "pending"}
	if err := s.mappingRepo.Upsert(ctx, mapping); err != nil {
		return nil, err
	}

	// Nếu thiết bị là ADMS Push Protocol
	if isADMSDevice(d) {
		// Nếu thiết bị ADMS nhưng không có heartbeat gần đây, từ chối để tránh đưa lệnh vào queue
		if !isDeviceOnline(d.LastHeartbeatAt, 15*time.Minute) {
			return nil, fmt.Errorf("device '%s' appears offline (no recent heartbeat)", d.Name)
		}
		if s.commandRepo == nil {
			return nil, fmt.Errorf("ADMS command queue repository is not configured")
		}

		// 1. Tạo/cập nhật thông tin nhân viên trên máy
		userCmd := buildADMSUserCommand(deviceUserID, emp.FullName, emp.CardNo)
		if _, err := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err != nil {
			return s.recordMappingFailure(ctx, mapping, err)
		}

		// 2. Nếu có vân tay trong DB, đẩy luôn template xuống máy.
		// Không kích hoạt quét vân tay ở đây, vì thao tác này là đồng bộ dữ liệu nhân viên chung.
		if s.fingerprintRepo != nil {
			fps, fpErr := s.fingerprintRepo.ListByEmployee(ctx, emp.ID)
			if fpErr != nil {
				zap.L().Error("[ADMS] failed to list fingerprints for employee", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID), zap.Error(fpErr))
				return s.recordMappingFailure(ctx, mapping, fpErr)
			}
			zap.L().Info("[ADMS] retrieved fingerprints for sync", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID), zap.Int("fingerprint_count", len(fps)))
			for _, fp := range fps {
				fpCmd := buildADMSFingerprintCommand(deviceUserID, fp.FingerIndex, fp.TemplateSize, fp.TemplateData)
				zap.L().Debug("[ADMS] enqueuing fingerprint command", zap.String("pin", deviceUserID), zap.Int("finger_index", fp.FingerIndex), zap.Int("template_size", fp.TemplateSize))
				if _, err := s.commandRepo.Enqueue(ctx, d.ID, fpCmd); err != nil {
					zap.L().Error("[ADMS] failed to enqueue fingerprint command", zap.String("pin", deviceUserID), zap.Error(err))
					return s.recordMappingFailure(ctx, mapping, err)
				}
			}
		} else {
			zap.L().Warn("[ADMS] fingerprint repository is nil during employee sync", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID))
		}

		now := time.Now()
		mapping.SyncStatus = "synced"
		mapping.LastSyncedAt = &now
		mapping.LastError = ""
		if err := s.mappingRepo.Upsert(ctx, mapping); err != nil {
			return nil, err
		}
		return mapping, nil
	}

	// Cấu hình SDK cũ (Pull)
	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return s.recordMappingFailure(ctx, mapping, err)
	}
	if err := adapter.Connect(ctx, port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}); err != nil {
		return s.recordMappingFailure(ctx, mapping, err)
	}
	defer adapter.Disconnect(ctx)
	deviceEmployee := *emp
	deviceEmployee.EmployeeCode = deviceUserID
	if err := adapter.PushEmployee(ctx, deviceEmployee); err != nil {
		return s.recordMappingFailure(ctx, mapping, err)
	}
	now := time.Now()
	mapping.SyncStatus = "synced"
	mapping.LastSyncedAt = &now
	mapping.LastError = ""
	if err := s.mappingRepo.Upsert(ctx, mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func (s *EmployeeService) recordMappingFailure(ctx context.Context, mapping *entity.EmployeeDeviceMapping, cause error) (*entity.EmployeeDeviceMapping, error) {
	mapping.SyncStatus = "failed"
	mapping.LastError = cause.Error()
	_ = s.mappingRepo.Upsert(ctx, mapping)
	return mapping, cause
}

func (s *EmployeeService) resolveEmployeeByIdentifier(ctx context.Context, identifier string) (*entity.Employee, error) {
	if s.employeeRepo == nil || strings.TrimSpace(identifier) == "" {
		return nil, nil
	}

	trimmed := strings.TrimSpace(identifier)
	if emp, err := s.employeeRepo.GetByID(ctx, trimmed); err == nil && emp != nil {
		return emp, nil
	}
	if emp, err := s.employeeRepo.GetByCode(ctx, trimmed); err == nil && emp != nil {
		return emp, nil
	}

	employees, err := s.employeeRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range employees {
		e := employees[i]
		if strings.EqualFold(strings.TrimSpace(e.ID), trimmed) || strings.EqualFold(strings.TrimSpace(e.EmployeeCode), trimmed) {
			return &employees[i], nil
		}
	}
	return nil, nil
}

func (s *EmployeeService) ListDeviceMappings(ctx context.Context, employeeID string) ([]entity.EmployeeDeviceMapping, error) {
	if s.mappingRepo == nil {
		return nil, fmt.Errorf("employee-device mapping is not configured")
	}
	return s.mappingRepo.ListByEmployee(ctx, employeeID)
}

func (s *EmployeeService) ConfirmFingerprintEnrolled(ctx context.Context, employeeID, deviceID string) error {
	if s.mappingRepo == nil {
		return fmt.Errorf("employee-device mapping is not configured")
	}

	if err := s.mappingRepo.MarkFingerprintEnrolled(ctx, employeeID, deviceID, time.Now()); err != nil {
		return err
	}
	// The employee-level flag drives the fingerprint badge in the UI.
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}
	if !emp.FingerprintEnrolled {
		emp.FingerprintEnrolled = true
		if err := s.employeeRepo.Update(ctx, emp); err != nil {
			return err
		}
	}

	// Lấy thông tin thiết bị
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	// Nếu là thiết bị PULL và có fingerprintRepo, tự động kéo vân tay về lưu vào DB
	if !isADMSDevice(d) && s.fingerprintRepo != nil && s.factory != nil {
		mappings, err := s.mappingRepo.ListByEmployee(ctx, employeeID)
		if err == nil {
			var deviceUserID string
			for _, m := range mappings {
				if m.DeviceID == deviceID {
					deviceUserID = m.DeviceUserID
					break
				}
			}

			if deviceUserID != "" {
				adapter, err := s.factory.NewAdapter(d.DeviceType)
				if err == nil {
					cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}
					if err := adapter.Connect(ctx, cfg); err == nil {
						defer adapter.Disconnect(ctx)
						// Thử quét và lưu ngón index từ 0 đến 9 để kéo hết vân tay nhân viên này đã đăng ký
						for idx := 0; idx <= 9; idx++ {
							fp, err := adapter.GetFingerprint(ctx, deviceUserID, idx)
							if err == nil && fp != nil {
								fp.EmployeeID = employeeID
								fp.SourceDeviceID = deviceID
								_ = s.fingerprintRepo.Upsert(ctx, fp)
								fmt.Printf("[EmployeeService] Successfully pulled and saved fingerprint template index %d for user %s from device %s\n", idx, employeeID, deviceID)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (s *EmployeeService) markEmployeeFingerprintEnrolled(ctx context.Context, emp *entity.Employee, deviceID string, mapping *entity.EmployeeDeviceMapping) error {
	if emp == nil || emp.ID == "" || emp.FingerprintEnrolled {
		return nil
	}

	hasFingerprint := false
	if mapping != nil && mapping.FingerprintEnrolled {
		hasFingerprint = true
	}
	if !hasFingerprint && s.fingerprintRepo != nil {
		fps, err := s.fingerprintRepo.ListByEmployee(ctx, emp.ID)
		if err == nil && len(fps) > 0 {
			hasFingerprint = true
		}
	}
	if !hasFingerprint {
		return nil
	}

	emp.FingerprintEnrolled = true
	if err := s.employeeRepo.Update(ctx, emp); err != nil {
		return err
	}
	if s.mappingRepo != nil && deviceID != "" {
		if mapping == nil {
			mapping, _ = s.mappingRepo.GetByEmployeeAndDevice(ctx, emp.ID, deviceID)
		}
		if mapping == nil {
			now := time.Now()
			_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
				EmployeeID:          emp.ID,
				DeviceID:            deviceID,
				DeviceUserID:        emp.EmployeeCode,
				SyncStatus:          "synced",
				FingerprintEnrolled: true,
				FingerprintAt:       &now,
			})
		} else if !mapping.FingerprintEnrolled {
			now := time.Now()
			_ = s.mappingRepo.MarkFingerprintEnrolled(ctx, emp.ID, deviceID, now)
		}
	}
	return nil
}

func (s *EmployeeService) CreateEmployee(ctx context.Context, e *entity.Employee) error {
	if e.EmployeeCode == "" || e.FullName == "" {
		return fmt.Errorf("employee_code and full_name are required")
	}
	if e.Status == "" {
		e.Status = "active"
	}
	return s.employeeRepo.Create(ctx, e)
}

// CreateEmployeeWithEnrollment tạo nhân viên và nếu được yêu cầu thì tự động gửi lệnh enroll xuống thiết bị ADMS.
func (s *EmployeeService) CreateEmployeeWithEnrollment(ctx context.Context, e *entity.Employee, enroll bool, deviceID, deviceUserID string) error {
	if err := s.CreateEmployee(ctx, e); err != nil {
		return err
	}
	if !enroll {
		return nil
	}
	if deviceID == "" {
		return fmt.Errorf("device_id is required when enroll_fingerprint is enabled")
	}
	if deviceUserID == "" {
		return fmt.Errorf("device_user_id is required when enroll_fingerprint is enabled")
	}
	if s.commandRepo == nil {
		return fmt.Errorf("command repository is not configured for device enrollment")
	}

	if s.mappingRepo != nil {
		_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
			EmployeeID:   e.ID,
			DeviceID:     deviceID,
			DeviceUserID: deviceUserID,
			SyncStatus:   "pending",
		})
	}

	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}
	if !isADMSDevice(d) {
		return fmt.Errorf("device %s is not ADMS-enabled", deviceID)
	}

	pin := deviceUserID
	if pin == "" {
		pin = e.EmployeeCode
	}
	userCmd := buildADMSUserCommand(pin, e.FullName, e.CardNo)
	if _, err := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err != nil {
		return err
	}
	return nil
}

func (s *EmployeeService) UpdateEmployee(ctx context.Context, e *entity.Employee) error {
	return s.employeeRepo.Update(ctx, e)
}

func (s *EmployeeService) DeleteEmployee(ctx context.Context, id string) error {
	emp, err := s.employeeRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if emp == nil {
		return nil
	}

	// Xóa nhân viên trong DB
	if err := s.employeeRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Enqueue lệnh xóa User trên các thiết bị ADMS đang online
	if s.deviceRepo != nil && s.commandRepo != nil {
		devices, err := s.deviceRepo.List(ctx)
		if err == nil {
			for _, d := range devices {
				if !isADMSDevice(d) {
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
					}
				}

				// Lệnh xóa User chuẩn ADMS: DATA DELETE USER Pin=%s
				cmdStr := fmt.Sprintf("DATA DELETE USER Pin=%s", pin)
				_, _ = s.commandRepo.Enqueue(ctx, d.ID, cmdStr)
			}
		}
	}

	return nil
}

func (s *EmployeeService) ListEmployees(ctx context.Context) ([]entity.Employee, error) {
	return s.employeeRepo.List(ctx)
}

func (s *EmployeeService) GetEmployee(ctx context.Context, id string) (*entity.Employee, error) {
	return s.employeeRepo.GetByID(ctx, id)
}

// PushEmployeesToDevice đẩy toàn bộ nhân viên active xuống thiết bị chỉ định
func (s *EmployeeService) PushEmployeesToDevice(ctx context.Context, deviceID string, trigger entity.SyncTriggerType) (*entity.SyncHistory, error) {
	hist := &entity.SyncHistory{
		DeviceID:    deviceID,
		SyncType:    entity.SyncTypeEmployee,
		TriggerType: trigger,
		StartedAt:   time.Now(),
	}
	if err := s.syncHistoryRepo.Create(ctx, hist); err != nil {
		return nil, err
	}

	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return s.failSync(ctx, hist, err)
	}
	if d == nil {
		return s.failSync(ctx, hist, fmt.Errorf("device not found: %s", deviceID))
	}

	// ── Nhánh ADMS Push Protocol ──────────────────────────────────────────────
	// Không connect SDK standalone; đẩy lệnh vào command queue thay vào đó.
	if isADMSDevice(d) {
		employees, err := s.employeeRepo.ListActive(ctx)
		if err != nil {
			return s.failSync(ctx, hist, err)
		}
		if s.commandRepo == nil {
			return s.failSync(ctx, hist, fmt.Errorf("command repository not configured for ADMS device"))
		}

		// Lấy danh sách mapping hiện có để dùng đúng device_user_id
		existingMappings := make(map[string]string) // employeeID → deviceUserID
		if s.mappingRepo != nil {
			if mappings, err2 := s.mappingRepo.ListByDevice(ctx, d.ID); err2 == nil {
				for _, m := range mappings {
					existingMappings[m.EmployeeID] = m.DeviceUserID
				}
			}
		}

		successCount := 0
		var lastErr error
		for _, emp := range employees {
			deviceUserID := emp.EmployeeCode // mặc định dùng employee_code làm PIN
			if duID, ok := existingMappings[emp.ID]; ok && duID != "" {
				deviceUserID = duID
			} else if s.mappingRepo != nil {
				// Tạo mapping mới nếu chưa có, giữ trạng thái pending đến khi thiết bị xác nhận.
				newMapping := &entity.EmployeeDeviceMapping{
					EmployeeID:   emp.ID,
					DeviceID:     d.ID,
					DeviceUserID: deviceUserID,
					SyncStatus:   "pending",
				}
				_ = s.mappingRepo.Upsert(ctx, newMapping)
			}

			// Enqueue lệnh ghi thông tin nhân viên lên máy chấm công
			userCmd := buildADMSUserCommand(deviceUserID, emp.FullName, emp.CardNo)
			if _, err2 := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err2 != nil {
				lastErr = err2
				continue
			}

			// Khi đã có template trong kho trung tâm, đồng bộ cả vân tay cùng
			// với hồ sơ nhân viên. Trước đây thao tác "Sync NV" chỉ gửi USER.
			if s.fingerprintRepo != nil {
				fps, fpErr := s.fingerprintRepo.ListByEmployee(ctx, emp.ID)
				if fpErr != nil {
					zap.L().Error("[ADMS] failed to list fingerprints in PushEmployeesToDevice", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID), zap.Error(fpErr))
					lastErr = fpErr
				} else {
					zap.L().Info("[ADMS] retrieved fingerprints in PushEmployeesToDevice", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID), zap.Int("fingerprint_count", len(fps)))
					for _, fp := range fps {
						fpCmd := buildADMSFingerprintCommand(deviceUserID, fp.FingerIndex, fp.TemplateSize, fp.TemplateData)
						zap.L().Debug("[ADMS] enqueuing fingerprint in PushEmployeesToDevice", zap.String("pin", deviceUserID), zap.Int("finger_index", fp.FingerIndex))
						if _, fpErr = s.commandRepo.Enqueue(ctx, d.ID, fpCmd); fpErr != nil {
							zap.L().Error("[ADMS] failed to enqueue fingerprint in PushEmployeesToDevice", zap.String("pin", deviceUserID), zap.Error(fpErr))
							lastErr = fpErr
							break
						}
					}
				}
			} else {
				zap.L().Warn("[ADMS] fingerprint repository is nil in PushEmployeesToDevice", zap.String("employee_id", emp.ID), zap.String("device_id", d.ID))
			}
			successCount++
		}

		hist.RecordCount = successCount
		hist.FinishedAt = time.Now()
		if lastErr != nil && successCount == 0 {
			hist.Status = entity.SyncStatusFailed
			hist.ErrorMessage = lastErr.Error()
		} else if lastErr != nil {
			hist.Status = entity.SyncStatusPartial
			hist.ErrorMessage = lastErr.Error()
		} else {
			hist.Status = entity.SyncStatusSuccess
		}
		_ = s.syncHistoryRepo.Update(ctx, hist)
		return hist, nil
	}

	// ── Nhánh Standalone SDK (Pull Protocol) ─────────────────────────────────
	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return s.failSync(ctx, hist, err)
	}

	cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return s.failSync(ctx, hist, err)
	}
	defer adapter.Disconnect(ctx)

	employees, err := s.employeeRepo.ListActive(ctx)
	if err != nil {
		return s.failSync(ctx, hist, err)
	}

	successCount := 0
	var lastErr error
	for _, emp := range employees {
		if err := adapter.PushEmployee(ctx, emp); err != nil {
			lastErr = err
			continue
		}
		successCount++
	}

	hist.RecordCount = successCount
	hist.FinishedAt = time.Now()
	if lastErr != nil && successCount == 0 {
		hist.Status = entity.SyncStatusFailed
		hist.ErrorMessage = lastErr.Error()
	} else if lastErr != nil {
		hist.Status = entity.SyncStatusPartial
		hist.ErrorMessage = lastErr.Error()
	} else {
		hist.Status = entity.SyncStatusSuccess
	}
	_ = s.syncHistoryRepo.Update(ctx, hist)
	return hist, nil
}

// PushEmployeeToAllDevices đẩy 1 nhân viên xuống TẤT CẢ thiết bị ADMS đang online.
// Gửi lệnh DATA UPDATE USER vào queue của từng thiết bị.
func (s *EmployeeService) PushEmployeeToAllDevices(ctx context.Context, employeeID string) (int, []string, error) {
	emp, err := s.resolveEmployeeByIdentifier(ctx, employeeID)
	if err != nil {
		return 0, nil, err
	}
	if emp == nil {
		return 0, nil, fmt.Errorf("employee not found: %s", employeeID)
	}

	if s.commandRepo == nil {
		return 0, nil, fmt.Errorf("command repository not configured")
	}

	devices, err := s.deviceRepo.List(ctx)
	if err != nil {
		return 0, nil, err
	}

	successCount := 0
	var errList []string

	for _, d := range devices {
		if !isADMSDevice(d) {
			continue
		}
		// Chỉ đẩy xuống thiết bị đang online (heartbeat trong 15 phút gần nhất)
		if !isDeviceOnline(d.LastHeartbeatAt, 15*time.Minute) {
			errList = append(errList, fmt.Sprintf("device '%s': offline or no heartbeat", d.Name))
			continue
		}

		// Xác định PIN (dùng mapping nếu có)
		pin := emp.EmployeeCode
		if s.mappingRepo != nil {
			existingMappings, _ := s.mappingRepo.ListByDevice(ctx, d.ID)
			for _, m := range existingMappings {
				if m.EmployeeID == employeeID {
					pin = m.DeviceUserID
					break
				}
			}
			// Tạo mapping mặc định nếu chưa có, nhưng giữ trạng thái pending đến khi thiết bị xác nhận.
			found := false
			for _, m := range existingMappings {
				if m.EmployeeID == employeeID {
					found = true
					break
				}
			}
			if !found {
				_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
					EmployeeID:   employeeID,
					DeviceID:     d.ID,
					DeviceUserID: pin,
					SyncStatus:   "pending",
				})
			}
		}

		userCmd := buildADMSUserCommand(pin, emp.FullName, emp.CardNo)
		if _, err := s.commandRepo.Enqueue(ctx, d.ID, userCmd); err != nil {
			errList = append(errList, fmt.Sprintf("device '%s': %v", d.Name, err))
			continue
		}

		if s.fingerprintRepo != nil {
			fps, fpErr := s.fingerprintRepo.ListByEmployee(ctx, emp.ID)
			if fpErr != nil {
				errList = append(errList, fmt.Sprintf("device '%s': load fingerprints: %v", d.Name, fpErr))
				continue
			}
			for _, fp := range fps {
				if _, err := s.commandRepo.Enqueue(ctx, d.ID, buildADMSFingerprintCommand(pin, fp.FingerIndex, fp.TemplateSize, fp.TemplateData)); err != nil {
					errList = append(errList, fmt.Sprintf("device '%s': %v", d.Name, err))
					break
				}
			}
			if len(errList) > 0 {
				if lastErr := errList[len(errList)-1]; strings.Contains(lastErr, fmt.Sprintf("device '%s':", d.Name)) {
					continue
				}
			}
		}
		successCount++
	}

	return successCount, errList, nil
}

// PullEmployeesFromDevice kéo danh sách nhân viên từ máy chấm công về web.
// Với mỗi nhân viên trên máy:
//   - Nếu đã có employee_code tương ứng trong DB → tạo/cập nhật mapping.
//   - Nếu chưa có → tạo nhân viên mới với trạng thái "active".
//
// Trả về số nhân viên đã import mới, số đã có sẵn (mapping), và danh sách lỗi.
func (s *EmployeeService) PullEmployeesFromDevice(ctx context.Context, deviceID string) (imported int, existing int, errList []string, retErr error) {
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return 0, 0, nil, err
	}
	if d == nil {
		return 0, 0, nil, fmt.Errorf("device not found: %s", deviceID)
	}

	// Pull qua SDK adapter (ADMS không hỗ trợ pull user trực tiếp — chỉ Pull SDK mới có)
	// Với ADMS: trả về dữ liệu từ employee_device_mappings đã được ghi khi máy push lên
	if isADMSDevice(d) {
		// Với ADMS: dùng dữ liệu mapping đã được xác nhận sync, không đếm các mapping pending/failed.
		if s.mappingRepo == nil {
			return 0, 0, nil, nil
		}
		mappings, err := s.mappingRepo.ListByDevice(ctx, deviceID)
		if err != nil {
			return 0, 0, nil, err
		}
		for _, m := range mappings {
			if m.DeviceUserID == "" {
				continue
			}

			var emp *entity.Employee
			if m.EmployeeID != "" {
				emp, err = s.employeeRepo.GetByID(ctx, m.EmployeeID)
				if err != nil {
					errList = append(errList, fmt.Sprintf("mapping %s: %v", m.DeviceUserID, err))
					continue
				}
			}
			if emp == nil {
				empID, err := resolveEmployeeIDForADMSPin(ctx, s.employeeRepo, s.mappingRepo, deviceID, m.DeviceUserID)
				if err == nil && empID != "" {
					emp, _ = s.employeeRepo.GetByID(ctx, empID)
				}
			}
			if emp != nil {
				existing++
				if err := s.markEmployeeFingerprintEnrolled(ctx, emp, deviceID, &m); err != nil {
					errList = append(errList, fmt.Sprintf("mark fingerprint for %s: %v", emp.EmployeeCode, err))
				}
			} else {
				// Tạo nhân viên mới từ dữ liệu mapping
				newEmp := &entity.Employee{
					EmployeeCode:        m.DeviceUserID,
					FullName:            m.EmployeeName,
					Status:              "active",
					JoinDate:            time.Now(),
					FingerprintEnrolled: m.FingerprintEnrolled,
				}
				if newEmp.FullName == "" {
					newEmp.FullName = fmt.Sprintf("NV_%s", m.DeviceUserID)
				}
				if err := s.employeeRepo.Create(ctx, newEmp); err != nil {
					errList = append(errList, fmt.Sprintf("create employee %s: %v", m.DeviceUserID, err))
					continue
				}
				imported++
			}
		}
		return imported, existing, errList, nil
	}

	// Pull SDK: kết nối trực tiếp vào máy để lấy danh sách user
	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return 0, 0, nil, err
	}
	cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 15 * time.Second}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return 0, 0, nil, fmt.Errorf("connect to device failed: %w", err)
	}
	defer adapter.Disconnect(ctx)

	deviceEmployees, err := adapter.GetEmployees(ctx)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("get employees from device failed: %w", err)
	}

	for _, de := range deviceEmployees {
		// Kiểm tra xem đã có nhân viên với employee_code này chưa
		existingEmp, err := s.employeeRepo.GetByCode(ctx, de.EmployeeCode)
		if err != nil {
			errList = append(errList, fmt.Sprintf("lookup %s: %v", de.EmployeeCode, err))
			continue
		}

		var webEmployee *entity.Employee
		if existingEmp != nil {
			// Đã có → chỉ cần upsert mapping
			existing++
			webEmployee = existingEmp
			if s.mappingRepo != nil {
				now := time.Now()
				_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
					EmployeeID:   existingEmp.ID,
					DeviceID:     deviceID,
					DeviceUserID: de.EmployeeCode,
					SyncStatus:   "synced",
					LastSyncedAt: &now,
				})
			}
		} else {
			// Chưa có → tạo mới
			newEmp := &entity.Employee{
				EmployeeCode: de.EmployeeCode,
				FullName:     de.FullName,
				CardNo:       de.CardNo,
				Status:       "active",
				JoinDate:     time.Now(),
			}
			if err := s.employeeRepo.Create(ctx, newEmp); err != nil {
				errList = append(errList, fmt.Sprintf("create %s: %v", de.EmployeeCode, err))
				continue
			}
			webEmployee = newEmp
			// Tạo mapping cho nhân viên mới
			if s.mappingRepo != nil {
				now := time.Now()
				_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
					EmployeeID:   newEmp.ID,
					DeviceID:     deviceID,
					DeviceUserID: de.EmployeeCode,
					SyncStatus:   "synced",
					LastSyncedAt: &now,
				})
			}
			imported++
		}

		if webEmployee != nil {
			if err := s.markEmployeeFingerprintEnrolled(ctx, webEmployee, deviceID, nil); err != nil {
				errList = append(errList, fmt.Sprintf("mark fingerprint for %s: %v", de.EmployeeCode, err))
			}
		}

		// A user record alone does not prove that a fingerprint exists. Inspect
		// the device templates so the web status mirrors the physical device.
		for fingerIndex := 0; webEmployee != nil && fingerIndex <= 9; fingerIndex++ {
			fp, fpErr := adapter.GetFingerprint(ctx, de.EmployeeCode, fingerIndex)
			if fpErr != nil || fp == nil {
				continue
			}

			now := time.Now()
			if !webEmployee.FingerprintEnrolled {
				webEmployee.FingerprintEnrolled = true
				if updateErr := s.employeeRepo.Update(ctx, webEmployee); updateErr != nil {
					errList = append(errList, fmt.Sprintf("mark fingerprint for %s: %v", de.EmployeeCode, updateErr))
				}
			}
			if s.mappingRepo != nil {
				if markErr := s.mappingRepo.MarkFingerprintEnrolled(ctx, webEmployee.ID, deviceID, now); markErr != nil {
					errList = append(errList, fmt.Sprintf("mark device fingerprint for %s: %v", de.EmployeeCode, markErr))
				}
			}
			if s.fingerprintRepo != nil {
				fp.EmployeeID = webEmployee.ID
				fp.SourceDeviceID = deviceID
				if saveErr := s.fingerprintRepo.Upsert(ctx, fp); saveErr != nil {
					errList = append(errList, fmt.Sprintf("save fingerprint for %s: %v", de.EmployeeCode, saveErr))
				}
			} else {
				// The badge is correct after the first template; no central store to fill.
				break
			}
		}
	}

	return imported, existing, errList, nil
}

func (s *EmployeeService) failSync(ctx context.Context, hist *entity.SyncHistory, err error) (*entity.SyncHistory, error) {
	hist.Status = entity.SyncStatusFailed
	hist.ErrorMessage = err.Error()
	hist.FinishedAt = time.Now()
	_ = s.syncHistoryRepo.Update(ctx, hist)
	return hist, err
}
