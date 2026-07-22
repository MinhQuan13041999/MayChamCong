package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"attendance-system/internal/usecase"
)

type AttendanceHandler struct {
	processor *usecase.AttendanceProcessorService
}

func NewAttendanceHandler(processor *usecase.AttendanceProcessorService) *AttendanceHandler {
	return &AttendanceHandler{processor: processor}
}

func (h *AttendanceHandler) Routes(r chi.Router) {
	// Các route nhân viên thường có thể thực hiện (viewer / employee)
	r.Post("/leave-requests", h.SubmitLeaveRequest)
	r.Get("/leave-requests", h.ListLeaveRequests)

	r.Post("/overtime-requests", h.SubmitOvertimeRequest)
	r.Get("/overtime-requests", h.ListOvertimeRequests)

	r.Post("/attendance-corrections", h.SubmitCorrectionRequest)
	r.Get("/attendance-corrections", h.ListCorrectionRequests)

	r.Get("/daily-attendance/report", h.GetDailyAttendanceReport)

	// Các route quản lý chỉ cho phép admin hoặc hr
	r.Group(func(adminOrHR chi.Router) {
		adminOrHR.Use(RequireRole("admin", "hr"))

		// Ca làm việc (Shift)
		adminOrHR.Post("/shifts", h.CreateShift)
		adminOrHR.Get("/shifts", h.ListShifts)
		adminOrHR.Delete("/shifts/{id}", h.DeleteShift)
		adminOrHR.Post("/employees/{id}/shifts", h.AssignShift)

		// Phê duyệt đơn từ
		adminOrHR.Post("/leave-requests/{id}/approve", h.ApproveLeaveRequest)
		adminOrHR.Post("/leave-requests/{id}/reject", h.RejectLeaveRequest)
		adminOrHR.Post("/overtime-requests/{id}/approve", h.ApproveOvertimeRequest)
		adminOrHR.Post("/overtime-requests/{id}/reject", h.RejectOvertimeRequest)
		adminOrHR.Post("/attendance-corrections/{id}/approve", h.ApproveCorrectionRequest)
		adminOrHR.Post("/attendance-corrections/{id}/reject", h.RejectCorrectionRequest)

		// Tính công ngày bằng tay
		adminOrHR.Post("/daily-attendance/process", h.ProcessDailyAttendance)

		// Nhật ký hệ thống (Audit logs)
		adminOrHR.Get("/audit-logs", h.ListAuditLogs)
	})
}

func getUserID(r *http.Request) *string {
	if val := r.Context().Value(ctxKeyUserID); val != nil {
		if s, ok := val.(string); ok {
			return &s
		}
	}
	return nil
}

// === SHIFT HANDLERS ===

type createShiftRequest struct {
	Name              string `json:"name"`
	StartTime         string `json:"start_time"`
	EndTime           string `json:"end_time"`
	BreakMinutes      int    `json:"break_minutes"`
	LateGraceMinutes  int    `json:"late_grace_minutes"`
	EarlyGraceMinutes int    `json:"early_grace_minutes"`
	MaxWorkingMinutes int    `json:"max_working_minutes"`
	Timezone          string `json:"timezone"`
}

func (h *AttendanceHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	var req createShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if req.Name == "" || req.StartTime == "" || req.EndTime == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name, start_time and end_time are required"))
		return
	}

	s, err := h.processor.CreateShift(r.Context(), req.Name, req.StartTime, req.EndTime, req.BreakMinutes, req.LateGraceMinutes, req.EarlyGraceMinutes, req.MaxWorkingMinutes, req.Timezone)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Ghi audit log
	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "CREATE_SHIFT", "shift", s.ID,
		fmt.Sprintf("Created shift '%s' (%s - %s)", s.Name, s.StartTime, s.EndTime), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, s)
}

