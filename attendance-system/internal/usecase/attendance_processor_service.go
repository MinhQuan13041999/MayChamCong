package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
	"attendance-system/internal/infrastructure/broadcast"
)

type AttendanceProcessorService struct {
	employeeRepo        port.EmployeeRepository
	shiftRepo           port.ShiftRepository
	empShiftRepo        port.EmployeeShiftRepository
	leaveRepo           port.LeaveRequestRepository
	otRepo              port.OvertimeRequestRepository
	dailyAttendanceRepo port.DailyAttendanceRepository
	attLogRepo          port.AttendanceLogRepository
	auditRepo           port.AuditLogRepository
	correctionRepo      port.AttendanceCorrectionRepository
	rotationRepo        port.RotationPatternRepository
	swapRepo            port.ShiftSwapRequestRepository
	attendancePolicy    AttendancePolicy
}

func NewAttendanceProcessorService(
	employeeRepo port.EmployeeRepository,
	shiftRepo port.ShiftRepository,
	empShiftRepo port.EmployeeShiftRepository,
	leaveRepo port.LeaveRequestRepository,
	otRepo port.OvertimeRequestRepository,
	dailyAttendanceRepo port.DailyAttendanceRepository,
	attLogRepo port.AttendanceLogRepository,
	auditRepo port.AuditLogRepository,
	correctionRepo port.AttendanceCorrectionRepository,
	rotationRepo port.RotationPatternRepository,
	swapRepo port.ShiftSwapRequestRepository,
	policies ...AttendancePolicy,
) *AttendanceProcessorService {
	policy := defaultAttendancePolicy()
	if len(policies) > 0 {
		policy = normalizeAttendancePolicy(policies[0])
	}
	return &AttendanceProcessorService{
		employeeRepo:        employeeRepo,
		shiftRepo:           shiftRepo,
		empShiftRepo:        empShiftRepo,
		leaveRepo:           leaveRepo,
		otRepo:              otRepo,
		dailyAttendanceRepo: dailyAttendanceRepo,
		attLogRepo:          attLogRepo,
		auditRepo:           auditRepo,
		correctionRepo:      correctionRepo,
		rotationRepo:        rotationRepo,
		swapRepo:            swapRepo,
		attendancePolicy:    policy,
	}
}


// === Ca Làm Việc (Shift) ===

func parseAndNormalizeTime(tStr string) (string, error) {
	if t, err := time.Parse("15:04:05", tStr); err == nil {
		return t.Format("15:04"), nil
	}
	if t, err := time.Parse("15:04", tStr); err == nil {
		return t.Format("15:04"), nil
	}
	return "", fmt.Errorf("invalid time format")
}

