package test

import (
	"context"
	"testing"

	"attendance-system/internal/domain/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessor_ApprovedOvertimeIsNotDoubleCounted(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-ot", EmployeeCode: "NVOT", FullName: "Nhan vien OT"}
	shift := buildShift("shift-ot", "08:00", "17:00", 60, 5, 5)
	mapping := &entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shift.ID}
	otRequest := entity.OvertimeRequest{ID: "ot-1", EmployeeID: emp.ID, Date: date, StartTime: "17:00", EndTime: "19:00", Status: "approved"}
	logs := []entity.AttendanceLog{
		{ID: 501, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 8, 0, 0), CheckType: entity.CheckTypeIn},
		{ID: 502, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 17, 0, 0), CheckType: entity.CheckTypeOut},
		{ID: 503, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 19, 0, 0), CheckType: entity.CheckTypeOut},
	}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(mapping, nil).Once()
	repos.shiftRepo.On("GetByID", ctx, shift.ID).Return(shift, nil).Once()
	repos.otRepo.On("ListApprovedOTOnDate", ctx, emp.ID, mock.Anything).Return([]entity.OvertimeRequest{otRequest}, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(501), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(502), true, "", "regular", (*string)(nil)).Return(nil).Once()
	otID := "ot-1"
	repos.attRepo.On("UpdateClassification", ctx, int64(503), true, "", "overtime", &otID).Return(nil).Once()
	repos.leaveRepo.On("CheckLeaveOnDate", ctx, emp.ID, mock.Anything).Return(nil, nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.AttendanceStatus == "present" && da.RegularWorkingMinutes == 480 &&
			da.OvertimeMinutes == 120 && da.WorkingHours == 10 && da.FirstIn != nil && da.LastOut != nil
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}

func TestProcessor_UnapprovedLateLogIsInvalidAndNotOvertime(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-no-ot", EmployeeCode: "NVNOOT", FullName: "Nhan vien"}
	shift := buildShift("shift-no-ot", "08:00", "17:00", 60, 5, 5)
	mapping := &entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shift.ID}
	logs := []entity.AttendanceLog{
		{ID: 601, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 8, 0, 0), CheckType: entity.CheckTypeIn},
		{ID: 602, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 17, 0, 0), CheckType: entity.CheckTypeOut},
		{ID: 603, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 19, 0, 0), CheckType: entity.CheckTypeOut},
	}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(mapping, nil).Once()
	repos.shiftRepo.On("GetByID", ctx, shift.ID).Return(shift, nil).Once()
	repos.otRepo.On("ListApprovedOTOnDate", ctx, emp.ID, mock.Anything).Return([]entity.OvertimeRequest{}, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(601), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(602), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(603), false, "Outside shift window and no approved overtime", "invalid", (*string)(nil)).Return(nil).Once()
	repos.leaveRepo.On("CheckLeaveOnDate", ctx, emp.ID, mock.Anything).Return(nil, nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.AttendanceStatus == "present" && da.RegularWorkingMinutes == 480 && da.OvertimeMinutes == 0 && da.WorkingHours == 8
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}
