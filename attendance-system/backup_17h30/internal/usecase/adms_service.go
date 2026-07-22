package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

type ADMSService struct {
	deviceRepo      port.DeviceRepository
	attendanceRepo  port.AttendanceLogRepository
	syncHistoryRepo port.SyncHistoryRepository
	mappingRepo     port.EmployeeDeviceMappingRepository
	commandRepo     port.DeviceCommandRepository
	fingerprintRepo port.FingerprintRepository
	employeeRepo    port.EmployeeRepository
	processor       *AttendanceProcessorService
}

func NewADMSService(
	deviceRepo port.DeviceRepository,
	attendanceRepo port.AttendanceLogRepository,
	syncHistoryRepo port.SyncHistoryRepository,
	mappingRepo port.EmployeeDeviceMappingRepository,
	commandRepo port.DeviceCommandRepository,
	fingerprintRepo port.FingerprintRepository,
	employeeRepo port.EmployeeRepository,
	processor *AttendanceProcessorService,
) *ADMSService {
	return &ADMSService{
		deviceRepo:      deviceRepo,
		attendanceRepo:  attendanceRepo,
		syncHistoryRepo: syncHistoryRepo,
		mappingRepo:     mappingRepo,
		commandRepo:     commandRepo,
		fingerprintRepo: fingerprintRepo,
		employeeRepo:    employeeRepo,
		processor:       processor,
	}
}

// RegisterOrGetConfig xử lý đăng ký thiết bị và trả về cấu hình Registry
func (s *ADMSService) RegisterOrGetConfig(ctx context.Context, sn string) (string, error) {
	d, err := s.deviceRepo.GetBySerialADMS(ctx, sn)
	if err != nil {
		return "", err
	}
	if d == nil {
		// Trả về RegistryOK nhưng cảnh báo thiết bị chưa được khai báo
		zap.L().Warn("ADMS request from unregistered device serial", zap.String("serial", sn))
		return "RegistryOK\nDelay=30\nErrorDelay=60\nTransInterval=10\nRealtime=1\n", nil
	}

	// Cập nhật trạng thái và heartbeat
	now := time.Now()
	if err := s.deviceRepo.UpdateHeartbeat(ctx, d.ID, now); err != nil {
		zap.L().Error("failed to update heartbeat", zap.String("device_id", d.ID), zap.Error(err))
	}

	// Trả về cấu hình chuẩn ZKTeco ADMS
	configLines := []string{
		"RegistryOK",
		"Delay=10",
		"ErrorDelay=30",
		"TransInterval=1",
		"Realtime=1",
		"Encrypt=None",
	}
	return strings.Join(configLines, "\n") + "\n", nil
}

// ProcessIncomingData xử lý log gửi lên từ máy chấm công
func inferADMSDataTable(table, body string) string {
	trimmedTable := strings.ToUpper(strings.TrimSpace(table))
	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return trimmedTable
	}

	upperBody := strings.ToUpper(trimmedBody)
	if strings.Contains(upperBody, "FINGERTEMPLATE") || strings.Contains(upperBody, "BIODATA") || strings.Contains(upperBody, "TEMPLATE=") || strings.Contains(upperBody, "TMP=") || strings.Contains(upperBody, "FID=") || strings.Contains(upperBody, "FINGERID=") || strings.HasPrefix(upperBody, "FP ") || strings.HasPrefix(upperBody, "FP\t") {
		return "BIODATA"
	}
	return trimmedTable
}

