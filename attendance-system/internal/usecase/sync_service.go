package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

const maxRetry = 3

// SyncService điều phối luồng đồng bộ dữ liệu chấm công (raw log) từ thiết bị về DB.
// Bản thân service KHÔNG tính công — chỉ lưu raw log để hệ thống khác (payroll/HRM) sử dụng.
type SyncService struct {
	deviceRepo        port.DeviceRepository
	attendanceRepo    port.AttendanceLogRepository
	syncHistoryRepo   port.SyncHistoryRepository
	factory           port.DeviceAdapterFactory
	cursorRepo        port.SyncCursorRepository
	mappingRepo       port.EmployeeDeviceMappingRepository
	employeeRepo      port.EmployeeRepository
	processor         *AttendanceProcessorService
	notifier          *NotificationService
	sdkConnectTimeout time.Duration
	sdkMaxRetries     int
}

func (s *SyncService) SetProcessor(processor *AttendanceProcessorService) {
	s.processor = processor
}

func (s *SyncService) SetEmployeeRepo(repo port.EmployeeRepository) {
	s.employeeRepo = repo
}

// SetNotificationService gắn bộ gửi thông báo cho log SDK mới.
func (s *SyncService) SetNotificationService(notifier *NotificationService) {
	s.notifier = notifier
}

func (s *SyncService) GetAdapterFactory() port.DeviceAdapterFactory {
	return s.factory
}

// SetSDKConnectionPolicy configures only standalone SDK devices. ADMS devices
// keep the historical connection/retry policy so enabling this feature cannot
// change their push behavior.
func (s *SyncService) SetSDKConnectionPolicy(timeout time.Duration, maxRetries int) {
	if timeout > 0 {
		s.sdkConnectTimeout = timeout
	}
	if maxRetries > 0 {
		s.sdkMaxRetries = maxRetries
	}
}

func NewSyncServiceWithCursor(deviceRepo port.DeviceRepository, attendanceRepo port.AttendanceLogRepository, syncHistoryRepo port.SyncHistoryRepository, cursorRepo port.SyncCursorRepository, factory port.DeviceAdapterFactory, mappingRepos ...port.EmployeeDeviceMappingRepository) *SyncService {
	s := NewSyncService(deviceRepo, attendanceRepo, syncHistoryRepo, factory)
	s.cursorRepo = cursorRepo
	if len(mappingRepos) > 0 {
		s.mappingRepo = mappingRepos[0]
	}
	return s
}

