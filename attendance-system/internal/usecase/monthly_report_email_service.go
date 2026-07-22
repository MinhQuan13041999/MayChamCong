package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/domain/entity"
)

// MonthlyReportEmailService creates and sends one report per employee.
// It reads Daily Attendance and never triggers attendance calculation.
type MonthlyReportEmailService struct {
	reportService *ReportService
	notifier      *NotificationService
	mu            sync.Mutex
	sentKeys      map[string]time.Time
}

type MonthlyReportSendResult struct {
	Total               int      `json:"total"`
	Queued              int      `json:"queued"`
	Sent                int      `json:"sent"`
	SkippedNoEmail      int      `json:"skipped_no_email"`
	SkippedInvalidEmail int      `json:"skipped_invalid_email"`
	SkippedAlreadySent  int      `json:"skipped_already_sent"`
	Failed              int      `json:"failed"`
	Errors              []string `json:"errors,omitempty"`
}

type MonthlyReportProgress struct {
	EmployeeID   string `json:"employee_id"`
	EmployeeCode string `json:"employee_code"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

func NewMonthlyReportEmailService(reportService *ReportService, notifier *NotificationService) *MonthlyReportEmailService {
	return &MonthlyReportEmailService{reportService: reportService, notifier: notifier, sentKeys: make(map[string]time.Time)}
}

func (s *MonthlyReportEmailService) SendMonthlyReports(ctx context.Context, year, month int, forceResend bool, progress func(MonthlyReportProgress)) (MonthlyReportSendResult, error) {
	if s == nil || s.reportService == nil || s.notifier == nil {
		return MonthlyReportSendResult{}, fmt.Errorf("monthly report email service is not configured")
	}
	if month < 1 || month > 12 {
		return MonthlyReportSendResult{}, fmt.Errorf("month must be between 1 and 12")
	}
	employees, matrix, err := s.reportService.GetMonthlyAttendanceMatrix(ctx, year, month)
	if err != nil {
		return MonthlyReportSendResult{}, err
	}
	result := MonthlyReportSendResult{Total: len(employees)}
	if len(employees) == 0 {
		return result, nil
	}
	jobs := make(chan entity.Employee)
	var wg sync.WaitGroup
	var resultMu sync.Mutex
	worker := func() {
		defer wg.Done()
		for emp := range jobs {
			key := fmt.Sprintf("%s:%04d-%02d", emp.ID, year, month)
			if !forceResend && s.wasSent(key) {
				resultMu.Lock()
				result.SkippedAlreadySent++
				resultMu.Unlock()
				if progress != nil {
					progress(MonthlyReportProgress{EmployeeID: emp.ID, EmployeeCode: emp.EmployeeCode, Status: "skipped_already_sent"})
				}
				continue
			}
			if strings.TrimSpace(emp.Email) == "" {
				resultMu.Lock()
				result.SkippedNoEmail++
				resultMu.Unlock()
				if progress != nil {
					progress(MonthlyReportProgress{EmployeeID: emp.ID, EmployeeCode: emp.EmployeeCode, Status: "skipped_no_email"})
				}
				continue
			}
			_, parseErr := mail.ParseAddress(strings.TrimSpace(emp.Email))
			if parseErr != nil || !strings.Contains(emp.Email, "@") {
				resultMu.Lock()
				result.SkippedInvalidEmail++
				resultMu.Unlock()
				if progress != nil {
					progress(MonthlyReportProgress{EmployeeID: emp.ID, EmployeeCode: emp.EmployeeCode, Status: "skipped_invalid_email"})
				}
				continue
			}

			csvData, summary, sendErr := BuildEmployeeMonthlyCSV(emp, matrix[emp.ID], year, month)
			if sendErr == nil {
				filename := fmt.Sprintf("BaoCaoChamCong_%04d-%02d_%s.csv", year, month, safeReportFilename(emp.EmployeeCode))
				subject := fmt.Sprintf("Báo cáo chấm công tháng %02d/%04d - %s", month, year, emp.FullName)
				message := monthlyReportEmailBody(emp, year, month, summary)
				sendErr = s.notifier.SendEmailWithAttachment(emp.Email, subject, message, filename, "text/csv; charset=utf-8", csvData)
			}
			resultMu.Lock()
			if sendErr != nil {
				zap.L().Error("failed to send monthly report email",
					zap.String("employee_code", emp.EmployeeCode),
					zap.String("email", emp.Email),
					zap.Error(sendErr))
				result.Failed++
				if len(result.Errors) < 20 {
					result.Errors = append(result.Errors, emp.EmployeeCode+": "+sendErr.Error())
				}
			} else {
				result.Sent++
				s.recordSent(key)
			}
			resultMu.Unlock()
			if progress != nil {
				status := "sent"
				if sendErr != nil {
					status = "failed"
				}
				progress(MonthlyReportProgress{EmployeeID: emp.ID, EmployeeCode: emp.EmployeeCode, Status: status, Error: errorText(sendErr)})
			}

			// Delay 1 second between emails to prevent hitting SMTP rate limits / connection limits
			time.Sleep(1 * time.Second)
		}
	}

	workerCount := 1
	if len(employees) < workerCount {
		workerCount = len(employees)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}
	for _, emp := range employees {
		select {
		case jobs <- emp:
			result.Queued++
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return result, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return result, nil
}

func (s *MonthlyReportEmailService) wasSent(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.sentKeys[key]
	return ok && time.Since(at) < 31*24*time.Hour
}

func (s *MonthlyReportEmailService) recordSent(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentKeys[key] = time.Now()
}

type monthlyReportSummary struct {
	WorkingHours    float64
	LateMinutes     int
	EarlyMinutes    int
	OvertimeMinutes int
	PresentDays     int
	LeaveDays       int
	AbsentDays      int
}

// BuildEmployeeMonthlyCSV receives only one employee's matrix, preventing cross-employee data leakage.
func BuildEmployeeMonthlyCSV(emp entity.Employee, matrix map[string]entity.DailyAttendance, year, month int) ([]byte, monthlyReportSummary, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"Mã nhân viên", "Họ tên", "Ngày", "Trạng thái", "Check-in", "Check-out", "Giờ làm", "Đi muộn (phút)", "Về sớm (phút)", "Tăng ca (phút)"}); err != nil {
		return nil, monthlyReportSummary{}, err
	}
	summary := monthlyReportSummary{}
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	days := first.AddDate(0, 1, 0).Add(-24 * time.Hour).Day()
	for day := 1; day <= days; day++ {
		date := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		row := []string{emp.EmployeeCode, emp.FullName, date, "absent", "", "", "0.00", "0", "0", "0"}
		if da, ok := matrix[date]; ok {
			row[3] = da.AttendanceStatus
			if da.FirstIn != nil {
				row[4] = da.FirstIn.In(time.Local).Format("15:04")
			}
			if da.LastOut != nil {
				row[5] = da.LastOut.In(time.Local).Format("15:04")
			}
			row[6] = fmt.Sprintf("%.2f", da.WorkingHours)
			row[7] = strconv.Itoa(da.LateMinutes)
			row[8] = strconv.Itoa(da.EarlyMinutes)
			row[9] = strconv.Itoa(da.OvertimeMinutes)
			summary.WorkingHours += da.WorkingHours
			summary.LateMinutes += da.LateMinutes
			summary.EarlyMinutes += da.EarlyMinutes
			summary.OvertimeMinutes += da.OvertimeMinutes
			switch da.AttendanceStatus {
			case "leave":
				summary.LeaveDays++
			case "absent":
				summary.AbsentDays++
			default:
				summary.PresentDays++
			}
		} else {
			summary.AbsentDays++
		}
		if err := w.Write(sanitizeCSVRow(row)); err != nil {
			return nil, monthlyReportSummary{}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, monthlyReportSummary{}, err
	}
	return buf.Bytes(), summary, nil
}

func sanitizeCSVRow(row []string) []string {
	for i, value := range row {
		if len(value) > 0 && strings.ContainsRune("=+-@", rune(value[0])) {
			row[i] = "'" + value
		}
	}
	return row
}

func safeReportFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "employee"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "employee"
	}
	return b.String()
}

func monthlyReportEmailBody(emp entity.Employee, year, month int, summary monthlyReportSummary) string {
	return fmt.Sprintf("Chào %s,\n\nĐính kèm là báo cáo chấm công tháng %02d/%04d của bạn.\n\nTổng giờ làm: %.2f\nNgày có công: %d\nNgày nghỉ phép: %d\nNgày vắng: %d\nĐi muộn: %d phút\nVề sớm: %d phút\nTăng ca: %d phút\n\nNếu có sai sót, vui lòng liên hệ bộ phận nhân sự.", emp.FullName, month, year, summary.WorkingHours, summary.PresentDays, summary.LeaveDays, summary.AbsentDays, summary.LateMinutes, summary.EarlyMinutes, summary.OvertimeMinutes)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
