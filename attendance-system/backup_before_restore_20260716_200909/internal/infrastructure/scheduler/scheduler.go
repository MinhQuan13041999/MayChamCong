package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/usecase"
)

// Scheduler chạy job đồng bộ chấm công tự động theo cấu hình cron cho từng thiết bị.
type Scheduler struct {
	cron        *cron.Cron
	syncService *usecase.SyncService
	deviceRepo  port.DeviceRepository
	log         *zap.Logger
}

func New(syncService *usecase.SyncService, deviceRepo port.DeviceRepository, log *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:        cron.New(),
		syncService: syncService,
		deviceRepo:  deviceRepo,
		log:         log,
	}
}

// StartAttendanceSync đăng ký job chạy theo lịch (ví dụ "*/15 * * * *" = mỗi 15 phút)
// đồng bộ log chấm công cho tất cả thiết bị đang có trong hệ thống.
func (s *Scheduler) StartAttendanceSync(spec string) error {
	_, err := s.cron.AddFunc(spec, func() {
		ctx := context.Background()
		devices, err := s.deviceRepo.List(ctx)
		if err != nil {
			s.log.Error("scheduler: failed to list devices", zap.Error(err))
			return
		}

		to := time.Now()
		from := to.Add(-30 * time.Minute) // overlap nhẹ để tránh sót log

		_ = to // Cursor-based sync selects its own replay window.
		_ = from
		for _, d := range devices {
			hist, err := s.syncService.SyncAttendanceFromCursor(ctx, d.ID, entity.SyncTriggerScheduled)
			if err != nil {
				s.log.Error("scheduler: sync failed",
					zap.String("device_id", d.ID), zap.Error(err))
				continue
			}
			s.log.Info("scheduler: sync completed",
				zap.String("device_id", d.ID),
				zap.String("status", string(hist.Status)),
				zap.Int("record_count", hist.RecordCount))
		}
	})
	return err
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// StartTenSecondsSync khởi chạy background worker đồng bộ mỗi 10 giây.
func (s *Scheduler) StartTenSecondsSync(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		s.log.Info("Background 10-second sync worker started")
		for {
			select {
			case <-ctx.Done():
				s.log.Info("Background 10-second sync worker stopped")
				return
			case <-ticker.C:
				devices, err := s.deviceRepo.List(ctx)
				if err != nil {
					s.log.Error("10s-scheduler: failed to list devices", zap.Error(err))
					continue
				}
				for _, d := range devices {
					if d.ADMSEnabled {
						continue // skip ADMS devices because they push automatically
					}
					hist, err := s.syncService.SyncAttendanceFromCursor(ctx, d.ID, entity.SyncTriggerScheduled)
					if err != nil {
						// Log debug level to avoid flooding logs when devices are offline
						s.log.Debug("10s-scheduler: sync failed", zap.String("device_id", d.ID), zap.Error(err))
						continue
					}
					if hist.RecordCount > 0 {
						s.log.Info("10s-scheduler: sync completed",
							zap.String("device_id", d.ID),
							zap.String("status", string(hist.Status)),
							zap.Int("record_count", hist.RecordCount))
					}
				}
			}
		}
	}()
}

