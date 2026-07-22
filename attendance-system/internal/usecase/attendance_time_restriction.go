package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"attendance-system/internal/domain/entity"
)

const (
	invalidReasonNoShift       = "No shift assigned"
	invalidReasonOutsideWindow = "Outside shift window and no approved overtime"
	workSegmentRegular         = "regular"
	workSegmentOvertime        = "overtime"
	workSegmentInvalid         = "invalid"
)

type AttendancePolicy struct {
	ShiftWindowBeforeMinutes  int
	ShiftWindowAfterMinutes   int
	OvertimeRequiresApproval  bool
	OvertimeRequiresDeviceLog bool
	OvertimeLogGraceMinutes   int
}

func defaultAttendancePolicy() AttendancePolicy {
	return AttendancePolicy{
		ShiftWindowBeforeMinutes:  60,
		ShiftWindowAfterMinutes:   60,
		OvertimeRequiresApproval:  true,
		OvertimeRequiresDeviceLog: true,
		OvertimeLogGraceMinutes:   15,
	}
}

func normalizeAttendancePolicy(policy AttendancePolicy) AttendancePolicy {
	if policy.ShiftWindowBeforeMinutes < 0 {
		policy.ShiftWindowBeforeMinutes = 60
	}
	if policy.ShiftWindowAfterMinutes < 0 {
		policy.ShiftWindowAfterMinutes = 60
	}
	if policy.OvertimeLogGraceMinutes < 0 {
		policy.OvertimeLogGraceMinutes = 15
	}
	return policy
}

type attendanceInterval struct {
	start     time.Time
	end       time.Time
	requestID string
}

