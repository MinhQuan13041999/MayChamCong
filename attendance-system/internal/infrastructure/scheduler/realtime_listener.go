package scheduler

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// StartRealtimeEventListener lắng nghe và kiểm tra log chấm công mới từ các
// máy chấm công ZKTeco qua giao thức SDK truyền thống. Cập nhật dữ liệu và hiển thị Web tức thì (1-2s).
func (s *Scheduler) StartRealtimeEventListener(ctx context.Context) {
	go func() {
		s.log.Info("Realtime SDK Attendance Listener started")
		var activeListeners sync.Map // map[string]context.CancelFunc

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.log.Info("Realtime SDK Attendance Listener stopped")
				activeListeners.Range(func(key, value any) bool {
					if cancel, ok := value.(context.CancelFunc); ok {
						cancel()
					}
					return true
				})
				return
			case <-ticker.C:
				devices, err := s.deviceRepo.List(ctx)
				if err != nil {
					s.log.Error("realtime-listener: failed to list devices", zap.Error(err))
					continue
				}

				for _, d := range devices {
					devType := strings.ToLower(string(d.DeviceType))
					if devType != "zkteco" && devType != "standalone" {
						continue
					}
					if isADMSDevice(&d) {
						continue // ADMS thiết bị tự PUSH qua HTTP
					}

					if _, loaded := activeListeners.Load(d.ID); loaded {
						continue
					}

					devCtx, devCancel := context.WithCancel(ctx)
					activeListeners.Store(d.ID, devCancel)

					device := d
					go func() {
						defer activeListeners.Delete(device.ID)
						s.listenDeviceEvents(devCtx, device)
					}()
				}
			}
		}
	}()
}

// listenDeviceEvents thực hiện kiểm tra log mới cho thiết bị SDK Standalone qua phiên kết nối duy trì (Persistent Session).
// Sử dụng ProcessLiveAttendanceLogs thay vì SyncAttendanceFromCursor để KHÔNG tạo các bản ghi lịch sử đồng bộ (sync_history) rác.
func (s *Scheduler) listenDeviceEvents(ctx context.Context, dev entity.Device) {
	s.log.Info("Starting real-time listener worker for device", zap.String("device_id", dev.ID), zap.String("ip", dev.IPAddress))

	if s.syncService == nil || s.syncService.GetAdapterFactory() == nil {
		return
	}

	factory := s.syncService.GetAdapterFactory()
	adapter, err := factory.NewAdapter(dev.DeviceType)
	if err != nil {
		s.log.Error("realtime-listener: failed to create adapter", zap.String("device_id", dev.ID), zap.Error(err))
		return
	}

	cfg := port.DeviceConfig{
		IPAddress: dev.IPAddress,
		Port:      dev.Port,
		Username:  dev.Username,
		Password:  dev.Password,
		Timeout:   5 * time.Second,
	}
	if cfg.Port == 0 {
		cfg.Port = 4370
	}

	for {
		if ctx.Err() != nil {
			return
		}

		// 1. Mở kết nối duy trì (Persistent Connection)
		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := adapter.Connect(connectCtx, cfg)
		cancel()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		s.log.Info("Real-time persistent session established", zap.String("device_id", dev.ID), zap.String("ip", dev.IPAddress))
		lastCheck := time.Now().Add(-15 * time.Minute)
		ticker := time.NewTicker(500 * time.Millisecond)

		sessionActive := true
		for sessionActive {
			select {
			case <-ctx.Done():
				discCtx, discCancel := context.WithTimeout(context.Background(), 2*time.Second)
				adapter.Disconnect(discCtx)
				discCancel()
				ticker.Stop()
				return
			case <-ticker.C:
				readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
				to := time.Now()
				logs, err := adapter.GetAttendanceLogs(readCtx, lastCheck, to)
				readCancel()

				if err != nil {
					s.log.Debug("realtime-listener session lost, reconnecting...", zap.String("device_id", dev.ID), zap.Error(err))
					discCtx, discCancel := context.WithTimeout(context.Background(), 2*time.Second)
					adapter.Disconnect(discCtx)
					discCancel()
					sessionActive = false
					break
				}

				if len(logs) > 0 {
					latestCheckTime := logs[len(logs)-1].CheckTime
					if latestCheckTime.After(lastCheck) {
						lastCheck = latestCheckTime.Add(1 * time.Second)
					}

					inserted, err := s.syncService.ProcessLiveAttendanceLogs(ctx, dev.ID, logs)
					if err != nil {
						s.log.Error("realtime-listener: ProcessLiveAttendanceLogs failed", zap.String("device_id", dev.ID), zap.Error(err))
					} else if inserted > 0 {
						s.log.Info("⚡ REALTIME SDK SCAN DETECTED & BROADCASTED!",
							zap.String("device_id", dev.ID),
							zap.Int("inserted", inserted),
						)
					}
				}
			}
		}
		ticker.Stop()

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func isADMSDevice(d *entity.Device) bool {
	if d == nil {
		return false
	}
	return d.ADMSEnabled
}
