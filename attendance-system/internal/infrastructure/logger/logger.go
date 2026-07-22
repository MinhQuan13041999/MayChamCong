package logger

import "go.uber.org/zap"

// New khởi tạo structured logger dùng zap.
// Log nên luôn gắn kèm các field như device_id, request_id, duration ở nơi gọi.
func New(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