func (s *ADMSService) ProcessIncomingData(ctx context.Context, sn string, table string, body string) (string, error) {
	d, err := s.deviceRepo.GetBySerialADMS(ctx, sn)
	if err != nil {
		return "OK\n", err
	}
	if d == nil {
		return "OK\n", fmt.Errorf("device not found for serial: %s", sn)
	}

	// Cập nhật heartbeat
	_ = s.deviceRepo.UpdateHeartbeat(ctx, d.ID, time.Now())

	resolvedTable := inferADMSDataTable(table, body)
	switch strings.ToUpper(resolvedTable) {
	case "ATTLOG":
		inserted, err := s.parseAndSaveAttLogs(ctx, d.ID, d.Name, body)
		if err != nil {
			fmt.Printf("[ADMS] parseAndSaveAttLogs error device=%s: %v\n", d.ID, err)
		} else {
			fmt.Printf("[ADMS] parseAndSaveAttLogs OK inserted=%d\n", inserted)
		}
		// Luôn trả về OK chuẩn ADMS để máy không gửi lại
		return "OK\n", nil
	case "BIODATA", "FINGERTEMPLATE":
		zap.L().Info("[ADMS] Received biometric payload", zap.String("serial", sn), zap.String("table", resolvedTable), zap.String("body_preview", truncateForLog(body, 200)))
		inserted, err := s.parseAndSaveBioData(ctx, d.ID, body)
		if err != nil {
			fmt.Printf("[ADMS] parseAndSaveBioData error device=%s: %v\n", d.ID, err)
		} else {
			fmt.Printf("[ADMS] parseAndSaveBioData OK inserted=%d\n", inserted)
		}
		// Luôn trả về OK chuẩn ADMS để máy không gửi lại
		return "OK\n", nil
	default:
		// Các bảng khác (OPERLOG, vv) trả về OK để máy không gửi lại
		return "OK\n", nil
	}
}

// GetPendingCommand trả về lệnh tiếp theo trong hàng đợi cho thiết bị
func (s *ADMSService) GetPendingCommand(ctx context.Context, sn string) (string, error) {
	d, err := s.deviceRepo.GetBySerialADMS(ctx, sn)
	if err != nil {
		return "OK\n", err
	}
	if d == nil {
		return "OK\n", fmt.Errorf("device not found for serial: %s", sn)
	}

	_ = s.deviceRepo.UpdateHeartbeat(ctx, d.ID, time.Now())

	cmds, err := s.commandRepo.GetPending(ctx, d.ID)
	if err != nil {
		return "OK\n", err
	}

	if len(cmds) == 0 {
		fmt.Printf("[ADMS] GETREQUEST SN=%s no pending command\n", sn)
		return "OK\n", nil
	}

	// Gửi lệnh đầu tiên trong queue
	cmd := cmds[0]
	fmt.Printf("[ADMS] GETREQUEST SN=%s sending cmd_id=%d row_id=%d command=%q\n", sn, cmd.CommandID, cmd.ID, cmd.Command)
	if err := s.commandRepo.MarkSent(ctx, cmd.ID); err != nil {
		return "OK\n", err
	}

	// Định dạng: C:CommandID:CommandString
	resp := fmt.Sprintf("C:%d:%s\n", cmd.CommandID, cmd.Command)
	return resp, nil
}