func (s *SyncService) SyncAttendanceFromCursor(ctx context.Context, deviceID string, trigger entity.SyncTriggerType) (*entity.SyncHistory, error) {
	to := time.Now()
	from := to.AddDate(0, 0, -30)
	if s.cursorRepo != nil {
		cursor, err := s.cursorRepo.GetAttendanceCursor(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		if cursor != nil {
			from = cursor.Add(-2 * time.Minute)
		}
	}
	h, err := s.SyncAttendance(ctx, deviceID, from, to, trigger)
	if err == nil && s.cursorRepo != nil {
		if err = s.cursorRepo.SetAttendanceCursor(ctx, deviceID, to); err != nil {
			return nil, err
		}
	}
	return h, err
}

func NewSyncService(
	deviceRepo port.DeviceRepository,
	attendanceRepo port.AttendanceLogRepository,
	syncHistoryRepo port.SyncHistoryRepository,
	factory port.DeviceAdapterFactory,
) *SyncService {
	return &SyncService{
		deviceRepo:      deviceRepo,
		attendanceRepo:  attendanceRepo,
		syncHistoryRepo: syncHistoryRepo,
		factory:         factory,
	}
}

// SyncAttendance thực hiện đồng bộ log chấm công cho 1 thiết bị, theo đúng luồng:
// Connect -> GetAttendanceLogs -> validate/de-dup -> lưu attendance_log -> ghi sync_history -> Disconnect
func (s *SyncService) SyncAttendance(ctx context.Context, deviceID string, from, to time.Time, trigger entity.SyncTriggerType) (*entity.SyncHistory, error) {
	hist := &entity.SyncHistory{
		DeviceID:    deviceID,
		SyncType:    entity.SyncTypeAttendance,
		TriggerType: trigger,
		StartedAt:   time.Now(),
	}
	if err := s.syncHistoryRepo.Create(ctx, hist); err != nil {
		return nil, err
	}

	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return s.fail(ctx, hist, err)
	}
	if d == nil {
		return s.fail(ctx, hist, fmt.Errorf("device not found: %s", deviceID))
	}

	// Attendance collection through the device adapter is deliberately enabled
	// even when ADMS Push is configured. ADMS may continue to deliver ATTLOG in
	// real time, while this SDK/PULL path acts as an additional collection path
	// and recovers records missed during an ADMS outage. The attendance table's
	// UNIQUE(device_id, employee_code, check_time) constraint de-duplicates a
	// scan that arrives through both protocols.

	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return s.fail(ctx, hist, err)
	}

	connectTimeout := 10 * time.Second
	maxAttempts := maxRetry
	if !isADMSDevice(d) {
		if s.sdkConnectTimeout > 0 {
			connectTimeout = s.sdkConnectTimeout
		}
		if s.sdkMaxRetries > 0 {
			maxAttempts = s.sdkMaxRetries
		}
	}
	cfg := port.DeviceConfig{
		IPAddress: d.IPAddress,
		Port:      d.Port,
		Username:  d.Username,
		Password:  d.Password,
		Timeout:   connectTimeout,
	}

	var lastErr error
	connected := false
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := adapter.Connect(ctx, cfg); err != nil {
			lastErr = err
			if err := sleepWithContext(ctx, time.Duration(attempt)*time.Second); err != nil {
				return s.fail(ctx, hist, err)
			}
			continue
		}
		connected = true
		break
	}
	if !connected {
		return s.fail(ctx, hist, fmt.Errorf("connect failed after %d attempts: %w", maxAttempts, lastErr))
	}
	defer adapter.Disconnect(ctx)

	logs, err := adapter.GetAttendanceLogs(ctx, from, to)
	if err != nil {
		return s.fail(ctx, hist, err)
	}

	// validate: loại bỏ bản ghi thiếu employee_code hoặc check_time
	valid := make([]entity.AttendanceLog, 0, len(logs))
	for _, l := range logs {
		if l.EmployeeCode == "" || l.CheckTime.IsZero() {
			continue
		}
		l.DeviceID = deviceID
		if s.mappingRepo != nil {
			mapping, mapErr := s.findDeviceMapping(ctx, deviceID, l.EmployeeCode)
			if mapErr != nil {
				return s.fail(ctx, hist, mapErr)
			}
			if mapping != nil {
				l.EmployeeCode = mapping.EmployeeCode
			}
		}
		l.SyncedAt = time.Now()
		valid = append(valid, l)
	}

	// de-duplicate được đảm bảo ở tầng DB qua UNIQUE(device_id, employee_code, check_time)
	var newLogs []entity.AttendanceLog
	if s.notifier != nil {
		newLogs = filterNewAttendanceLogs(ctx, s.attendanceRepo, valid)
	}
	inserted, err := s.attendanceRepo.BulkInsert(ctx, valid)
	if err != nil {
		hist.RecordCount = inserted
		return s.fail(ctx, hist, err)
	}

	hist.RecordCount = inserted
	hist.Status = entity.SyncStatusSuccess
	hist.FinishedAt = time.Now()
	_ = s.syncHistoryRepo.Update(ctx, hist)

	_ = s.deviceRepo.UpdateStatus(ctx, deviceID, "online", time.Now())

	if inserted > 0 && len(valid) > 0 {
		latestLog := valid[len(valid)-1]

		empName := "Nhân viên"
		avatarURL := ""
		deptName := ""
		if s.employeeRepo != nil {
			emp, _ := s.employeeRepo.GetByCode(ctx, latestLog.EmployeeCode)
			if emp != nil {
				if emp.FullName != "" {
					empName = emp.FullName
				}
				avatarURL = emp.AvatarURL
				deptName = emp.DepartmentID
			}
		}

		broadcast.Global.Broadcast("fingerprint_scanned_realtime", map[string]any{
			"device_id":            deviceID,
			"device_name":          d.Name,
			"employee_code":        latestLog.EmployeeCode,
			"employee_name":        empName,
			"department":           deptName,
			"avatar_url":           avatarURL,
			"check_time":           latestLog.CheckTime.Format("2006-01-02 15:04:05"),
			"check_type":           string(latestLog.CheckType),
			"verify_mode":          string(latestLog.VerifyMode),
		})

		broadcast.Global.Broadcast("attendance_synced", map[string]any{
			"device_id":            deviceID,
			"inserted":             inserted,
			"device_name":          d.Name,
			"latest_employee_code": latestLog.EmployeeCode,
			"latest_check_time":    latestLog.CheckTime.Format(time.RFC3339),
			"latest_check_type":    string(latestLog.CheckType),
			"latest_verify_mode":   string(latestLog.VerifyMode),
		})
		if s.notifier != nil && len(newLogs) > 0 {
			go s.notifier.NotifyAttendanceLogs(context.Background(), newLogs)
		}
	}
	return hist, nil
}

