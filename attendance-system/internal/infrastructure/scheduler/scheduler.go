package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/usecase"
)

// Scheduler chạy job đồng bộ chấm công tự động theo cấu hình cron cho từng thiết bị.
type Scheduler struct {
	cron                *cron.Cron
	syncService         *usecase.SyncService
	notificationService *usecase.NotificationService
	deviceRepo          port.DeviceRepository
	log                 *zap.Logger
	pollInterval        time.Duration
	syncTimeout         time.Duration
}

// SDKPollConfig controls the fast background pull without changing the
// existing cron/manual synchronization paths.
type SDKPollConfig struct {
	PollInterval time.Duration
	SyncTimeout  time.Duration
}

func (s *Scheduler) SetNotificationService(service *usecase.NotificationService) {
	s.notificationService = service
}

func New(syncService *usecase.SyncService, deviceRepo port.DeviceRepository, log *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:         cron.New(),
		syncService:  syncService,
		deviceRepo:   deviceRepo,
		log:          log,
		pollInterval: 10 * time.Minute,
		syncTimeout:  30 * time.Second,
	}
}

func (s *Scheduler) SetSDKPollConfig(cfg SDKPollConfig) {
	if cfg.PollInterval > 0 {
		s.pollInterval = cfg.PollInterval
	}
	if cfg.SyncTimeout > 0 {
		s.syncTimeout = cfg.SyncTimeout
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

// StartTenSecondsSync khởi chạy background worker đồng bộ SDK nhanh. Mỗi thiết
// bị có tối đa một lượt đồng bộ đang chạy; thiết bị offline không chặn máy khác.
func (s *Scheduler) StartTenSecondsSync(ctx context.Context) {
	go func() {
		interval := s.pollInterval
		if interval <= 0 {
			interval = 10 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		s.log.Info("Background SDK attendance sync worker started", zap.Duration("interval", interval))
		var inFlight sync.Map
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
					if _, loaded := inFlight.LoadOrStore(d.ID, struct{}{}); loaded {
						continue
					}
					device := d
					go func() {
						defer inFlight.Delete(device.ID)
						syncCtx := ctx
						cancel := func() {}
						if s.syncTimeout > 0 {
							syncCtx, cancel = context.WithTimeout(ctx, s.syncTimeout)
						}
						defer cancel()
						// Keep the existing supplemental SDK pull for ADMS devices;
						// DB de-duplication makes both sources safe together.
						hist, err := s.syncService.SyncAttendanceFromCursor(syncCtx, device.ID, entity.SyncTriggerScheduled)
						if err != nil {
							s.log.Debug("sdk-scheduler: sync failed", zap.String("device_id", device.ID), zap.Error(err))
							return
						}
						if hist.RecordCount > 0 {
							s.log.Info("sdk-scheduler: sync completed",
								zap.String("device_id", device.ID),
								zap.String("status", string(hist.Status)),
								zap.Int("record_count", hist.RecordCount))
						}
					}()
				}
			}
		}
	}()
}

// StartCheckoutReminderMonitor kiểm tra thiếu check-out mỗi phút.
func (s *Scheduler) StartCheckoutReminderMonitor(ctx context.Context) {
	if s.notificationService == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		s.log.Info("Checkout reminder monitor started")
		for {
			select {
			case <-ctx.Done():
				s.log.Info("Checkout reminder monitor stopped")
				return
			case <-ticker.C:
				if err := s.notificationService.CheckMissingCheckouts(ctx); err != nil {
					s.log.Error("checkout reminder check failed", zap.Error(err))
				}
			}
		}
	}()
}