// ProcessDailyAttendanceForEmployee tách giờ thường và OT. Cửa sổ ±1 giờ
// chỉ dùng nhận diện log; giờ thường luôn bị giới hạn trong biên ca.
func (s *AttendanceProcessorService) ProcessDailyAttendanceForEmployee(ctx context.Context, employeeID string, date time.Time) error {
	emp, err := s.employeeRepo.GetByID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp == nil {
		return fmt.Errorf("employee not found: %s", employeeID)
	}

	shift, _, err := s.ResolveShiftForEmployeeOnDate(ctx, employeeID, date)
	if err != nil {
		return err
	}

	loc := time.Local
	if shift != nil && shift.Timezone != "" {
		loc, err = time.LoadLocation(shift.Timezone)
		if err != nil {
			return err
		}
	}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Nanosecond)
	da := &entity.DailyAttendance{
		EmployeeID:       employeeID,
		Date:             dayStart,
		AttendanceStatus: "absent",
	}

	if shift == nil {
		logs, queryErr := s.attLogRepo.Query(ctx, dayStart, dayEnd, emp.EmployeeCode, "")
		if queryErr != nil {
			return queryErr
		}
		for _, attendanceLog := range logs {
			if err := s.attLogRepo.UpdateClassification(ctx, attendanceLog.ID, false, invalidReasonNoShift, workSegmentInvalid, nil); err != nil {
				return err
			}
		}
		return s.dailyAttendanceRepo.Upsert(ctx, da)
	}
	da.ShiftID = &shift.ID

	shiftStart := s.combineDateAndTime(dayStart, shift.StartTime)
	shiftEnd := s.combineDateAndTime(dayStart, shift.EndTime)
	if !shiftEnd.After(shiftStart) {
		shiftEnd = shiftEnd.AddDate(0, 0, 1)
	}
	regularWindowStart := shiftStart.Add(-time.Duration(s.attendancePolicy.ShiftWindowBeforeMinutes) * time.Minute)
	regularWindowEnd := shiftEnd.Add(time.Duration(s.attendancePolicy.ShiftWindowAfterMinutes) * time.Minute)

	approvedOT, err := s.otRepo.ListApprovedOTOnDate(ctx, employeeID, dayStart)
	if err != nil {
		return err
	}
	otWindows := s.buildOvertimeWindows(dayStart, shiftEnd, approvedOT)

	queryStart := dayStart
	if regularWindowStart.Before(queryStart) {
		queryStart = regularWindowStart
	}
	queryEnd := dayEnd
	if regularWindowEnd.After(queryEnd) {
		queryEnd = regularWindowEnd
	}
	grace := time.Duration(s.attendancePolicy.OvertimeLogGraceMinutes) * time.Minute
	for _, window := range otWindows {
		if candidate := window.start.Add(-grace); candidate.Before(queryStart) {
			queryStart = candidate
		}
		if candidate := window.end.Add(grace); candidate.After(queryEnd) {
			queryEnd = candidate
		}
	}

	logs, err := s.attLogRepo.Query(ctx, queryStart, queryEnd, emp.EmployeeCode, "")
	if err != nil {
		return err
	}
	validLogs := make([]entity.AttendanceLog, 0, len(logs))
	for _, attendanceLog := range logs {
		segment, requestID := classifyAttendanceLog(attendanceLog.CheckTime, shiftStart, shiftEnd, regularWindowStart, regularWindowEnd, otWindows, grace)
		isValid := segment != workSegmentInvalid
		reason := ""
		if !isValid {
			reason = invalidReasonOutsideWindow
		}
		if err := s.attLogRepo.UpdateClassification(ctx, attendanceLog.ID, isValid, reason, segment, requestID); err != nil {
			return err
		}
		if isValid {
			attendanceLog.IsValid = true
			attendanceLog.WorkSegment = segment
			attendanceLog.OvertimeRequestID = requestID
			validLogs = append(validLogs, attendanceLog)
		}
	}

	if len(validLogs) == 0 {
		return s.upsertAbsentOrLeave(ctx, da, employeeID, dayStart)
	}

	firstIn, lastOut, complete := attendanceBounds(validLogs)
	if !firstIn.IsZero() {
		da.FirstIn = &firstIn
	}
	if complete {
		da.LastOut = &lastOut
	} else {
		return s.dailyAttendanceRepo.Upsert(ctx, da)
	}

	regularStart := maxTime(firstIn, shiftStart)
	regularEnd := minTime(lastOut, shiftEnd)
	regularDuration := time.Duration(0)
	if regularEnd.After(regularStart) {
		regularDuration = regularEnd.Sub(regularStart)
		if regularDuration > 5*time.Hour && shift.BreakMinutes > 0 {
			regularDuration -= time.Duration(shift.BreakMinutes) * time.Minute
		}
		if regularDuration < 0 {
			regularDuration = 0
		}
		if shift.MaxWorkingMinutes > 0 {
			maxDuration := time.Duration(shift.MaxWorkingMinutes) * time.Minute
			if regularDuration > maxDuration {
				regularDuration = maxDuration
			}
		}
	}
	da.RegularWorkingMinutes = int(regularDuration / time.Minute)

	if firstIn.After(shiftStart.Add(time.Duration(shift.LateGraceMinutes) * time.Minute)) {
		da.LateMinutes = int(firstIn.Sub(shiftStart) / time.Minute)
	}
	if lastOut.Before(shiftEnd.Add(-time.Duration(shift.EarlyGraceMinutes) * time.Minute)) {
		da.EarlyMinutes = int(shiftEnd.Sub(lastOut) / time.Minute)
	}

	if da.RegularWorkingMinutes > 0 {
		switch {
		case da.LateMinutes > 0:
			da.AttendanceStatus = "late"
		case da.EarlyMinutes > 0:
			da.AttendanceStatus = "early"
		default:
			da.AttendanceStatus = "present"
		}
	}

	if s.attendancePolicy.OvertimeRequiresDeviceLog {
		da.OvertimeMinutes = actualOvertimeMinutes(firstIn, lastOut, shiftStart, shiftEnd, otWindows)
	} else {
		da.OvertimeMinutes = approvedOvertimeMinutes(shiftStart, shiftEnd, otWindows)
	}
	da.WorkingHours = float64(da.RegularWorkingMinutes+da.OvertimeMinutes) / 60

	leave, leaveErr := s.leaveRepo.CheckLeaveOnDate(ctx, employeeID, dayStart)
	if leaveErr == nil && leave != nil {
		da.LeaveID = &leave.ID
	}
	return s.dailyAttendanceRepo.Upsert(ctx, da)
}

func (s *AttendanceProcessorService) buildOvertimeWindows(dayStart, shiftEnd time.Time, requests []entity.OvertimeRequest) []attendanceInterval {
	windows := make([]attendanceInterval, 0, len(requests)+1)
	for _, request := range requests {
		start := s.combineDateAndTime(dayStart, request.StartTime)
		end := s.combineDateAndTime(dayStart, request.EndTime)
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
		windows = append(windows, attendanceInterval{start: start, end: end, requestID: request.ID})
	}
	if !s.attendancePolicy.OvertimeRequiresApproval && len(windows) == 0 {
		end := shiftEnd.Add(time.Duration(s.attendancePolicy.ShiftWindowAfterMinutes) * time.Minute)
		if end.After(shiftEnd) {
			windows = append(windows, attendanceInterval{start: shiftEnd, end: end})
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].start.Before(windows[j].start) })
	return windows
}

