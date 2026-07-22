package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// ResilientAdapter bọc bất kỳ DeviceAdapter nào và thêm:
//   - Circuit Breaker: nếu thiết bị lỗi liên tục → ngắt mạch tự động, không spam request.
//   - Sau thời gian chờ (Timeout), circuit tự chuyển sang half-open để thử lại.
//
// Implement đầy đủ port.DeviceAdapter — Service layer không biết gì về lớp bọc này.
type ResilientAdapter struct {
	inner   port.DeviceAdapter
	breaker *gobreaker.CircuitBreaker
}

// NewResilient tạo ResilientAdapter với circuit breaker cấu hình mặc định.
func NewResilient(inner port.DeviceAdapter, name string) *ResilientAdapter {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,           // số request thử lại ở half-open
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second, // sau 30s open → thử half-open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Mở circuit khi có 5 lỗi liên tiếp
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			// Log có thể thêm vào đây sau
			_ = fmt.Sprintf("circuit breaker [%s]: %v -> %v", name, from, to)
		},
	})
	return &ResilientAdapter{inner: inner, breaker: cb}
}

func (r *ResilientAdapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.Connect(ctx, cfg)
	})
	return err
}

func (r *ResilientAdapter) Disconnect(ctx context.Context) error {
	return r.inner.Disconnect(ctx) // Disconnect luôn cho phép, không qua breaker
}

func (r *ResilientAdapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	result, err := r.breaker.Execute(func() (interface{}, error) {
		return r.inner.CheckStatus(ctx)
	})
	if err != nil {
		return port.DeviceStatus{}, err
	}
	return result.(port.DeviceStatus), nil
}

func (r *ResilientAdapter) SyncTime(ctx context.Context) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.SyncTime(ctx)
	})
	return err
}

func (r *ResilientAdapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	result, err := r.breaker.Execute(func() (interface{}, error) {
		return r.inner.GetEmployees(ctx)
	})
	if err != nil {
		return nil, err
	}
	return result.([]entity.Employee), nil
}

func (r *ResilientAdapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.PushEmployee(ctx, emp)
	})
	return err
}

func (r *ResilientAdapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.PushFingerprint(ctx, employeeCode, fp)
	})
	return err
}

func (r *ResilientAdapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	result, err := r.breaker.Execute(func() (interface{}, error) {
		return r.inner.GetFingerprint(ctx, employeeCode, fingerIndex)
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.(*entity.EmployeeFingerprint), nil
}

func (r *ResilientAdapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	result, err := r.breaker.Execute(func() (interface{}, error) {
		return r.inner.GetAttendanceLogs(ctx, from, to)
	})
	if err != nil {
		return nil, err
	}
	return result.([]entity.AttendanceLog), nil
}

func (r *ResilientAdapter) ClearAttendanceLogs(ctx context.Context) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.ClearAttendanceLogs(ctx)
	})
	return err
}

func (r *ResilientAdapter) Reboot(ctx context.Context) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.Reboot(ctx)
	})
	return err
}

func (r *ResilientAdapter) Reset(ctx context.Context) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.Reset(ctx)
	})
	return err
}

func (r *ResilientAdapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.EnrollFingerprint(ctx, employeeCode, fingerIndex)
	})
	return err
}