func (s *AttendanceProcessorService) CreateShift(ctx context.Context, name, startTime, endTime string, breakMinutes, lateGrace, earlyGrace, maxWorking int, timezone string) (*entity.Shift, error) {
	start, err := parseAndNormalizeTime(startTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time")
	}
	end, err := parseAndNormalizeTime(endTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_time")
	}
	startTime = start
	endTime = end

	if breakMinutes < 0 || lateGrace < 0 || earlyGrace < 0 || maxWorking < 0 {
		return nil, fmt.Errorf("attendance policy minutes cannot be negative")
	}
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}
	shift := &entity.Shift{
		Name:         name,
		StartTime:    startTime,
		EndTime:      endTime,
		BreakMinutes: breakMinutes, LateGraceMinutes: lateGrace, EarlyGraceMinutes: earlyGrace, MaxWorkingMinutes: maxWorking, Timezone: timezone,
	}
	if err := s.shiftRepo.Create(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (s *AttendanceProcessorService) ListShifts(ctx context.Context) ([]entity.Shift, error) {
	return s.shiftRepo.List(ctx)
}

func (s *AttendanceProcessorService) DeleteShift(ctx context.Context, id string) error {
	return s.shiftRepo.Delete(ctx, id)
}

func (s *AttendanceProcessorService) UpdateShift(ctx context.Context, id, name, startTime, endTime string, breakMinutes, lateGrace, earlyGrace, maxWorking int, timezone string) (*entity.Shift, error) {
	start, err := parseAndNormalizeTime(startTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time")
	}
	end, err := parseAndNormalizeTime(endTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_time")
	}
	startTime = start
	endTime = end

	if breakMinutes < 0 || lateGrace < 0 || earlyGrace < 0 || maxWorking < 0 {
		return nil, fmt.Errorf("attendance policy minutes cannot be negative")
	}
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	shift, err := s.shiftRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, fmt.Errorf("shift not found")
	}

	shift.Name = name
	shift.StartTime = startTime
	shift.EndTime = endTime
	shift.BreakMinutes = breakMinutes
	shift.LateGraceMinutes = lateGrace
	shift.EarlyGraceMinutes = earlyGrace
	shift.MaxWorkingMinutes = maxWorking
	shift.Timezone = timezone

	if err := s.shiftRepo.Update(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

// === Gán Ca (Employee Shift) ===

func (s *AttendanceProcessorService) AssignShift(ctx context.Context, employeeID string, shiftID, rotationPatternID *string, startDate time.Time, endDate *time.Time) (*entity.EmployeeShift, error) {
	es := &entity.EmployeeShift{
		EmployeeID:        employeeID,
		ShiftID:           shiftID,
		RotationPatternID: rotationPatternID,
		StartDate:         startDate,
		EndDate:           endDate,
	}
	if err := s.empShiftRepo.Create(ctx, es); err != nil {
		return nil, err
	}
	return es, nil
}

func (s *AttendanceProcessorService) ListEmployeeShifts(ctx context.Context) ([]entity.EmployeeShift, error) {
	return s.empShiftRepo.List(ctx)
}

func (s *AttendanceProcessorService) DeleteEmployeeShift(ctx context.Context, id string) error {
	return s.empShiftRepo.Delete(ctx, id)
}

func (s *AttendanceProcessorService) AssignShiftBatch(ctx context.Context, employeeIDs []string, shiftID, rotationPatternID *string, startDate time.Time, endDate *time.Time) error {
	for _, empID := range employeeIDs {
		_, err := s.AssignShift(ctx, empID, shiftID, rotationPatternID, startDate, endDate)
		if err != nil {
			return err
		}
	}
	return nil
}

// === Chu kỳ xoay ca (Rotation Pattern) ===

func (s *AttendanceProcessorService) CreateRotationPattern(ctx context.Context, name string, sequence string) (*entity.RotationPattern, error) {
	rp := &entity.RotationPattern{
		Name:            name,
		PatternSequence: sequence,
	}
	if err := s.rotationRepo.Create(ctx, rp); err != nil {
		return nil, err
	}
	return rp, nil
}

func (s *AttendanceProcessorService) ListRotationPatterns(ctx context.Context) ([]entity.RotationPattern, error) {
	return s.rotationRepo.List(ctx)
}

func (s *AttendanceProcessorService) DeleteRotationPattern(ctx context.Context, id string) error {
	return s.rotationRepo.Delete(ctx, id)
}

func (s *AttendanceProcessorService) UpdateRotationPattern(ctx context.Context, id string, name string, sequence string) (*entity.RotationPattern, error) {
	rp, err := s.rotationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rp == nil {
		return nil, fmt.Errorf("rotation pattern not found")
	}

	rp.Name = name
	rp.PatternSequence = sequence

	if err := s.rotationRepo.Update(ctx, rp); err != nil {
		return nil, err
	}
	return rp, nil
}

// === Đơn đổi ca (Shift Swap Request) ===

func (s *AttendanceProcessorService) CreateShiftSwapRequest(ctx context.Context, requestingEmployeeID, targetEmployeeID string, requestingDate, targetDate time.Time) (*entity.ShiftSwapRequest, error) {
	ssr := &entity.ShiftSwapRequest{
		RequestingEmployeeID: requestingEmployeeID,
		TargetEmployeeID:     targetEmployeeID,
		RequestingDate:       requestingDate,
		TargetDate:           targetDate,
	}
	if err := s.swapRepo.Create(ctx, ssr); err != nil {
		return nil, err
	}
	return ssr, nil
}

func (s *AttendanceProcessorService) ListShiftSwapRequests(ctx context.Context) ([]entity.ShiftSwapRequest, error) {
	return s.swapRepo.List(ctx)
}

func (s *AttendanceProcessorService) ApproveShiftSwapRequest(ctx context.Context, id string, approvedBy string) error {
	ssr, err := s.swapRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ssr == nil {
		return fmt.Errorf("shift swap request not found")
	}

	if err := s.swapRepo.UpdateStatus(ctx, id, "approved", approvedBy); err != nil {
		return err
	}

	// Xử lý lại công cho cả 2 nhân viên trong các ngày liên quan
	_ = s.ProcessDailyAttendanceForEmployee(ctx, ssr.RequestingEmployeeID, ssr.RequestingDate)
	_ = s.ProcessDailyAttendanceForEmployee(ctx, ssr.TargetEmployeeID, ssr.TargetDate)

	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": ssr.RequestingEmployeeID,
		"date":        ssr.RequestingDate.Format("2006-01-02"),
	})
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": ssr.TargetEmployeeID,
		"date":        ssr.TargetDate.Format("2006-01-02"),
	})

	return nil
}