// ConfirmCommand xử lý phản hồi từ thiết bị cho một lệnh
func (s *ADMSService) ConfirmCommand(ctx context.Context, sn string, body string) (string, error) {
	d, err := s.deviceRepo.GetBySerialADMS(ctx, sn)
	if err != nil {
		return "OK\n", err
	}
	if d == nil {
		return "OK\n", fmt.Errorf("device not found for serial: %s", sn)
	}

	_ = s.deviceRepo.UpdateHeartbeat(ctx, d.ID, time.Now())

	// Body có định dạng: ID=101&Return=0 hoặc ID=101\tReturn=0 hoặc ID=101\nReturn=0
	// Ta viết bộ parse linh hoạt
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var cmdID int64
		var retCode *int
		hasID := false

		// Parse key-values from common ADMS bodies such as:
		//   ID=101&Return=0
		//   ID=101	Return=0
		//   ID=101
		pairs := strings.FieldsFunc(line, func(r rune) bool {
			return r == '\t' || r == '&' || r == ' '
		})

		for _, p := range pairs {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToUpper(strings.TrimSpace(kv[0]))
			v := strings.TrimSpace(kv[1])
			if k == "ID" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					cmdID = id
					hasID = true
				}
			} else if k == "RETURN" {
				if rVal, err := strconv.Atoi(v); err == nil {
					retCode = &rVal
				}
			}
		}

		if !hasID {
			continue
		}

		// Treat missing/empty return value as success. The device may acknowledge
		// with only ID=... or with ID=...&Return=0. Marking missing return as failed
		// causes commands to disappear from the queue even though they were accepted.
		if retCode == nil || *retCode == 0 {
			_ = s.commandRepo.Ack(ctx, d.ID, cmdID)
			if cmd, err := s.commandRepo.GetByDeviceIDAndCommandID(ctx, d.ID, cmdID); err == nil && cmd != nil {
				zap.L().Info("[ADMS] Command ACK received", zap.String("serial", sn), zap.Int64("cmd_id", cmdID), zap.String("command", cmd.Command))
				if pin := extractPinFromADMSCommand(cmd.Command); pin != "" {
					if err := s.markDeviceUserSynced(ctx, d.ID, pin); err != nil {
						zap.L().Warn("[ADMS] failed to mark synced user after ACK", zap.String("serial", sn), zap.Int64("cmd_id", cmdID), zap.String("pin", pin), zap.Error(err))
					}
				}
			} else {
				zap.L().Info("[ADMS] Command ACK received (no command found)", zap.String("serial", sn), zap.Int64("cmd_id", cmdID))
			}
			continue
		}

		zap.L().Warn("[ADMS] Command executed with non-zero return code",
			zap.String("serial", sn),
			zap.Int64("cmd_id", cmdID),
			zap.Int("return_code", *retCode),
		)
		_ = s.commandRepo.MarkFailedByDeviceCmdID(ctx, d.ID, cmdID)

		cmd, err := s.commandRepo.GetByDeviceIDAndCommandID(ctx, d.ID, cmdID)
		if err == nil && cmd != nil {
			originalCommand := strings.TrimSpace(cmd.Command)
			upperCmd := strings.ToUpper(originalCommand)
			if strings.HasPrefix(upperCmd, "ENROLL_INFO ") {
				pin := extractPinFromADMSCommand(originalCommand)
				if pin != "" {
					fallbackCmd := buildADMSFallbackEnrollCommand(pin)
					_, _ = s.commandRepo.Enqueue(ctx, d.ID, fallbackCmd)
					zap.L().Info("[ADMS] Enqueued fallback enroll command after ENROLL_INFO failure",
						zap.String("serial", sn),
						zap.Int64("cmd_id", cmdID),
						zap.String("fallback_command", fallbackCmd),
					)
				}
			} else if strings.HasPrefix(upperCmd, "DATA UPDATE") {
				pin := extractPinFromADMSCommand(originalCommand)
				if pin != "" {
					var fallbackCmd string
					if strings.Contains(upperCmd, "FINGERTEMPLATE") || strings.Contains(upperCmd, "BIODATA") || strings.Contains(upperCmd, "TEMPLATE=") || strings.Contains(upperCmd, "TMP=") {
						// Try a fingerprint-compatible variant when the device rejects the current biometric payload.
						var fingerIndex int
						var size int
						var template string
						if parsed := parseADMSBiometricPayload(originalCommand); parsed != nil {
							fingerIndex = parsed.FingerIndex
							size = parsed.TemplateSize
							template = parsed.TemplateData
						}
						fallbackCmd = nextADMSFingerprintCommandVariant(pin, fingerIndex, size, template, originalCommand)
						if fallbackCmd != "" {
							_, _ = s.commandRepo.Enqueue(ctx, d.ID, fallbackCmd)
							zap.L().Info("[ADMS] Enqueued alternate fingerprint command after rejection",
								zap.String("serial", sn),
								zap.Int64("cmd_id", cmdID),
								zap.String("fallback_command", fallbackCmd),
							)
						}
					} else {
						// Try a different ADMS user payload variant when the device rejects the current one.
						var fullName, cardNo string
						if parsed := parseADMSUserPayload(originalCommand); parsed != nil {
							fullName = parsed.FullName
							cardNo = parsed.CardNo
						}
						fallbackCmd = nextADMSUserCommandVariant(pin, fullName, cardNo, originalCommand)
						if fallbackCmd != "" {
							_, _ = s.commandRepo.Enqueue(ctx, d.ID, fallbackCmd)
							zap.L().Info("[ADMS] Enqueued alternate user command after rejection",
								zap.String("serial", sn),
								zap.Int64("cmd_id", cmdID),
								zap.String("fallback_command", fallbackCmd),
							)
						}
					}
				}
			}
		}
	}

	return "OK\n", nil
}

