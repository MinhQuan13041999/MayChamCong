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
		MaxRequests: 3, // số request thử lại ở half-open
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

func (r *ResilientAdapter) DeleteEmployee(ctx context.Context, employeeCode string) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.DeleteEmployee(ctx, employeeCode)
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
	// A missing finger slot is a normal result when pulling a device: the
	// service probes indexes 0..9 and most users have only one or two templates.
	// Do not feed that expected per-finger miss into the device circuit breaker;
	// otherwise five empty slots open the breaker and all later indexes are
	// reported only as "circuit breaker is open", hiding the real SDK result.
	return r.inner.GetFingerprint(ctx, employeeCode, fingerIndex)
}

func (r *ResilientAdapter) DeleteFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.DeleteFingerprint(ctx, employeeCode, fingerIndex)
	})
	return err
}

func (r *ResilientAdapter) GetEmployeeFingerprints(ctx context.Context, employeeCode string) ([]entity.EmployeeFingerprint, error) {
	if reader, ok := r.inner.(interface {
		GetEmployeeFingerprints(context.Context, string) ([]entity.EmployeeFingerprint, error)
	}); ok {
		return reader.GetEmployeeFingerprints(ctx, employeeCode)
	}
	fingerprints := make([]entity.EmployeeFingerprint, 0, 10)
	for fingerIndex := 0; fingerIndex <= 9; fingerIndex++ {
		fp, err := r.inner.GetFingerprint(ctx, employeeCode, fingerIndex)
		if err == nil && fp != nil && fp.TemplateData != "" {
			fingerprints = append(fingerprints, *fp)
		}
	}
	return fingerprints, nil
}

func (r *ResilientAdapter) GetAllEmployeeFingerprints(ctx context.Context, employeeCodes []string) (map[string][]entity.EmployeeFingerprint, error) {
	if reader, ok := r.inner.(interface {
		GetAllEmployeeFingerprints(context.Context, []string) (map[string][]entity.EmployeeFingerprint, error)
	}); ok {
		return reader.GetAllEmployeeFingerprints(ctx, employeeCodes)
	}
	result := make(map[string][]entity.EmployeeFingerprint, len(employeeCodes))
	for _, employeeCode := range employeeCodes {
		fps, err := r.GetEmployeeFingerprints(ctx, employeeCode)
		if err != nil {
			return nil, err
		}
		if len(fps) > 0 {
			result[employeeCode] = fps
		}
	}
	return result, nil
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

func (r *ResilientAdapter) ClearEmployees(ctx context.Context) error {
	_, err := r.breaker.Execute(func() (interface{}, error) {
		return nil, r.inner.ClearEmployees(ctx)
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
