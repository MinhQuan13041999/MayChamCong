package usecase

import (
	"context"
	"fmt"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// DeviceService xử lý business logic liên quan tới thiết bị chấm công.
// Service KHÔNG gọi trực tiếp SDK hãng máy — chỉ phụ thuộc vào port.DeviceAdapter.
type DeviceService struct {
	deviceRepo  port.DeviceRepository
	factory     port.DeviceAdapterFactory
	commandRepo port.DeviceCommandRepository
}

func NewDeviceService(deviceRepo port.DeviceRepository, factory port.DeviceAdapterFactory) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo, factory: factory}
}

func (s *DeviceService) SetCommandRepo(repo port.DeviceCommandRepository) {
	s.commandRepo = repo
}

func (s *DeviceService) CreateDevice(ctx context.Context, d *entity.Device) error {
	if d.Name == "" || d.IPAddress == "" {
		return fmt.Errorf("device name and ip_address are required")
	}
	d.Status = "offline"
	return s.deviceRepo.Create(ctx, d)
}

func (s *DeviceService) UpdateDevice(ctx context.Context, d *entity.Device) error {
	return s.deviceRepo.Update(ctx, d)
}

func (s *DeviceService) DeleteDevice(ctx context.Context, id string) error {
	return s.deviceRepo.Delete(ctx, id)
}

func (s *DeviceService) ListDevices(ctx context.Context) ([]entity.Device, error) {
	return s.deviceRepo.List(ctx)
}

func (s *DeviceService) GetDevice(ctx context.Context, id string) (*entity.Device, error) {
	return s.deviceRepo.GetByID(ctx, id)
}

// TestConnection kiểm tra kết nối và cập nhật trạng thái online/offline
func (s *DeviceService) TestConnection(ctx context.Context, id string) (port.DeviceStatus, error) {
	d, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return port.DeviceStatus{}, err
	}

	if isADMSDevice(d) {
		// Cho thiết bị ADMS, kiểm tra Heartbeat gần nhất (trong vòng 10 phút để tránh offline nhầm do độ trễ hoặc lệch múi giờ)
		online := isDeviceOnline(d.LastHeartbeatAt, 10*time.Minute)
		status := "offline"
		if online {
			status = "online"
		}
		_ = s.deviceRepo.UpdateStatus(ctx, id, status, time.Now())
		return port.DeviceStatus{
			Online:       online,
			FirmwareInfo: d.FirmwareVersion,
			UserCount:    0,
			LogCount:     0,
		}, nil
	}

	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return port.DeviceStatus{}, err
	}

	cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 5 * time.Second}
	if err := adapter.Connect(ctx, cfg); err != nil {
		_ = s.deviceRepo.UpdateStatus(ctx, id, "offline", time.Now())
		return port.DeviceStatus{}, err
	}
	defer adapter.Disconnect(ctx)

	status, err := adapter.CheckStatus(ctx)
	if err != nil {
		_ = s.deviceRepo.UpdateStatus(ctx, id, "offline", time.Now())
		return port.DeviceStatus{}, err
	}

	newStatus := "offline"
	if status.Online {
		newStatus = "online"
	}
	_ = s.deviceRepo.UpdateStatus(ctx, id, newStatus, time.Now())
	return status, nil
}

func (s *DeviceService) RebootDevice(ctx context.Context, id string) error {
	d, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if isADMSDevice(d) {
		if s.commandRepo == nil {
			return fmt.Errorf("ADMS command repository is not configured")
		}
		// Đưa lệnh RESTART vào hàng đợi lệnh cho thiết bị ADMS
		_, err := s.commandRepo.Enqueue(ctx, d.ID, "RESTART")
		return err
	}

	adapter, err := s.factory.NewAdapter(d.DeviceType)
	if err != nil {
		return err
	}
	cfg := port.DeviceConfig{IPAddress: d.IPAddress, Port: d.Port, Timeout: 5 * time.Second}
	if err := adapter.Connect(ctx, cfg); err != nil {
		return err
	}
	defer adapter.Disconnect(ctx)
	return adapter.Reboot(ctx)
}

// ListPendingCommands trả về danh sách lệnh pending cho thiết bị (dùng để debug)
func (s *DeviceService) ListPendingCommands(ctx context.Context, deviceID string) ([]map[string]any, error) {
	if s.commandRepo == nil {
		return nil, fmt.Errorf("command repository not configured")
	}
	cmds, err := s.commandRepo.GetPending(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, map[string]any{
			"id":         c.ID,
			"command_id": c.CommandID,
			"command":    c.Command,
			"status":     c.Status,
			"created_at": c.CreatedAt,
			"sent_at":    c.SentAt,
			"acked_at":   c.AckedAt,
		})
	}
	return out, nil
}

// CancelPendingCommands cancels pending queued commands for a device and returns how many were cancelled
func (s *DeviceService) CancelPendingCommands(ctx context.Context, deviceID string) (int, error) {
	if s.commandRepo == nil {
		return 0, fmt.Errorf("command repository not configured")
	}
	return s.commandRepo.CancelPendingByDevice(ctx, deviceID)
}