func (s *AttendanceProcessorService) RejectShiftSwapRequest(ctx context.Context, id string, approvedBy string) error {
	ssr, err := s.swapRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ssr == nil {
		return fmt.Errorf("shift swap request not found")
	}

	if err := s.swapRepo.UpdateStatus(ctx, id, "rejected", approvedBy); err != nil {
		return err
	}
	return nil
}

// === Giải quyết Ca làm việc thực tế cho Nhân viên vào ngày cụ thể ===

func (s *AttendanceProcessorService) ResolveShiftForEmployeeOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.Shift, *entity.EmployeeShift, error) {
	swap, err := s.swapRepo.GetApprovedSwapForEmployeeOnDate(ctx, employeeID, date)
	if err != nil {
		return nil, nil, err
	}

	var baseEmployeeID string
	var baseDate time.Time

	if swap != nil {
		if swap.RequestingEmployeeID == employeeID && swap.RequestingDate.Format("2006-01-02") == date.Format("2006-01-02") {
			baseEmployeeID = swap.TargetEmployeeID
			baseDate = swap.TargetDate
		} else {
			baseEmployeeID = swap.RequestingEmployeeID
			baseDate = swap.RequestingDate
		}
	} else {
		baseEmployeeID = employeeID
		baseDate = date
	}

	mapping, err := s.empShiftRepo.GetActiveShiftForEmployee(ctx, baseEmployeeID, baseDate)
	if err != nil {
		return nil, nil, err
	}
	if mapping == nil {
		return nil, nil, nil
	}

	if mapping.ShiftID != nil && *mapping.ShiftID != "" {
		shift, err := s.shiftRepo.GetByID(ctx, *mapping.ShiftID)
		if err != nil {
			return nil, nil, err
		}
		return shift, mapping, nil
	}

	if mapping.RotationPatternID != nil && *mapping.RotationPatternID != "" {
		pattern, err := s.rotationRepo.GetByID(ctx, *mapping.RotationPatternID)
		if err != nil {
			return nil, nil, err
		}
		if pattern == nil {
			return nil, mapping, nil
		}

		type PatternItem struct {
			ShiftID  *string `json:"shift_id"`
			Duration int     `json:"duration"`
		}
		var sequence []PatternItem
		if err := json.Unmarshal([]byte(pattern.PatternSequence), &sequence); err != nil {
			return nil, mapping, err
		}

		totalCycleDays := 0
		for _, item := range sequence {
			totalCycleDays += item.Duration
		}
		if totalCycleDays == 0 {
			return nil, mapping, nil
		}

		y1, m1, d1 := mapping.StartDate.Date()
		y2, m2, d2 := baseDate.Date()
		t1 := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
		daysDiff := int(t2.Sub(t1).Hours() / 24)

		if daysDiff < 0 {
			return nil, mapping, nil
		}

		cycleDay := daysDiff % totalCycleDays
		accumulated := 0
		var selectedShiftID *string
		for _, item := range sequence {
			if cycleDay < accumulated+item.Duration {
				selectedShiftID = item.ShiftID
				break
			}
			accumulated += item.Duration
		}

		if selectedShiftID != nil && *selectedShiftID != "" {
			shift, err := s.shiftRepo.GetByID(ctx, *selectedShiftID)
			if err != nil {
				return nil, mapping, err
			}
			return shift, mapping, nil
		}
		return nil, mapping, nil
	}

	return nil, mapping, nil
}


// === Nghỉ Phép (Leave Request) ===

