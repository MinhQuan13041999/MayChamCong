package adapter

import (
	"fmt"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/adapter/hikvision"
	"attendance-system/internal/infrastructure/adapter/sunbeam"
	"attendance-system/internal/infrastructure/adapter/zkteco"
)

// Factory implement port.DeviceAdapterFactory.
// Khi cần thêm hãng máy mới: tạo package adapter mới, implement port.DeviceAdapter,
// và đăng ký thêm 1 case dưới đây — không cần sửa Service/Controller.
type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewAdapter(deviceType entity.DeviceType) (port.DeviceAdapter, error) {
	var inner port.DeviceAdapter
	switch deviceType {
	case entity.DeviceTypeZKTeco:
		inner = zkteco.New()
	case entity.DeviceTypeSunbeam:
		inner = sunbeam.New()
	case entity.DeviceTypeHikvision:
		inner = hikvision.New()
	default:
		return nil, fmt.Errorf("unsupported device type: %s", deviceType)
	}
	// Bọc circuit breaker cho mọi adapter — Service layer không bị ảnh hưởng
	return NewResilient(inner, string(deviceType)), nil
}
