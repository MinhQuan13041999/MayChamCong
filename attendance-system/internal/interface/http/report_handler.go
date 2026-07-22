package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/infrastructure/broadcast"
	"attendance-system/internal/usecase"
)

type ReportHandler struct {
	service      *usecase.ReportService
	emailService *usecase.MonthlyReportEmailService
	auditService *usecase.AttendanceProcessorService
}

func NewReportHandler(service *usecase.ReportService, emailService *usecase.MonthlyReportEmailService, auditService *usecase.AttendanceProcessorService) *ReportHandler {
	return &ReportHandler{service: service, emailService: emailService, auditService: auditService}
}

func (h *ReportHandler) Routes(r chi.Router) {
	r.Get("/dashboard/stats", h.DashboardStats)
	r.Get("/attendance-summary", h.AttendanceSummary)
	r.Get("/reports/attendance-excel", h.ExportAttendanceExcel)
	r.Group(func(adminOrHR chi.Router) {
		adminOrHR.Use(RequireRole("admin", "hr"))
		adminOrHR.Post("/reports/monthly-email", h.SendMonthlyReports)
	})
}

type monthlyReportEmailRequest struct {
	Month       string `json:"month"`
	ForceResend bool   `json:"force_resend"`
}

func (h *ReportHandler) SendMonthlyReports(w http.ResponseWriter, r *http.Request) {
	if h.emailService == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("monthly report email service is not configured"))
		return
	}
	var req monthlyReportEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	target, err := time.Parse("2006-01", req.Month)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid month format, use YYYY-MM"))
		return
	}
	result, err := h.emailService.SendMonthlyReports(r.Context(), target.Year(), int(target.Month()), req.ForceResend, func(progress usecase.MonthlyReportProgress) {
		broadcast.Global.Broadcast("monthly_report_progress", progress)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if h.auditService != nil {
		description := fmt.Sprintf("Sent monthly attendance reports for %s: sent=%d failed=%d skipped_no_email=%d skipped_invalid_email=%d", req.Month, result.Sent, result.Failed, result.SkippedNoEmail, result.SkippedInvalidEmail)
		_ = h.auditService.CreateAuditLog(r.Context(), getUserID(r), "SEND_MONTHLY_ATTENDANCE_EMAIL", "monthly_report", req.Month, description, r.RemoteAddr)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ReportHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetDashboardStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *ReportHandler) AttendanceSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := parseTimeOrDefault(q.Get("from"), time.Now().AddDate(0, 0, -7))
	to := parseTimeOrDefault(q.Get("to"), time.Now())
	items, err := h.service.GetAttendanceSummary(r.Context(), from, to, q.Get("employee_code"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *ReportHandler) ExportAttendanceExcel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	monthStr := r.URL.Query().Get("month")
	var targetTime time.Time
	var err error
	if monthStr == "" {
		targetTime = time.Now()
	} else {
		targetTime, err = time.Parse("2006-01", monthStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid month format, use YYYY-MM"))
			return
		}
	}

	year, month, _ := targetTime.Date()

	employees, matrix, err := h.service.GetMonthlyAttendanceMatrix(ctx, year, int(month))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Tính số ngày của tháng
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, -1)
	numDays := lastDay.Day()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=BaoCaoChamCong_%d_%02d.csv", year, month))
	w.WriteHeader(http.StatusOK)

	// Ghi UTF-8 BOM
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	// CSV Header
	header := []string{"Mã NV", "Họ và Tên"}
	for d := 1; d <= numDays; d++ {
		header = append(header, fmt.Sprintf("Ngày %d", d))
	}
	header = append(header, "Tổng công (ngày)", "Tổng đi muộn (phút)", "Tổng về sớm (phút)", "Tổng tăng ca (phút)")

	writeCSVRow := func(row []string) {
		for i, field := range row {
			escaped := field
			if strings.ContainsAny(field, ",\"\r\n") {
				escaped = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
			}
			_, _ = w.Write([]byte(escaped))
			if i < len(row)-1 {
				_, _ = w.Write([]byte(","))
			}
		}
		_, _ = w.Write([]byte("\r\n"))
	}

	writeCSVRow(header)

	for _, emp := range employees {
		row := []string{emp.EmployeeCode, emp.FullName}
		var totalHours float64
		var totalLate int
		var totalEarly int
		var totalOT int

		empMatrix := matrix[emp.ID]

		for d := 1; d <= numDays; d++ {
			dateStr := fmt.Sprintf("%d-%02d-%02d", year, int(month), d)
			cell := "-"
			if da, ok := empMatrix[dateStr]; ok {
				if da.LeaveID != nil && *da.LeaveID != "" {
					cell = "P"
					totalHours += da.WorkingHours
				} else if da.AttendanceStatus == "absent" {
					cell = "V"
				} else {
					inStr := "--:--"
					if da.FirstIn != nil {
						inStr = da.FirstIn.Format("15:04")
					}
					outStr := "--:--"
					if da.LastOut != nil {
						outStr = da.LastOut.Format("15:04")
					}
					statusChar := "X"
					if da.AttendanceStatus == "late" {
						statusChar = "M"
					} else if da.AttendanceStatus == "early" {
						statusChar = "S"
					}
					cell = fmt.Sprintf("%s (%s-%s)", statusChar, inStr, outStr)
					totalHours += da.WorkingHours
					totalLate += da.LateMinutes
					totalEarly += da.EarlyMinutes
					totalOT += da.OvertimeMinutes
				}
			}
			row = append(row, cell)
		}

		totalDays := totalHours / 8.0

		row = append(row,
			fmt.Sprintf("%.2f", totalDays),
			fmt.Sprintf("%d", totalLate),
			fmt.Sprintf("%d", totalEarly),
			fmt.Sprintf("%d", totalOT),
		)
		writeCSVRow(row)
	}
}