// ProcessLiveAttendanceLogs xử lý và broadcast log chấm công real-time thu thập trực tiếp từ COM SDK listener.
// Phương thức này KHÔNG ghi sync_history rác và KHÔNG mở kết nối thứ hai đến thiết bị.
func (s *SyncService) ProcessLiveAttendanceLogs(ctx context.Context, deviceID string, logs []entity.AttendanceLog) (int, error) {
	if len(logs) == 0 {
		return 0, nil
	}
	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil || d == nil {
		return 0, err
	}

	var valid []entity.AttendanceLog
	for _, l := range logs {
		if l.EmployeeCode == "" {
			continue
		}
		l.DeviceID = deviceID
		if s.mappingRepo != nil {
			mapping, mapErr := s.findDeviceMapping(ctx, deviceID, l.EmployeeCode)
			if mapErr == nil && mapping != nil {
				l.EmployeeCode = mapping.EmployeeCode
			}
		}
		l.SyncedAt = time.Now()
		valid = append(valid, l)
	}

	if len(valid) == 0 {
		return 0, nil
	}

	var newLogs []entity.AttendanceLog
	if s.notifier != nil {
		newLogs = filterNewAttendanceLogs(ctx, s.attendanceRepo, valid)
	}

	inserted, err := s.attendanceRepo.BulkInsert(ctx, valid)
	if err != nil {
		return 0, err
	}

	_ = s.deviceRepo.UpdateStatus(ctx, deviceID, "online", time.Now())

	if inserted > 0 && len(valid) > 0 {
		latestLog := valid[len(valid)-1]

		empName := "Nhân viên"
		avatarURL := ""
		deptName := ""
		if s.employeeRepo != nil {
			emp, _ := s.employeeRepo.GetByCode(ctx, latestLog.EmployeeCode)
			if emp != nil {
				if emp.FullName != "" {
					empName = emp.FullName
				}
				avatarURL = emp.AvatarURL
				deptName = emp.DepartmentID
			}
		}

		fmt.Printf("⚡ [PROCESS LIVE LOGS] BROADCASTING fingerprint_scanned_realtime for Employee: %s (%s) at %s\n",
			latestLog.EmployeeCode, empName, latestLog.CheckTime.Format("2006-01-02 15:04:05"))

		broadcast.Global.Broadcast("fingerprint_scanned_realtime", map[string]any{
			"device_id":            deviceID,
			"device_name":          d.Name,
			"employee_code":        latestLog.EmployeeCode,
			"employee_name":        empName,
			"department":           deptName,
			"avatar_url":           avatarURL,
			"check_time":           latestLog.CheckTime.Format("2006-01-02 15:04:05"),
			"check_type":           string(latestLog.CheckType),
			"verify_mode":          string(latestLog.VerifyMode),
		})

		broadcast.Global.Broadcast("attendance_synced", map[string]any{
			"device_id":            deviceID,
			"inserted":             inserted,
			"device_name":          d.Name,
			"latest_employee_code": latestLog.EmployeeCode,
			"latest_check_time":    latestLog.CheckTime.Format(time.RFC3339),
			"latest_check_type":    string(latestLog.CheckType),
			"latest_verify_mode":   string(latestLog.VerifyMode),
		})

		if s.notifier != nil && len(newLogs) > 0 {
			go s.notifier.NotifyAttendanceLogs(context.Background(), newLogs)
		}
	}

	return inserted, nil
}