func (s *AttendanceProcessorService) SubmitLeaveRequest(ctx context.Context, employeeID, leaveType string, startDate, endDate time.Time, reason string) (*entity.LeaveRequest, error) {
	lr := &entity.LeaveRequest{
		EmployeeID: employeeID,
		LeaveType:  leaveType,
		StartDate:  startDate,
		EndDate:    endDate,
		Reason:     reason,
	}
	if err := s.leaveRepo.Create(ctx, lr); err != nil {
		return nil, err
	}
	return lr, nil
}

func (s *AttendanceProcessorService) ApproveLeaveRequest(ctx context.Context, id string, approvedBy string) error {
	lr, err := s.leaveRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if lr == nil {
		return fmt.Errorf("leave request not found")
	}
	if err := s.leaveRepo.UpdateStatus(ctx, id, "approved", approvedBy); err != nil {
		return err
	}
	for d := lr.StartDate; !d.After(lr.EndDate); d = d.AddDate(0, 0, 1) {
		_ = s.ProcessDailyAttendanceForEmployee(ctx, lr.EmployeeID, d)
	}
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": lr.EmployeeID,
		"date":        lr.StartDate.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) RejectLeaveRequest(ctx context.Context, id string, approvedBy string) error {
	lr, err := s.leaveRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if lr == nil {
		return fmt.Errorf("leave request not found")
	}
	if err := s.leaveRepo.UpdateStatus(ctx, id, "rejected", approvedBy); err != nil {
		return err
	}
	for d := lr.StartDate; !d.After(lr.EndDate); d = d.AddDate(0, 0, 1) {
		_ = s.ProcessDailyAttendanceForEmployee(ctx, lr.EmployeeID, d)
	}
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": lr.EmployeeID,
		"date":        lr.StartDate.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) ListLeaveRequests(ctx context.Context, employeeID, status string) ([]entity.LeaveRequest, error) {
	return s.leaveRepo.List(ctx, employeeID, status)
}

// === Làm Thêm Giờ (Overtime Request) ===

func (s *AttendanceProcessorService) SubmitOvertimeRequest(ctx context.Context, employeeID string, date time.Time, startTime, endTime string) (*entity.OvertimeRequest, error) {
	if employeeID == "" {
		return nil, fmt.Errorf("employee_id is required")
	}
	start, err := parseAndNormalizeTime(startTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time")
	}
	end, err := parseAndNormalizeTime(endTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end_time")
	}
	parsedStart, _ := time.Parse("15:04", start)
	parsedEnd, _ := time.Parse("15:04", end)

	if parsedStart.Hour() == parsedEnd.Hour() && parsedStart.Minute() == parsedEnd.Minute() {
		return nil, fmt.Errorf("overtime start_time and end_time must be different")
	}
	ot := &entity.OvertimeRequest{
		EmployeeID: employeeID,
		Date:       date,
		StartTime:  start,
		EndTime:    end,
	}
	if err := s.otRepo.Create(ctx, ot); err != nil {
		return nil, err
	}
	return ot, nil
}

func (s *AttendanceProcessorService) ApproveOvertimeRequest(ctx context.Context, id string, approvedBy string) error {
	ot, err := s.otRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ot == nil {
		return fmt.Errorf("overtime request not found")
	}
	if err := s.otRepo.UpdateStatus(ctx, id, "approved", approvedBy); err != nil {
		return err
	}
	_ = s.ProcessDailyAttendanceForEmployee(ctx, ot.EmployeeID, ot.Date)
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": ot.EmployeeID,
		"date":        ot.Date.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) RejectOvertimeRequest(ctx context.Context, id string, approvedBy string) error {
	ot, err := s.otRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ot == nil {
		return fmt.Errorf("overtime request not found")
	}
	if err := s.otRepo.UpdateStatus(ctx, id, "rejected", approvedBy); err != nil {
		return err
	}
	_ = s.ProcessDailyAttendanceForEmployee(ctx, ot.EmployeeID, ot.Date)
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": ot.EmployeeID,
		"date":        ot.Date.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) ListOvertimeRequests(ctx context.Context, employeeID, status string) ([]entity.OvertimeRequest, error) {
	return s.otRepo.List(ctx, employeeID, status)
}

// === Đơn Sửa Công (Attendance Correction) ===