func truncateForLog(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func extractPinFromADMSCommand(command string) string {
	trimmed := strings.TrimSpace(command)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "ENROLL_INFO") {
		return ""
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\t' || r == ' '
	})
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), "PIN") {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

func (s *ADMSService) markDeviceUserSynced(ctx context.Context, deviceID string, pin string) error {
	if s.mappingRepo == nil || s.employeeRepo == nil {
		return nil
	}

	trimmedPin := strings.TrimSpace(pin)
	if trimmedPin == "" {
		return nil
	}
	normalizedPin := normalizeADMSPin(trimmedPin)

	var mapping *entity.EmployeeDeviceMapping
	var err error
	if mapping, err = s.mappingRepo.GetByDeviceUserID(ctx, deviceID, trimmedPin); err != nil {
		return err
	}
	if mapping == nil && normalizedPin != "" && normalizedPin != trimmedPin {
		mapping, err = s.mappingRepo.GetByDeviceUserID(ctx, deviceID, normalizedPin)
		if err != nil {
			return err
		}
	}

	empID, err := resolveEmployeeIDForADMSPin(ctx, s.employeeRepo, s.mappingRepo, deviceID, trimmedPin)
	if err != nil {
		return err
	}
	if empID == "" && normalizedPin != "" && normalizedPin != trimmedPin {
		empID, err = resolveEmployeeIDForADMSPin(ctx, s.employeeRepo, s.mappingRepo, deviceID, normalizedPin)
		if err != nil {
			return err
		}
	}
	if empID == "" {
		return nil
	}

	now := time.Now()
	if mapping == nil {
		mapping, err = s.mappingRepo.GetByEmployeeAndDevice(ctx, empID, deviceID)
		if err != nil {
			return err
		}
	}
	if mapping == nil {
		mapping = &entity.EmployeeDeviceMapping{EmployeeID: empID, DeviceID: deviceID}
	}
	mapping.EmployeeID = empID
	mapping.DeviceID = deviceID
	mapping.DeviceUserID = trimmedPin
	mapping.SyncStatus = "synced"
	mapping.LastSyncedAt = &now
	return s.mappingRepo.Upsert(ctx, mapping)
}

func (s *ADMSService) parseAndSaveAttLogs(ctx context.Context, deviceID string, deviceName string, body string) (int, error) {
	lines := strings.Split(body, "\n")
	var logs []entity.AttendanceLog

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PIN") {
			// Bỏ qua dòng trống hoặc header
			continue
		}

		// ZKTeco ATTLOG format thường là:
		// PIN \t Date Time \t Status \t Verify \t Workcode ...
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			// Thử split theo space
			parts = strings.Fields(line)
		}
		if len(parts) < 2 {
			continue
		}

		pin := strings.TrimSpace(parts[0])
		timeStr := strings.TrimSpace(parts[1])

		// Parse time
		var checkTime time.Time
		var err error
		timeFormats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"06-01-02 15:04:05",
			time.RFC3339,
		}
		for _, f := range timeFormats {
			checkTime, err = time.ParseInLocation(f, timeStr, time.Local)
			if err == nil {
				break
			}
		}
		if err != nil {
			continue // không parse được thời gian
		}

		status := "0"
		if len(parts) > 2 {
			status = strings.TrimSpace(parts[2])
		}

		verify := "1"
		if len(parts) > 3 {
			verify = strings.TrimSpace(parts[3])
		}

		// Mapping CheckType
		checkType := entity.CheckTypeIn
		if status == "1" {
			checkType = entity.CheckTypeOut
		} else if status == "4" || status == "5" {
			checkType = entity.CheckTypeIn // OT
		}

		// Mapping VerifyMode
		verifyMode := entity.VerifyModeFingerprint
		if verify == "3" || verify == "4" {
			verifyMode = entity.VerifyModeCard
		} else if verify == "15" || verify == "20" {
			verifyMode = entity.VerifyModeFace
		}

		// Translate device user ID to employee code
		empCode := pin
		if s.mappingRepo != nil {
			mapping, _ := s.mappingRepo.GetByDeviceUserID(ctx, deviceID, pin)
			if mapping != nil {
				empCode = mapping.EmployeeCode
			}
		}

		// Bọc raw line thành JSON hợp lệ để lưu vào cột JSONB
		rawJSON, _ := json.Marshal(map[string]string{
			"raw":    line,
			"pin":    pin,
			"time":   timeStr,
			"status": status,
			"verify": verify,
			"source": "adms_push",
		})

		logs = append(logs, entity.AttendanceLog{
			DeviceID:     deviceID,
			EmployeeCode: empCode,
			CheckTime:    checkTime,
			CheckType:    checkType,
			VerifyMode:   verifyMode,
			SyncedAt:     time.Now(),
			RawPayload:   rawJSON,
		})
	}

	if len(logs) == 0 {
		return 0, nil
	}

	// Ghi nhận lịch sử đồng bộ
	hist := &entity.SyncHistory{
		DeviceID:    deviceID,
		SyncType:    entity.SyncTypeAttendance,
		TriggerType: "adms_push",
		StartedAt:   time.Now(),
	}
	_ = s.syncHistoryRepo.Create(ctx, hist)

	inserted, err := s.attendanceRepo.BulkInsert(ctx, logs)
	if err != nil {
		hist.RecordCount = inserted
		hist.Status = entity.SyncStatusFailed
		hist.ErrorMessage = err.Error()
		hist.FinishedAt = time.Now()
		_ = s.syncHistoryRepo.Update(ctx, hist)
		return inserted, err
	}

	hist.RecordCount = inserted
	hist.Status = entity.SyncStatusSuccess
	hist.FinishedAt = time.Now()
	_ = s.syncHistoryRepo.Update(ctx, hist)

	// Broadcast realtime via SSE — luôn broadcast khi nhận được log hợp lệ từ máy
	// (kể cả inserted==0 do trùng lặp, vẫn cần notify UI để load lại data mới nhất)
	latestLog := logs[len(logs)-1]
	broadcast.Global.Broadcast("attendance_synced", map[string]any{
		"device_id":            deviceID,
		"device_name":          deviceName,
		"inserted":             inserted,
		"total_parsed":         len(logs),
		"latest_employee_code": latestLog.EmployeeCode,
		"latest_check_time":    latestLog.CheckTime.Format(time.RFC3339),
		"latest_check_type":    string(latestLog.CheckType),
	})

	// Tính toán công tự động (chỉ khi có bản ghi mới thực sự)
	if inserted > 0 && s.processor != nil {
		dateMap := make(map[string]time.Time)
		for _, l := range logs {
			dStr := l.CheckTime.Format("2006-01-02")
			if _, ok := dateMap[dStr]; !ok {
				dateMap[dStr] = l.CheckTime
			}
		}
		for _, t := range dateMap {
			go func(targetDate time.Time) {
				procCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = s.processor.ProcessDailyAttendance(procCtx, targetDate)
			}(t)
		}
	}

	return inserted, nil
}