// findDeviceMapping also tries zero-padded numeric aliases. Some ZKTeco TCP
// SDKs expose PIN "08" as integer 8, while ADMS/COM preserve the leading zero.
// Resolving the alias here keeps both collection paths mapped to the same
// employee without changing stored employee codes or ADMS behavior.
func (s *SyncService) findDeviceMapping(ctx context.Context, deviceID, deviceUserID string) (*entity.EmployeeDeviceMapping, error) {
	if s.mappingRepo == nil {
		return nil, nil
	}
	mapping, err := s.mappingRepo.GetByDeviceUserID(ctx, deviceID, deviceUserID)
	if err != nil || mapping != nil {
		return mapping, err
	}
	trimmed := strings.TrimSpace(deviceUserID)
	if trimmed == "" {
		return nil, nil
	}
	number, parseErr := strconv.ParseUint(trimmed, 10, 64)
	if parseErr != nil {
		return nil, nil
	}
	normalized := strconv.FormatUint(number, 10)
	for width := len(normalized) + 1; width <= 10; width++ {
		candidate := fmt.Sprintf("%0*s", width, normalized)
		if candidate == deviceUserID {
			continue
		}
		mapping, err = s.mappingRepo.GetByDeviceUserID(ctx, deviceID, candidate)
		if err != nil || mapping != nil {
			return mapping, err
		}
	}
	return nil, nil
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RetrySync đọc lại 1 sync_history đã thất bại và chạy lại
func (s *SyncService) RetrySync(ctx context.Context, syncHistoryID string) (*entity.SyncHistory, error) {
	old, err := s.syncHistoryRepo.GetByID(ctx, syncHistoryID)
	if err != nil {
		return nil, err
	}
	// Retry lại trong khoảng thời gian tương tự lần đồng bộ trước (đơn giản hoá: 24h gần nhất)
	to := time.Now()
	from := to.Add(-24 * time.Hour)
	return s.SyncAttendance(ctx, old.DeviceID, from, to, entity.SyncTriggerManual)
}

func (s *SyncService) fail(ctx context.Context, hist *entity.SyncHistory, err error) (*entity.SyncHistory, error) {
	hist.Status = entity.SyncStatusFailed
	hist.ErrorMessage = err.Error()
	hist.FinishedAt = time.Now()
	_ = s.syncHistoryRepo.Update(ctx, hist)
	// SDK/PULL failure must not make an ADMS-connected terminal appear fully
	// offline. Keep the device online while it still has a recent heartbeat;
	// the failed SDK attempt remains visible in sync_history for diagnosis.
	if d, getErr := s.deviceRepo.GetByID(ctx, hist.DeviceID); getErr == nil && d != nil &&
		isADMSDevice(d) && isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute) {
		return hist, err
	}
	_ = s.deviceRepo.UpdateStatus(ctx, hist.DeviceID, "offline", time.Now())
	return hist, err
}