func (s *AttendanceProcessorService) SubmitCorrectionRequest(ctx context.Context, employeeID string, date time.Time, correctedTime time.Time, checkType, reason string) (*entity.AttendanceCorrection, error) {
	ac := &entity.AttendanceCorrection{
		EmployeeID:    employeeID,
		Date:          date,
		CorrectedTime: correctedTime,
		CheckType:     checkType,
		Reason:        reason,
	}
	if err := s.correctionRepo.Create(ctx, ac); err != nil {
		return nil, err
	}
	return ac, nil
}

func (s *AttendanceProcessorService) ApproveCorrectionRequest(ctx context.Context, id string, approvedBy string) error {
	ac, err := s.correctionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ac == nil {
		return fmt.Errorf("attendance correction request not found")
	}
	if err := s.correctionRepo.UpdateStatus(ctx, id, "approved", approvedBy); err != nil {
		return err
	}

	// 1. Chèn log chấm công nhân tạo vào bảng attendance_log
	emp, err := s.employeeRepo.GetByID(ctx, ac.EmployeeID)
	if err != nil || emp == nil {
		return fmt.Errorf("employee not found for correction")
	}

	log := &entity.AttendanceLog{
		DeviceID:     "manual",
		EmployeeCode: emp.EmployeeCode,
		CheckTime:    ac.CorrectedTime,
		CheckType:    entity.CheckType(ac.CheckType),
		VerifyMode:   "manual",
		SyncedAt:     time.Now(),
	}
	_, _ = s.attLogRepo.BulkInsert(ctx, []entity.AttendanceLog{*log})

	// 2. Chạy lại tính toán bảng công cho ngày đó
	_ = s.ProcessDailyAttendanceForEmployee(ctx, ac.EmployeeID, ac.Date)
	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"employee_id": ac.EmployeeID,
		"date":        ac.Date.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) RejectCorrectionRequest(ctx context.Context, id string, approvedBy string) error {
	ac, err := s.correctionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ac == nil {
		return fmt.Errorf("attendance correction request not found")
	}
	if err := s.correctionRepo.UpdateStatus(ctx, id, "rejected", approvedBy); err != nil {
		return err
	}
	return nil
}

func (s *AttendanceProcessorService) ListCorrectionRequests(ctx context.Context, employeeID, status string) ([]entity.AttendanceCorrection, error) {
	return s.correctionRepo.List(ctx, employeeID, status)
}

// === Tính Công Ngày (Daily Attendance Processing Engine) ===

func (s *AttendanceProcessorService) ProcessDailyAttendance(ctx context.Context, date time.Time) error {
	employees, err := s.employeeRepo.ListActive(ctx)
	if err != nil {
		return err
	}

	for _, emp := range employees {
		if err := s.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date); err != nil {
			log.Printf("[AttendanceProcessor] error processing for employee %s (%s) on date %s: %v", emp.FullName, emp.ID, date.Format("2006-01-02"), err)
			continue
		}
	}

	broadcast.Global.Broadcast("attendance_processed", map[string]any{
		"date": date.Format("2006-01-02"),
	})
	return nil
}

func (s *AttendanceProcessorService) GetDailyAttendanceReport(ctx context.Context, employeeID string, from, to time.Time) ([]entity.DailyAttendance, error) {
	return s.dailyAttendanceRepo.Query(ctx, employeeID, from, to)
}

// === Audit Log ===

func (s *AttendanceProcessorService) CreateAuditLog(ctx context.Context, userID *string, action, objectType, objectID, description, ip string) error {
	al := &entity.AuditLog{
		UserID:      userID,
		Action:      action,
		ObjectType:  objectType,
		ObjectID:    objectID,
		Description: description,
		IPAddress:   ip,
	}
	return s.auditRepo.Create(ctx, al)
}

func (s *AttendanceProcessorService) ListAuditLogs(ctx context.Context, limit, offset int) ([]entity.AuditLog, error) {
	return s.auditRepo.List(ctx, limit, offset)
}

// Helper kết hợp ngày và chuỗi giờ "HH:MM"
func (s *AttendanceProcessorService) combineDateAndTime(date time.Time, timeStr string) time.Time {
	var hour, min int
	_, _ = fmt.Sscanf(timeStr, "%d:%d", &hour, &min)
	return time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, date.Location())
}
