package usecase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"attendance-system/internal/config"
	"attendance-system/internal/domain/entity"
)

type notificationEmployeeRepoStub struct{ employee entity.Employee }

func (s *notificationEmployeeRepoStub) Create(context.Context, *entity.Employee) error { return nil }
func (s *notificationEmployeeRepoStub) Update(context.Context, *entity.Employee) error { return nil }
func (s *notificationEmployeeRepoStub) Delete(context.Context, string) error           { return nil }
func (s *notificationEmployeeRepoStub) DeleteAll(context.Context) (int64, error)       { return 0, nil }
func (s *notificationEmployeeRepoStub) GetByID(context.Context, string) (*entity.Employee, error) {
	e := s.employee
	return &e, nil
}
func (s *notificationEmployeeRepoStub) GetByCode(_ context.Context, code string) (*entity.Employee, error) {
	if code != s.employee.EmployeeCode {
		return nil, nil
	}
	e := s.employee
	return &e, nil
}
func (s *notificationEmployeeRepoStub) List(context.Context) ([]entity.Employee, error) {
	return []entity.Employee{s.employee}, nil
}
func (s *notificationEmployeeRepoStub) ListActive(context.Context) ([]entity.Employee, error) {
	return []entity.Employee{s.employee}, nil
}

type notificationShiftRepoStub struct{ shift entity.Shift }

func (s *notificationShiftRepoStub) Create(context.Context, *entity.Shift) error { return nil }
func (s *notificationShiftRepoStub) GetByID(context.Context, string) (*entity.Shift, error) {
	sh := s.shift
	return &sh, nil
}
func (s *notificationShiftRepoStub) List(context.Context) ([]entity.Shift, error) {
	return []entity.Shift{s.shift}, nil
}
func (s *notificationShiftRepoStub) Update(context.Context, *entity.Shift) error { return nil }
func (s *notificationShiftRepoStub) Delete(context.Context, string) error { return nil }

type notificationEmployeeShiftRepoStub struct{ assignment entity.EmployeeShift }

func (s *notificationEmployeeShiftRepoStub) Create(context.Context, *entity.EmployeeShift) error {
	return nil
}
func (s *notificationEmployeeShiftRepoStub) GetActiveShiftForEmployee(context.Context, string, time.Time) (*entity.EmployeeShift, error) {
	a := s.assignment
	return &a, nil
}
func (s *notificationEmployeeShiftRepoStub) Delete(context.Context, string) error { return nil }
func (s *notificationEmployeeShiftRepoStub) List(context.Context) ([]entity.EmployeeShift, error) {
	return []entity.EmployeeShift{s.assignment}, nil
}

type notificationAttendanceRepoStub struct{ logs []entity.AttendanceLog }

func (s *notificationAttendanceRepoStub) BulkInsert(context.Context, []entity.AttendanceLog) (int, error) {
	return 0, nil
}
func (s *notificationAttendanceRepoStub) Query(_ context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error) {
	var result []entity.AttendanceLog
	for _, log := range s.logs {
		if log.CheckTime.Before(from) || log.CheckTime.After(to) {
			continue
		}
		if employeeCode != "" && log.EmployeeCode != employeeCode {
			continue
		}
		if deviceID != "" && log.DeviceID != deviceID {
			continue
		}
		result = append(result, log)
	}
	return result, nil
}
func (s *notificationAttendanceRepoStub) UpdateValidity(context.Context, int64, bool, string) error {
	return nil
}
func (s *notificationAttendanceRepoStub) UpdateClassification(context.Context, int64, bool, string, string, *string) error {
	return nil
}

