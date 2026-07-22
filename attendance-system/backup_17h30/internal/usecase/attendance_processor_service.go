package usecase

import (
	"context"
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
) *AttendanceProcessorService {
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
	}
}

// === Ca Làm Việc (Shift) ===

func (s *AttendanceProcessorService) CreateShift(ctx context.Context, name, startTime, endTime string, breakMinutes, lateGrace, earlyGrace, maxWorking int, timezone string) (*entity.Shift, error) {
	if _, err := time.Parse("15:04", startTime); err != nil {
		return nil, fmt.Errorf("invalid start_time")
	}
	if _, err := time.Parse("15:04", endTime); err != nil {
		return nil, fmt.Errorf("invalid end_time")
	}
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

// === Gán Ca (Employee Shift) ===

func (s *AttendanceProcessorService) AssignShift(ctx context.Context, employeeID, shiftID string, startDate time.Time, endDate *time.Time) (*entity.EmployeeShift, error) {
	es := &entity.EmployeeShift{
		EmployeeID: employeeID,
		ShiftID:    shiftID,
		StartDate:  startDate,
		EndDate:    endDate,
	}
	if err := s.empShiftRepo.Create(ctx, es); err != nil {
		return nil, err
	}
	return es, nil
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
	ot := &entity.OvertimeRequest{
		EmployeeID: employeeID,
		Date:       date,
		StartTime:  startTime,
		EndTime:    endTime,
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

func (s *AttendanceProcessorService) ProcessDailyAttendanceForEmployee(ctx context.Context, employeeID string, date time.Time) error {
	// 1. Lấy thông tin nhân viên
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}

	// 2. Tìm ca làm việc đang áp dụng vào ngày này
	activeShiftMapping, err := s.empShiftRepo.GetActiveShiftForEmployee(ctx, employeeID, date)
	if err != nil {
		return err
	}

	var shift *entity.Shift
	if activeShiftMapping != nil {
		shift, err = s.shiftRepo.GetByID(ctx, activeShiftMapping.ShiftID)
		if err != nil {
			return err
		}
	}

	// Xác định khoảng thời gian của ngày
	loc := time.Local
	if shift != nil && shift.Timezone != "" {
		loc, err = time.LoadLocation(shift.Timezone)
		if err != nil {
			return err
		}
	}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, loc)
	if shift != nil && shift.EndTime <= shift.StartTime {
		dayEnd = dayEnd.AddDate(0, 0, 1)
	}

	// 3. Lấy toàn bộ log chấm công thô của ngày hôm đó
	logs, err := s.attLogRepo.Query(ctx, dayStart, dayEnd, emp.EmployeeCode, "")
	if err != nil {
		return err
	}

	da := &entity.DailyAttendance{
		EmployeeID:       employeeID,
		Date:             dayStart,
		AttendanceStatus: "absent",
		WorkingHours:     0.0,
	}

	if shift != nil {
		da.ShiftID = &shift.ID
	}

	// 4. Nếu không có log chấm công -> Kiểm tra nghỉ phép
	if len(logs) == 0 {
		leave, err := s.leaveRepo.CheckLeaveOnDate(ctx, employeeID, dayStart)
		if err != nil {
			return err
		}
		if leave != nil {
			da.AttendanceStatus = "leave"
			da.WorkingHours = 8.0 // Nghỉ phép tính là 8h công
		} else {
			da.AttendanceStatus = "absent"
			da.WorkingHours = 0.0
		}
		return s.dailyAttendanceRepo.Upsert(ctx, da)
	}

	// 5. Nếu có log chấm công -> Tìm giờ vào đầu tiên (first_in) và giờ ra cuối cùng (last_out)
	var firstIn, lastOut time.Time
	for _, l := range logs {
		if firstIn.IsZero() || l.CheckTime.Before(firstIn) {
			firstIn = l.CheckTime
		}
		if lastOut.IsZero() || l.CheckTime.After(lastOut) {
			lastOut = l.CheckTime
		}
	}

	da.FirstIn = &firstIn
	if len(logs) > 1 && lastOut.After(firstIn) {
		da.LastOut = &lastOut
	}

	// Thuật toán Tự động nhận dạng Ca làm việc (Auto Shift Match)
	if shift == nil {
		allShifts, err := s.shiftRepo.List(ctx)
		if err == nil && len(allShifts) > 0 {
			var bestShift *entity.Shift
			minDiff := 180 // Khớp trong vòng 3 tiếng (180 phút)

			firstInHour, firstInMin, _ := firstIn.Clock()
			firstInTotalMin := firstInHour*60 + firstInMin

			for i := range allShifts {
				var shHour, shMin int
				_, errScan := fmt.Sscanf(allShifts[i].StartTime, "%d:%d", &shHour, &shMin)
				if errScan != nil {
					continue
				}
				shTotalMin := shHour*60 + shMin
				diff := firstInTotalMin - shTotalMin
				if diff < 0 {
					diff = -diff
				}
				// Xử lý vòng thời gian 24h
				if 1440-diff < diff {
					diff = 1440 - diff
				}

				if diff < minDiff {
					minDiff = diff
					bestShift = &allShifts[i]
				}
			}

			if bestShift != nil {
				shift = bestShift
				da.ShiftID = &bestShift.ID
			}
		}
	}

	// Nếu chỉ có 1 log (thiếu checkout)
	if da.LastOut == nil {
		da.AttendanceStatus = "absent"
		da.WorkingHours = 0.0
		return s.dailyAttendanceRepo.Upsert(ctx, da)
	}

	// 6. Tính toán đi muộn / về sớm nếu có ca làm việc
	if shift != nil {
		shiftStart := s.combineDateAndTime(dayStart, shift.StartTime)
		shiftEnd := s.combineDateAndTime(dayStart, shift.EndTime)
		if !shiftEnd.After(shiftStart) {
			shiftEnd = shiftEnd.AddDate(0, 0, 1)
		}

		// Đi muộn
		if firstIn.After(shiftStart.Add(time.Duration(shift.LateGraceMinutes) * time.Minute)) {
			da.LateMinutes = int(firstIn.Sub(shiftStart).Minutes())
		}
		// Về sớm
		if lastOut.Before(shiftEnd.Add(-time.Duration(shift.EarlyGraceMinutes) * time.Minute)) {
			da.EarlyMinutes = int(shiftEnd.Sub(lastOut).Minutes())
		}

		// Tính giờ công thực tế
		duration := lastOut.Sub(firstIn).Hours()
		// Trừ thời gian nghỉ trưa nếu ca làm việc dài hơn 5 tiếng
		if duration > 5.0 && shift.BreakMinutes > 0 {
			duration -= float64(shift.BreakMinutes) / 60.0
		}
		if duration < 0 {
			duration = 0
		}
		if shift.MaxWorkingMinutes > 0 && duration > float64(shift.MaxWorkingMinutes)/60.0 {
			duration = float64(shift.MaxWorkingMinutes) / 60.0
		}
		da.WorkingHours = duration

		// Xác định trạng thái
		if da.LateMinutes > 0 {
			da.AttendanceStatus = "late"
		} else if da.EarlyMinutes > 0 {
			da.AttendanceStatus = "early"
		} else {
			da.AttendanceStatus = "present"
		}
	} else {
		// Nếu không có ca (chấm công tự do)
		duration := lastOut.Sub(firstIn).Hours()
		if duration < 0 {
			duration = 0
		}
		da.WorkingHours = duration
		da.AttendanceStatus = "present"
	}

	// Ghi nhận mã nghỉ phép nếu có
	leave, err := s.leaveRepo.CheckLeaveOnDate(ctx, employeeID, dayStart)
	if err == nil && leave != nil {
		da.LeaveID = &leave.ID
	}

	// 7. Cộng giờ tăng ca (OT) nếu có đăng ký được duyệt
	ot, err := s.otRepo.GetApprovedOTOnDate(ctx, employeeID, dayStart)
	if err != nil {
		return err
	}
	if ot != nil {
		otStart := s.combineDateAndTime(dayStart, ot.StartTime)
		otEnd := s.combineDateAndTime(dayStart, ot.EndTime)
		if otEnd.After(otStart) {
			otHours := otEnd.Sub(otStart).Hours()
			da.WorkingHours += otHours
			da.OvertimeMinutes = int(otEnd.Sub(otStart).Minutes())
		}
	}

	return s.dailyAttendanceRepo.Upsert(ctx, da)
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