func (h *AttendanceHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	list, err := h.processor.ListShifts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AttendanceHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.processor.DeleteShift(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "DELETE_SHIFT", "shift", id,
		fmt.Sprintf("Deleted shift ID %s", id), r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Shift deleted successfully"})
}

// === EMPLOYEE SHIFT HANDLERS ===

type assignShiftRequest struct {
	ShiftID   string `json:"shift_id"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD (optional)
}

func (h *AttendanceHandler) AssignShift(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "id")
	var req assignShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid start_date format, use YYYY-MM-DD"))
		return
	}

	var endDatePtr *time.Time
	if req.EndDate != "" {
		endDate, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid end_date format, use YYYY-MM-DD"))
			return
		}
		endDatePtr = &endDate
	}

	es, err := h.processor.AssignShift(r.Context(), employeeID, req.ShiftID, startDate, endDatePtr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "ASSIGN_SHIFT", "employee_shift", es.ID,
		fmt.Sprintf("Assigned shift %s to employee %s starting %s", req.ShiftID, employeeID, req.StartDate), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, es)
}

// === LEAVE REQUEST HANDLERS ===

type submitLeaveRequest struct {
	EmployeeID string `json:"employee_id"`
	LeaveType  string `json:"leave_type"`
	StartDate  string `json:"start_date"` // YYYY-MM-DD
	EndDate    string `json:"end_date"`   // YYYY-MM-DD
	Reason     string `json:"reason"`
}

func (h *AttendanceHandler) SubmitLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var req submitLeaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid start_date format, use YYYY-MM-DD"))
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid end_date format, use YYYY-MM-DD"))
		return
	}

	lr, err := h.processor.SubmitLeaveRequest(r.Context(), req.EmployeeID, req.LeaveType, startDate, endDate, req.Reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "SUBMIT_LEAVE", "leave_request", lr.ID,
		fmt.Sprintf("Employee %s submitted leave request (%s to %s)", req.EmployeeID, req.StartDate, req.EndDate), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, lr)
}

func (h *AttendanceHandler) ListLeaveRequests(w http.ResponseWriter, r *http.Request) {
	employeeID := r.URL.Query().Get("employee_id")
	status := r.URL.Query().Get("status")

	list, err := h.processor.ListLeaveRequests(r.Context(), employeeID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AttendanceHandler) ApproveLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.ApproveLeaveRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "APPROVE_LEAVE", "leave_request", id,
		"Approved leave request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Leave request approved"})
}

func (h *AttendanceHandler) RejectLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.RejectLeaveRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "REJECT_LEAVE", "leave_request", id,
		"Rejected leave request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Leave request rejected"})
}

// === OVERTIME REQUEST HANDLERS ===

type submitOTRequest struct {
	EmployeeID string `json:"employee_id"`
	Date       string `json:"date"`       // YYYY-MM-DD
	StartTime  string `json:"start_time"` // HH:MM
	EndTime    string `json:"end_time"`   // HH:MM
}

func (h *AttendanceHandler) SubmitOvertimeRequest(w http.ResponseWriter, r *http.Request) {
	var req submitOTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid date format, use YYYY-MM-DD"))
		return
	}

	ot, err := h.processor.SubmitOvertimeRequest(r.Context(), req.EmployeeID, date, req.StartTime, req.EndTime)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "SUBMIT_OT", "overtime_request", ot.ID,
		fmt.Sprintf("Employee %s submitted overtime request for %s (%s-%s)", req.EmployeeID, req.Date, req.StartTime, req.EndTime), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, ot)
}

func (h *AttendanceHandler) ListOvertimeRequests(w http.ResponseWriter, r *http.Request) {
	employeeID := r.URL.Query().Get("employee_id")
	status := r.URL.Query().Get("status")

	list, err := h.processor.ListOvertimeRequests(r.Context(), employeeID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AttendanceHandler) ApproveOvertimeRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.ApproveOvertimeRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "APPROVE_OT", "overtime_request", id,
		"Approved overtime request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Overtime request approved"})
}

func (h *AttendanceHandler) RejectOvertimeRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.RejectOvertimeRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "REJECT_OT", "overtime_request", id,
		"Rejected overtime request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Overtime request rejected"})
}

// === DAILY ATTENDANCE PROCESS & REPORT ===

type processAttendanceRequest struct {
	Date string `json:"date"` // YYYY-MM-DD
}

func (h *AttendanceHandler) ProcessDailyAttendance(w http.ResponseWriter, r *http.Request) {
	var req processAttendanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid date format, use YYYY-MM-DD"))
		return
	}

	if err := h.processor.ProcessDailyAttendance(r.Context(), date); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "PROCESS_ATTENDANCE", "daily_attendance", "",
		fmt.Sprintf("Processed daily attendance for date %s", req.Date), r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Daily attendance processing completed"})
}

func (h *AttendanceHandler) GetDailyAttendanceReport(w http.ResponseWriter, r *http.Request) {
	employeeID := r.URL.Query().Get("employee_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("from and to query parameters (YYYY-MM-DD) are required"))
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid from date, use YYYY-MM-DD"))
		return
	}

	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid to date, use YYYY-MM-DD"))
		return
	}

	// Chuyển 'to' thành cuối ngày đó
	to = to.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	report, err := h.processor.GetDailyAttendanceReport(r.Context(), employeeID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// === AUDIT LOG HANDLERS ===

func (h *AttendanceHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid limit parameter"))
			return
		}
	}
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid offset parameter"))
			return
		}
	}

	list, err := h.processor.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// === ATTENDANCE CORRECTION REQUEST HANDLERS ===

type submitCorrectionRequest struct {
	EmployeeID    string `json:"employee_id"`
	Date          string `json:"date"`            // YYYY-MM-DD
	CorrectedTime string `json:"corrected_time"`   // YYYY-MM-DD HH:MM:SS or HH:MM
	CheckType     string `json:"check_type"`      // in / out
	Reason        string `json:"reason"`
}

func (h *AttendanceHandler) SubmitCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	var req submitCorrectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid date format, use YYYY-MM-DD"))
		return
	}

	var correctedTime time.Time
	correctedTime, err = time.Parse(time.RFC3339, req.CorrectedTime)
	if err != nil {
		correctedTime, err = time.Parse("2006-01-02 15:04:05", req.CorrectedTime)
		if err != nil {
			if len(req.CorrectedTime) == 5 {
				var hour, min int
				if _, errScan := fmt.Sscanf(req.CorrectedTime, "%d:%d", &hour, &min); errScan == nil {
					correctedTime = time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, time.Local)
					err = nil
				}
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("invalid corrected_time format, use YYYY-MM-DDTHH:MM:SSZ or HH:MM"))
				return
			}
		}
	}

	ac, err := h.processor.SubmitCorrectionRequest(r.Context(), req.EmployeeID, date, correctedTime, req.CheckType, req.Reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userID := getUserID(r)
	_ = h.processor.CreateAuditLog(r.Context(), userID, "SUBMIT_CORRECTION", "attendance_correction", ac.ID,
		fmt.Sprintf("Employee %s submitted correction request for %s (%s)", req.EmployeeID, req.Date, req.CorrectedTime), r.RemoteAddr)

	writeJSON(w, http.StatusCreated, ac)
}

func (h *AttendanceHandler) ListCorrectionRequests(w http.ResponseWriter, r *http.Request) {
	employeeID := r.URL.Query().Get("employee_id")
	status := r.URL.Query().Get("status")

	list, err := h.processor.ListCorrectionRequests(r.Context(), employeeID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AttendanceHandler) ApproveCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.ApproveCorrectionRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "APPROVE_CORRECTION", "attendance_correction", id,
		"Approved attendance correction request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Attendance correction request approved"})
}

func (h *AttendanceHandler) RejectCorrectionRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := getUserID(r)
	approverID := ""
	if userID != nil {
		approverID = *userID
	}

	if err := h.processor.RejectCorrectionRequest(r.Context(), id, approverID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.processor.CreateAuditLog(r.Context(), userID, "REJECT_CORRECTION", "attendance_correction", id,
		"Rejected attendance correction request", r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Attendance correction request rejected"})
}
