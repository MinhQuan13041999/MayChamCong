package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/usecase"
)

type stubAttendanceRepo struct {
	logs []entity.AttendanceLog
}

func (s stubAttendanceRepo) Query(ctx context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error) {
	return s.logs, nil
}

type stubEmployeeRepo struct {
	employees []entity.Employee
}

func (s stubEmployeeRepo) List(ctx context.Context) ([]entity.Employee, error) {
	return s.employees, nil
}
func (s stubEmployeeRepo) GetByCode(ctx context.Context, code string) (*entity.Employee, error) {
	for _, emp := range s.employees {
		if emp.EmployeeCode == code {
			return &emp, nil
		}
	}
	return nil, nil
}

func (s stubEmployeeRepo) Create(ctx context.Context, e *entity.Employee) error { return nil }
func (s stubEmployeeRepo) Update(ctx context.Context, e *entity.Employee) error { return nil }
func (s stubEmployeeRepo) Delete(ctx context.Context, id string) error          { return nil }
func (s stubEmployeeRepo) GetByID(ctx context.Context, id string) (*entity.Employee, error) {
	return nil, nil
}
func (s stubEmployeeRepo) ListActive(ctx context.Context) ([]entity.Employee, error) { return nil, nil }

type stubDeviceRepo struct{}

func (s stubDeviceRepo) List(ctx context.Context) ([]entity.Device, error) {
	return []entity.Device{{Status: "online"}, {Status: "offline"}}, nil
}
func (s stubDeviceRepo) Create(ctx context.Context, d *entity.Device) error { return nil }
func (s stubDeviceRepo) Update(ctx context.Context, d *entity.Device) error { return nil }
func (s stubDeviceRepo) Delete(ctx context.Context, id string) error        { return nil }
func (s stubDeviceRepo) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	return nil, nil
}
func (s stubDeviceRepo) UpdateStatus(ctx context.Context, id string, status string, checkedAt time.Time) error {
	return nil
}

type stubSyncRepo struct{}

func (s stubSyncRepo) List(ctx context.Context, deviceID, status string) ([]entity.SyncHistory, error) {
	return []entity.SyncHistory{{Status: entity.SyncStatusSuccess}, {Status: entity.SyncStatusFailed}}, nil
}
func (s stubSyncRepo) Create(ctx context.Context, h *entity.SyncHistory) (string, error) {
	return "1", nil
}
func (s stubSyncRepo) Update(ctx context.Context, h *entity.SyncHistory) error { return nil }
func (s stubSyncRepo) GetByID(ctx context.Context, id string) (*entity.SyncHistory, error) {
	return nil, nil
}

type stubDailyAttendanceRepo struct {
	items []entity.DailyAttendance
}

func (s stubDailyAttendanceRepo) Query(ctx context.Context, employeeID string, from, to time.Time) ([]entity.DailyAttendance, error) {
	return s.items, nil
}

func TestReportService_GetAttendanceSummary(t *testing.T) {
	service := usecase.NewReportService(stubAttendanceRepo{logs: []entity.AttendanceLog{
		{EmployeeCode: "EMP001", CheckTime: time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC), CheckType: entity.CheckTypeIn},
		{EmployeeCode: "EMP001", CheckTime: time.Date(2026, 7, 13, 17, 0, 0, 0, time.UTC), CheckType: entity.CheckTypeOut},
		{EmployeeCode: "EMP002", CheckTime: time.Date(2026, 7, 13, 8, 30, 0, 0, time.UTC), CheckType: entity.CheckTypeIn},
	}}, stubEmployeeRepo{employees: []entity.Employee{{EmployeeCode: "EMP001", FullName: "Nguyen Van A"}, {EmployeeCode: "EMP002", FullName: "Tran Thi B"}}}, stubDeviceRepo{}, stubSyncRepo{}, stubDailyAttendanceRepo{})

	summary, err := service.GetAttendanceSummary(context.Background(), time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 13, 23, 59, 59, 0, time.UTC), "")
	require.NoError(t, err)
	require.Len(t, summary, 2)
	statusByCode := make(map[string]string, len(summary))
	for _, item := range summary {
		statusByCode[item.EmployeeCode] = item.Status
	}
	require.Equal(t, "present", statusByCode["EMP001"])
	require.Equal(t, "partial", statusByCode["EMP002"])
}

func TestReportService_GetMonthlyAttendanceMatrix(t *testing.T) {
	employees := []entity.Employee{{ID: "employee-1", EmployeeCode: "EMP001", FullName: "Nguyen Van A"}}
	date := time.Date(2026, 7, 13, 0, 0, 0, 0, time.Local)
	service := usecase.NewReportService(
		stubAttendanceRepo{}, stubEmployeeRepo{employees: employees}, stubDeviceRepo{}, stubSyncRepo{},
		stubDailyAttendanceRepo{items: []entity.DailyAttendance{{EmployeeID: "employee-1", Date: date, AttendanceStatus: "present"}}},
	)

	gotEmployees, matrix, err := service.GetMonthlyAttendanceMatrix(context.Background(), 2026, 7)
	require.NoError(t, err)
	require.Equal(t, employees, gotEmployees)
	require.Equal(t, "present", matrix["employee-1"]["2026-07-13"].AttendanceStatus)
}