func TestAttendanceMessage(t *testing.T) {
	log := entity.AttendanceLog{CheckTime: time.Date(2026, 7, 20, 7, 58, 0, 0, time.Local), CheckType: entity.CheckTypeIn}
	got := attendanceMessage("Nguyễn Văn A", log)
	if got != "Chào Nguyễn Văn A, bạn đã check-in thành công lúc 07:58." {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestFilterNewAttendanceLogsRemovesPersistedAndBatchDuplicates(t *testing.T) {
	checkTime := time.Date(2026, 7, 20, 7, 58, 0, 0, time.UTC)
	existing := entity.AttendanceLog{DeviceID: "device-1", EmployeeCode: "NV01", CheckTime: checkTime}
	newLog := entity.AttendanceLog{DeviceID: "device-1", EmployeeCode: "NV02", CheckTime: checkTime.Add(time.Minute)}
	repo := &notificationAttendanceRepoStub{logs: []entity.AttendanceLog{existing}}
	got := filterNewAttendanceLogs(context.Background(), repo, []entity.AttendanceLog{existing, newLog, newLog})
	if len(got) != 1 || got[0].EmployeeCode != "NV02" {
		t.Fatalf("unexpected new logs: %+v", got)
	}
}

func TestNotifyAttendanceLogsSendsInstantZaloMessageOnce(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("access_token") != "token" {
			t.Errorf("missing Zalo access token")
		}
		var payload struct {
			Message map[string]string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if payload.Message["text"] != "Chào Nguyễn Văn A, bạn đã check-in thành công lúc 07:58." {
			t.Errorf("unexpected message: %q", payload.Message["text"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error": 0})
	}))
	defer server.Close()

	fixedNow := time.Date(2026, 7, 20, 7, 59, 0, 0, time.Local)
	emp := entity.Employee{ID: "employee-1", EmployeeCode: "NV01", FullName: "Nguyễn Văn A", ZaloUserID: "zalo-123"}
	service := NewNotificationService(&notificationEmployeeRepoStub{employee: emp}, nil, nil, nil,
		config.NotificationConfig{Enabled: true, ZaloEnabled: true, ZaloAPIURL: server.URL, ZaloAccessToken: "token", InstantMaxAgeMinutes: 10})
	service.now = func() time.Time { return fixedNow }
	log := entity.AttendanceLog{DeviceID: "device-1", EmployeeCode: emp.EmployeeCode, CheckTime: fixedNow.Add(-time.Minute), CheckType: entity.CheckTypeIn}

	service.NotifyAttendanceLogs(context.Background(), []entity.AttendanceLog{log, log})
	if got := calls.Load(); got != 1 {
		t.Fatalf("Zalo calls = %d, want 1", got)
	}
}

func TestCheckMissingCheckoutsSendsOneZaloReminder(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload struct {
			Recipient map[string]string `json:"recipient"`
			Message   map[string]string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if payload.Recipient["user_id"] != "zalo-123" {
			t.Errorf("unexpected recipient: %+v", payload.Recipient)
		}
		if !strings.Contains(payload.Message["text"], "chưa ghi nhận check-out") {
			t.Errorf("unexpected message: %q", payload.Message["text"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fixedNow := time.Date(2026, 7, 20, 17, 31, 0, 0, time.UTC)
	emp := entity.Employee{ID: "employee-1", EmployeeCode: "NV01", FullName: "Nguyễn Văn A", Status: "active", ZaloUserID: "zalo-123"}
	shiftID := "shift-1"
	service := NewNotificationService(
		&notificationEmployeeRepoStub{employee: emp},
		&notificationShiftRepoStub{shift: entity.Shift{ID: "shift-1", Name: "Hành chính", StartTime: "08:00", EndTime: "17:00", Timezone: "UTC"}},
		&notificationEmployeeShiftRepoStub{assignment: entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shiftID}},
		&notificationAttendanceRepoStub{logs: []entity.AttendanceLog{{EmployeeCode: emp.EmployeeCode, CheckTime: fixedNow.Add(-9 * time.Hour), CheckType: entity.CheckTypeIn}}},
		config.NotificationConfig{Enabled: true, ZaloEnabled: true, ZaloAPIURL: server.URL, ZaloAccessToken: "token", CheckoutGraceMinutes: 30},
	)
	service.now = func() time.Time { return fixedNow }

	if err := service.CheckMissingCheckouts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.CheckMissingCheckouts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("Zalo calls = %d, want 1", got)
	}
}