func resolveEmployeeIDForADMSPin(ctx context.Context, employeeRepo port.EmployeeRepository, mappingRepo port.EmployeeDeviceMappingRepository, deviceID, pin string) (string, error) {
	trimmedPin := strings.TrimSpace(pin)
	if trimmedPin == "" {
		return "", nil
	}

	if mappingRepo != nil {
		if mapping, err := mappingRepo.GetByDeviceUserID(ctx, deviceID, trimmedPin); err == nil && mapping != nil && mapping.EmployeeID != "" {
			return mapping.EmployeeID, nil
		}
		if normalizedPin := normalizeADMSPin(trimmedPin); normalizedPin != "" && normalizedPin != trimmedPin {
			if mapping, err := mappingRepo.GetByDeviceUserID(ctx, deviceID, normalizedPin); err == nil && mapping != nil && mapping.EmployeeID != "" {
				return mapping.EmployeeID, nil
			}
		}
	}

	if employeeRepo != nil {
		if emp, err := employeeRepo.GetByCode(ctx, trimmedPin); err == nil && emp != nil && emp.ID != "" {
			return emp.ID, nil
		}
		if normalizedPin := normalizeADMSPin(trimmedPin); normalizedPin != "" && normalizedPin != trimmedPin {
			if emp, err := employeeRepo.GetByCode(ctx, normalizedPin); err == nil && emp != nil && emp.ID != "" {
				return emp.ID, nil
			}
		}
		if employees, err := employeeRepo.List(ctx); err == nil {
			for _, emp := range employees {
				if emp.ID == "" {
					continue
				}
				employeeCode := strings.TrimSpace(emp.EmployeeCode)
				if employeeCode == "" {
					continue
				}
				if strings.EqualFold(employeeCode, trimmedPin) || strings.EqualFold(employeeCode, normalizeADMSPin(trimmedPin)) || strings.EqualFold(normalizeADMSPin(employeeCode), normalizeADMSPin(trimmedPin)) {
					return emp.ID, nil
				}
			}
		}
	}

	return "", nil
}