func classifyAttendanceLog(checkTime, shiftStart, shiftEnd, regularWindowStart, regularWindowEnd time.Time, otWindows []attendanceInterval, grace time.Duration) (string, *string) {
	outsideScheduledShift := checkTime.Before(shiftStart) || checkTime.After(shiftEnd)
	if outsideScheduledShift {
		for _, window := range otWindows {
			if !checkTime.Before(window.start.Add(-grace)) && !checkTime.After(window.end.Add(grace)) {
				if window.requestID == "" {
					return workSegmentOvertime, nil
				}
				requestID := window.requestID
				return workSegmentOvertime, &requestID
			}
		}
	}
	if !checkTime.Before(regularWindowStart) && !checkTime.After(regularWindowEnd) {
		return workSegmentRegular, nil
	}
	return workSegmentInvalid, nil
}

func attendanceBounds(logs []entity.AttendanceLog) (time.Time, time.Time, bool) {
	sort.Slice(logs, func(i, j int) bool { return logs[i].CheckTime.Before(logs[j].CheckTime) })
	var firstIn time.Time
	for _, attendanceLog := range logs {
		if attendanceLog.CheckType == entity.CheckTypeIn {
			firstIn = attendanceLog.CheckTime
			break
		}
	}
	if firstIn.IsZero() {
		firstIn = logs[0].CheckTime
	}

	var lastOut time.Time
	for i := len(logs) - 1; i >= 0; i-- {
		if logs[i].CheckType == entity.CheckTypeOut && logs[i].CheckTime.After(firstIn) {
			lastOut = logs[i].CheckTime
			break
		}
	}
	if lastOut.IsZero() {
		// Một số máy luôn trả trạng thái IN. Chỉ fallback khi không có OUT nào.
		hasAnyOut := false
		for _, attendanceLog := range logs {
			if attendanceLog.CheckType == entity.CheckTypeOut {
				hasAnyOut = true
				break
			}
		}
		if !hasAnyOut && len(logs) > 1 && logs[len(logs)-1].CheckTime.After(firstIn) {
			lastOut = logs[len(logs)-1].CheckTime
		}
	}
	return firstIn, lastOut, !lastOut.IsZero()
}

func actualOvertimeMinutes(firstIn, lastOut, shiftStart, shiftEnd time.Time, windows []attendanceInterval) int {
	if !lastOut.After(firstIn) {
		return 0
	}
	return overtimeMinutesWithin(firstIn, lastOut, shiftStart, shiftEnd, windows)
}

func approvedOvertimeMinutes(shiftStart, shiftEnd time.Time, windows []attendanceInterval) int {
	if len(windows) == 0 {
		return 0
	}
	start, end := windows[0].start, windows[0].end
	for _, window := range windows[1:] {
		if window.start.Before(start) {
			start = window.start
		}
		if window.end.After(end) {
			end = window.end
		}
	}
	return overtimeMinutesWithin(start, end, shiftStart, shiftEnd, windows)
}

func overtimeMinutesWithin(presenceStart, presenceEnd, shiftStart, shiftEnd time.Time, windows []attendanceInterval) int {
	merged := mergeIntervals(windows)
	total := time.Duration(0)
	for _, window := range merged {
		intersectionStart := maxTime(presenceStart, window.start)
		intersectionEnd := minTime(presenceEnd, window.end)
		if !intersectionEnd.After(intersectionStart) {
			continue
		}
		total += intersectionEnd.Sub(intersectionStart)
		regularOverlapStart := maxTime(intersectionStart, shiftStart)
		regularOverlapEnd := minTime(intersectionEnd, shiftEnd)
		if regularOverlapEnd.After(regularOverlapStart) {
			total -= regularOverlapEnd.Sub(regularOverlapStart)
		}
	}
	if total < 0 {
		return 0
	}
	return int(total / time.Minute)
}

func mergeIntervals(windows []attendanceInterval) []attendanceInterval {
	if len(windows) == 0 {
		return nil
	}
	items := append([]attendanceInterval(nil), windows...)
	sort.Slice(items, func(i, j int) bool { return items[i].start.Before(items[j].start) })
	merged := []attendanceInterval{items[0]}
	for _, current := range items[1:] {
		last := &merged[len(merged)-1]
		if !current.start.After(last.end) {
			if current.end.After(last.end) {
				last.end = current.end
			}
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func (s *AttendanceProcessorService) upsertAbsentOrLeave(ctx context.Context, da *entity.DailyAttendance, employeeID string, dayStart time.Time) error {
	leave, err := s.leaveRepo.CheckLeaveOnDate(ctx, employeeID, dayStart)
	if err != nil {
		return err
	}
	if leave != nil {
		da.AttendanceStatus = "leave"
		da.WorkingHours = 8
		da.RegularWorkingMinutes = 480
	}
	return s.dailyAttendanceRepo.Upsert(ctx, da)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
