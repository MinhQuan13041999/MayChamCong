package usecase

import (
	"context"
	"sort"
	"time"

	"attendance-system/internal/domain/entity"
)

type attendanceLogRepository interface {
	Query(ctx context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error)
}

type employeeRepository interface {
	List(ctx context.Context) ([]entity.Employee, error)
	GetByCode(ctx context.Context, code string) (*entity.Employee, error)
}

type deviceRepository interface {
	List(ctx context.Context) ([]entity.Device, error)
}

type syncHistoryRepository interface {
	List(ctx context.Context, deviceID, status string) ([]entity.SyncHistory, error)
}

type dailyAttendanceRepository interface {
	Query(ctx context.Context, employeeID string, from, to time.Time) ([]entity.DailyAttendance, error)
}

// ReportService cung cấp các thống kê và báo cáo cho UI dashboard.
type ReportService struct {
	attendanceRepo attendanceLogRepository
	employeeRepo   employeeRepository
	deviceRepo     deviceRepository
	syncRepo       syncHistoryRepository
	dailyAttRepo   dailyAttendanceRepository
}

// DashboardStats thống kê tổng quan cho dashboard.
type DashboardStats struct {
	DevicesTotal      int `json:"devices_total"`
	EmployeesTotal    int `json:"employees_total"`
	AttendanceEntries int `json:"attendance_entries"`
	SyncHistoryTotal  int `json:"sync_history_total"`
	OnlineDevices     int `json:"online_devices"`
}

// AttendanceSummaryItem tóm tắt chấm công theo nhân viên và ngày.
type AttendanceSummaryItem struct {
	EmployeeCode  string `json:"employee_code"`
	FullName      string `json:"full_name"`
	Date          string `json:"date"`
	CheckInCount  int    `json:"check_in_count"`
	CheckOutCount int    `json:"check_out_count"`
	Status        string `json:"status"`
}

func NewReportService(attendanceRepo attendanceLogRepository, employeeRepo employeeRepository, deviceRepo deviceRepository, syncRepo syncHistoryRepository, dailyAttRepo dailyAttendanceRepository) *ReportService {
	return &ReportService{
		attendanceRepo: attendanceRepo,
		employeeRepo:   employeeRepo,
		deviceRepo:     deviceRepo,
		syncRepo:       syncRepo,
		dailyAttRepo:   dailyAttRepo,
	}
}

func (s *ReportService) GetDashboardStats(ctx context.Context) (DashboardStats, error) {
	devices, err := s.deviceRepo.List(ctx)
	if err != nil {
		return DashboardStats{}, err
	}
	employees, err := s.employeeRepo.List(ctx)
	if err != nil {
		return DashboardStats{}, err
	}
	logs, err := s.attendanceRepo.Query(ctx, time.Now().AddDate(0, 0, -30), time.Now(), "", "")
	if err != nil {
		return DashboardStats{}, err
	}
	syncHistory, err := s.syncRepo.List(ctx, "", "")
	if err != nil {
		return DashboardStats{}, err
	}

	online := 0
	for _, device := range devices {
		if device.Status == "online" {
			online++
		}
	}

	return DashboardStats{
		DevicesTotal:      len(devices),
		EmployeesTotal:    len(employees),
		AttendanceEntries: len(logs),
		SyncHistoryTotal:  len(syncHistory),
		OnlineDevices:     online,
	}, nil
}

func (s *ReportService) GetAttendanceSummary(ctx context.Context, from, to time.Time, employeeCode string) ([]AttendanceSummaryItem, error) {
	logs, err := s.attendanceRepo.Query(ctx, from, to, employeeCode, "")
	if err != nil {
		return nil, err
	}

	employees, err := s.employeeRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	nameByCode := make(map[string]string, len(employees))
	for _, emp := range employees {
		nameByCode[emp.EmployeeCode] = emp.FullName
	}

	type key struct {
		employeeCode string
		date         string
	}

	// Group logs by employee and date
	logsGroup := make(map[key][]entity.AttendanceLog)
	for _, log := range logs {
		date := log.CheckTime.Format("2006-01-02")
		k := key{employeeCode: log.EmployeeCode, date: date}
		logsGroup[k] = append(logsGroup[k], log)
	}

	aggregated := make(map[key]*AttendanceSummaryItem)
	for k, groupLogs := range logsGroup {
		n := len(groupLogs)
		// Reverse because Query returns DESC order (we want chronological order)
		for i := 0; i < n/2; i++ {
			groupLogs[i], groupLogs[n-1-i] = groupLogs[n-1-i], groupLogs[i]
		}

		item := &AttendanceSummaryItem{
			EmployeeCode: k.employeeCode,
			FullName:     nameByCode[k.employeeCode],
			Date:         k.date,
		}

		if n == 1 {
			// Single log: treat as check-in
			item.CheckInCount = 1
			item.CheckOutCount = 0
		} else if n >= 2 {
			// Multiple logs: earliest is check-in, latest is check-out
			item.CheckInCount = 1
			item.CheckOutCount = 1
		}

		aggregated[k] = item
	}

	items := make([]AttendanceSummaryItem, 0, len(aggregated))
	for _, item := range aggregated {
		item.Status = summarizeStatus(item.CheckInCount, item.CheckOutCount)
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].EmployeeCode < items[j].EmployeeCode
		}
		return items[i].Date < items[j].Date
	})

	return items, nil
}

func summarizeStatus(checkIn, checkOut int) string {
	switch {
	case checkIn > 0 && checkOut > 0:
		return "present"
	case checkIn > 0 || checkOut > 0:
		return "partial"
	default:
		return "absent"
	}
}

func (s *ReportService) GetMonthlyAttendanceMatrix(ctx context.Context, year int, month int) ([]entity.Employee, map[string]map[string]entity.DailyAttendance, error) {
	employees, err := s.employeeRepo.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 1, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	daList, err := s.dailyAttRepo.Query(ctx, "", from, to)
	if err != nil {
		return nil, nil, err
	}

	matrix := make(map[string]map[string]entity.DailyAttendance)
	for _, da := range daList {
		dateStr := da.Date.Format("2006-01-02")
		if _, ok := matrix[da.EmployeeID]; !ok {
			matrix[da.EmployeeID] = make(map[string]entity.DailyAttendance)
		}
		matrix[da.EmployeeID][dateStr] = da
	}

	return employees, matrix, nil
}
