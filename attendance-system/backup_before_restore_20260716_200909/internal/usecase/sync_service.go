package usecase

import (
	"context"
	"fmt"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

const maxRetry = 3

// SyncService điều phối luồng đồng bộ dữ liệu chấm công (raw log) từ thiết bị về DB.
// Bản thân service KHÔNG tính công — chỉ lưu raw log để hệ thống khác (payroll/HRM) sử dụng.
type SyncService struct {
	deviceRepo      port.DeviceRepository
	attendanceRepo  port.AttendanceLogRepository
	syncHistoryRepo port.SyncHistoryRepository
	factory         port.DeviceAdapterFactory
	cursorRepo      port.SyncCursorRepository
	mappingRepo     port.EmployeeDeviceMappingRepository
	processor       *AttendanceProcessorService
}

func (s *SyncService) SetProcessor(processor *AttendanceProcessorService) {
	s.processor = processor
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

	if isADMSDevice(d) {
		hist.RecordCount = 0
		hist.Status = entity.SyncStatusSuccess
		hist.FinishedAt = time.Now()
		_ = s.syncHistoryRepo.Update(ctx, hist)
		return hist, nil
	}

	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return s.fail(ctx, hist, err)
	}

	cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 10 * time.Second}

	var lastErr error
	connected := false
	for attempt := 1; attempt <= maxRetry; attempt++ {
		if err := adapter.Connect(ctx, cfg); err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second) // simple backoff
			continue
		}
		connected = true
		break
	}
	if !connected {
		return s.fail(ctx, hist, fmt.Errorf("connect failed after %d attempts: %w", maxRetry, lastErr))
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
			mapping, mapErr := s.mappingRepo.GetByDeviceUserID(ctx, deviceID, l.EmployeeCode)
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

	if inserted > 0 {
		broadcast.Global.Broadcast("attendance_synced", map[string]any{
			"device_id":   deviceID,
			"inserted":    inserted,
			"device_name": d.Name,
		})

		if s.processor != nil {
			dates := make(map[string]time.Time)
			for _, l := range valid {
				dVal := time.Date(l.CheckTime.Year(), l.CheckTime.Month(), l.CheckTime.Day(), 0, 0, 0, 0, time.UTC)
				dates[dVal.Format("2006-01-02")] = dVal
			}
			for _, dVal := range dates {
				_ = s.processor.ProcessDailyAttendance(ctx, dVal)
			}
		}
	}
	return hist, nil
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
	_ = s.deviceRepo.UpdateStatus(ctx, hist.DeviceID, "offline", time.Now())
	return hist, err
}