type admsBiometricPayload struct {
	Pin          string
	FingerIndex  int
	TemplateData string
	TemplateSize int
}

func parseADMSBiometricPayload(raw string) *admsBiometricPayload {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	payload := &admsBiometricPayload{}

	// Accept payloads with a command prefix like: DATA UPDATE FINGERTEMPLATE PIN=... FID=1 TMP=...
	// or just the bare field list: PIN=... FingerID=1 Template=...
	line := trimmed
	if idx := strings.Index(line, " "); idx > 0 {
		// Keep the original content but also allow prefixes like DATA UPDATE BIODATA
		// by splitting on spaces only after a known command keyword.
		line = strings.TrimSpace(line)
	}

	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == '\t' || r == ' '
	})

	for _, part := range parts {
		if strings.HasPrefix(strings.ToUpper(part), "FP") && strings.Contains(strings.ToUpper(part), "PIN") {
			continue
		}
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(part), "DATA") || strings.HasPrefix(strings.ToUpper(part), "UPDATE") || strings.HasPrefix(strings.ToUpper(part), "FINGERTEMPLATE") || strings.HasPrefix(strings.ToUpper(part), "BIODATA") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])
		switch key {
		case "pin":
			payload.Pin = value
		case "fingerid", "fid", "index", "no":
			if val, err := strconv.Atoi(value); err == nil {
				payload.FingerIndex = val
			}
		case "template", "tmp", "val", "value":
			if value != "" {
				payload.TemplateData = value
			}
		case "size", "len":
			if val, err := strconv.Atoi(value); err == nil {
				payload.TemplateSize = val
			}
		case "valid":
			// Ignore validity flag; just treat the presence of a template as success.
			continue
		}
	}

	if payload.Pin == "" || payload.TemplateData == "" || payload.FingerIndex < 0 {
		return nil
	}
	return payload
}

