package test

import (
	"context"
	"testing"

	"attendance-system/internal/domain/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type restrictionTestRepos struct {
	empRepo        *MockEmployeeRepository
	shiftRepo      *MockShiftRepository
	esRepo         *MockEmployeeShiftRepository
	leaveRepo      *MockLeaveRequestRepository
	otRepo         *MockOvertimeRequestRepository
	daRepo         *MockDailyAttendanceRepository
	attRepo        *MockAttendanceLogRepository
	auditRepo      *MockAuditLogRepository
	correctionRepo *MockAttendanceCorrectionRepository
}

func newRestrictionTestRepos() *restrictionTestRepos {
	return &restrictionTestRepos{
		empRepo:        new(MockEmployeeRepository),
		shiftRepo:      new(MockShiftRepository),
		esRepo:         new(MockEmployeeShiftRepository),
		leaveRepo:      new(MockLeaveRequestRepository),
		otRepo:         new(MockOvertimeRequestRepository),
		daRepo:         new(MockDailyAttendanceRepository),
		attRepo:        new(MockAttendanceLogRepository),
		auditRepo:      new(MockAuditLogRepository),
		correctionRepo: new(MockAttendanceCorrectionRepository),
	}
}

func TestProcessor_OutsideShiftWindowIsInvalidAndAbsent(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-1", EmployeeCode: "NV001", FullName: "Nhan vien 1"}
	shift := buildShift("shift-1", "08:00", "17:00", 0, 5, 5)
	mapping := &entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shift.ID}
	logs := []entity.AttendanceLog{{ID: 101, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 6, 30, 0)}}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(mapping, nil).Once()
	repos.shiftRepo.On("GetByID", ctx, shift.ID).Return(shift, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.otRepo.On("ListApprovedOTOnDate", ctx, emp.ID, mock.Anything).Return([]entity.OvertimeRequest{}, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(101), false, "Outside shift window and no approved overtime", "invalid", (*string)(nil)).Return(nil).Once()
	repos.leaveRepo.On("CheckLeaveOnDate", ctx, emp.ID, mock.Anything).Return(nil, nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.AttendanceStatus == "absent" && da.WorkingHours == 0 && da.FirstIn == nil && da.LastOut == nil
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}

func TestProcessor_ValidLateLogsAreUsedForAttendance(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-2", EmployeeCode: "NV002", FullName: "Nhan vien 2"}
	shift := buildShift("shift-2", "08:00", "17:00", 0, 5, 5)
	mapping := &entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shift.ID}
	logs := []entity.AttendanceLog{
		{ID: 201, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 8, 30, 0)},
		{ID: 202, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 17, 0, 0)},
	}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(mapping, nil).Once()
	repos.shiftRepo.On("GetByID", ctx, shift.ID).Return(shift, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(201), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(202), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.leaveRepo.On("CheckLeaveOnDate", ctx, emp.ID, mock.Anything).Return(nil, nil).Once()
	repos.otRepo.On("ListApprovedOTOnDate", ctx, emp.ID, mock.Anything).Return([]entity.OvertimeRequest{}, nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.AttendanceStatus == "late" && da.LateMinutes == 30 && da.FirstIn != nil &&
			da.FirstIn.Hour() == 8 && da.FirstIn.Minute() == 30 && da.LastOut != nil && da.WorkingHours == 8.5
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}

func TestProcessor_NoAssignedShiftInvalidatesAllLogs(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-3", EmployeeCode: "NV003", FullName: "Nhan vien 3"}
	logs := []entity.AttendanceLog{
		{ID: 301, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 8, 0, 0)},
		{ID: 302, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 17, 0, 0)},
	}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(nil, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(301), false, "No shift assigned", "invalid", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(302), false, "No shift assigned", "invalid", (*string)(nil)).Return(nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.ShiftID == nil && da.AttendanceStatus == "absent" && da.WorkingHours == 0 && da.FirstIn == nil && da.LastOut == nil
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}