func (s *ADMSService) parseAndSaveBioData(ctx context.Context, deviceID string, body string) (int, error) {
	lines := strings.Split(body, "\n")
	inserted := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		payload := parseADMSBiometricPayload(line)
		if payload == nil {
			continue
		}

		pin := payload.Pin
		fingerIndex := payload.FingerIndex
		templateData := payload.TemplateData
		templateSize := payload.TemplateSize

		if pin != "" && fingerIndex >= 0 && templateData != "" {
			// Tìm employee từ mapping hoặc employee_code, hỗ trợ cả PIN chuẩn hóa như 001 <-> EMP001
			empID, err := resolveEmployeeIDForADMSPin(ctx, s.employeeRepo, s.mappingRepo, deviceID, pin)
			if err != nil {
				zap.L().Warn("failed to resolve employee for ADMS biometric payload", zap.String("device_id", deviceID), zap.String("pin", pin), zap.Error(err))
				continue
			}
			if empID == "" {
				// Không tìm thấy employee, bỏ qua record này
				continue
			}

			if err := s.markDeviceUserSynced(ctx, deviceID, pin); err != nil {
				zap.L().Warn("failed to mark synced user for biometric payload", zap.String("device_id", deviceID), zap.String("pin", pin), zap.Error(err))
			}

			fp := &entity.EmployeeFingerprint{
				EmployeeID:     empID,
				FingerIndex:    fingerIndex,
				TemplateData:   templateData,
				TemplateSize:   templateSize,
				AlgoVersion:    "10.0", // Mặc định ZKTeco ADMS
				SourceDeviceID: deviceID,
			}

			if err := s.fingerprintRepo.Upsert(ctx, fp); err != nil {
				zap.L().Warn("failed to save ADMS biometric fingerprint", zap.String("employee_id", empID), zap.Int("finger_index", fingerIndex), zap.Error(err))
			} else {
				inserted++
				if emp, getErr := s.employeeRepo.GetByID(ctx, empID); getErr == nil && emp != nil {
					if !emp.FingerprintEnrolled {
						emp.FingerprintEnrolled = true
						if updateErr := s.employeeRepo.Update(ctx, emp); updateErr != nil {
							zap.L().Warn("[ADMS] failed to mark employee fingerprint as enrolled", zap.String("employee_id", empID), zap.Error(updateErr))
						}
					}
					if s.mappingRepo != nil {
						if mapping, getMapErr := s.mappingRepo.GetByEmployeeAndDevice(ctx, emp.ID, deviceID); getMapErr == nil && mapping != nil {
							if !mapping.FingerprintEnrolled {
								now := time.Now()
								_ = s.mappingRepo.MarkFingerprintEnrolled(ctx, emp.ID, deviceID, now)
								mapping.FingerprintEnrolled = true
								mapping.FingerprintAt = &now
								_ = s.mappingRepo.Upsert(ctx, mapping)
							}
						} else if getMapErr == nil {
							now := time.Now()
							_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
								EmployeeID:          emp.ID,
								DeviceID:            deviceID,
								DeviceUserID:        pin,
								SyncStatus:          "synced",
								FingerprintEnrolled: true,
								FingerprintAt:       &now,
							})
						}
					}
				}

				// Phát event realtime cho UI rằng vân tay mới đã được nhận
				// Nếu có mappingRepo, đảm bảo có mapping và đánh dấu enrolled
				if s.mappingRepo != nil {
					m, _ := s.mappingRepo.GetByDeviceUserID(ctx, deviceID, pin)
					now := time.Now()
					if m == nil {
						_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
							EmployeeID:          empID,
							DeviceID:            deviceID,
							DeviceUserID:        pin,
							SyncStatus:          "synced",
							FingerprintEnrolled: true,
							FingerprintAt:       &now,
						})
					} else {
						_ = s.mappingRepo.MarkFingerprintEnrolled(ctx, empID, deviceID, now)
						_ = s.mappingRepo.Upsert(ctx, &entity.EmployeeDeviceMapping{
							EmployeeID:          empID,
							DeviceID:            deviceID,
							DeviceUserID:        pin,
							SyncStatus:          "synced",
							FingerprintEnrolled: true,
							FingerprintAt:       &now,
						})
					}
				}

				// Phát event realtime cho UI rằng vân tay mới đã được nhận
				broadcast.Global.Broadcast("fingerprint_updated", map[string]any{
					"employee_id":  empID,
					"device_id":    deviceID,
					"finger_index": fingerIndex,
				})

				// Tự động kích hoạt đồng bộ vân tay sang các thiết bị khác (Biometric Sync)
				// Để tránh vòng lặp đồng bộ, ta gọi ngầm propagation service
				go func(eID string, srcDevID string) {
					bgCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
					defer cancel()

					emp, err := s.employeeRepo.GetByID(bgCtx, eID)
					if err != nil || emp == nil {
						return
					}

					fps, err := s.fingerprintRepo.ListByEmployee(bgCtx, eID)
					if err != nil || len(fps) == 0 {
						return
					}

					devices, err := s.deviceRepo.List(bgCtx)
					if err != nil {
						return
					}

					enqueuedCount := 0
					for _, d := range devices {
						if !isADMSDevice(d) || d.ID == srcDevID {
							continue
						}
						// Thiết bị phải hoạt động trong 10 phút qua
						if !isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
							continue
						}

						pin := emp.EmployeeCode
						if s.mappingRepo != nil {
							mapping, _ := s.mappingRepo.GetByEmployeeAndDevice(bgCtx, eID, d.ID)
							if mapping != nil {
								pin = mapping.DeviceUserID
							}
						}

						// 1. Tạo lệnh USER
						userCmd := buildADMSUserCommand(pin, emp.FullName, emp.CardNo)
						_, _ = s.commandRepo.Enqueue(bgCtx, d.ID, userCmd)

						// 2. Tạo lệnh FINGERTEMPLATE
						for _, f := range fps {
							fpCmd := fmt.Sprintf("DATA UPDATE fingertemplate Pin=%s\tFingerID=%d\tSize=%d\tVal=1\tTemplate=%s",
								pin, f.FingerIndex, f.TemplateSize, f.TemplateData)
							_, _ = s.commandRepo.Enqueue(bgCtx, d.ID, fpCmd)
							enqueuedCount++
						}
					}

					if enqueuedCount > 0 {
						broadcast.Global.Broadcast("fingerprint_synced", map[string]any{
							"employee_id": eID,
							"commands":    enqueuedCount,
						})
					}
				}(empID, deviceID)
			}
		}
	}

	return inserted, nil
}
