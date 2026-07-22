package test

import (
	"context"
	"testing"

	"attendance-system/internal/domain/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessor_OvernightShiftUsesNextDayLogsWithinWindow(t *testing.T) {
	ctx := context.Background()
	date := dayAt(2026, 7, 14, 0, 0, 0)
	emp := &entity.Employee{ID: "emp-night", EmployeeCode: "NVNIGHT", FullName: "Nhan vien ca dem"}
	shift := buildShift("shift-night", "22:00", "06:00", 0, 5, 5)
	mapping := &entity.EmployeeShift{EmployeeID: emp.ID, ShiftID: &shift.ID}
	logs := []entity.AttendanceLog{
		{ID: 401, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 22, 0, 0)},
		{ID: 402, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 15, 6, 0, 0)},
		{ID: 403, EmployeeCode: emp.EmployeeCode, CheckTime: dayAt(2026, 7, 14, 20, 30, 0)},
	}
	repos := newRestrictionTestRepos()

	repos.empRepo.On("GetByID", ctx, emp.ID).Return(emp, nil).Once()
	repos.esRepo.On("GetActiveShiftForEmployee", ctx, emp.ID, date).Return(mapping, nil).Once()
	repos.shiftRepo.On("GetByID", ctx, shift.ID).Return(shift, nil).Once()
	repos.attRepo.On("Query", ctx, mock.Anything, mock.Anything, emp.EmployeeCode, "").Return(logs, nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(401), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(402), true, "", "regular", (*string)(nil)).Return(nil).Once()
	repos.attRepo.On("UpdateClassification", ctx, int64(403), false, "Outside shift window and no approved overtime", "invalid", (*string)(nil)).Return(nil).Once()
	repos.leaveRepo.On("CheckLeaveOnDate", ctx, emp.ID, mock.Anything).Return(nil, nil).Once()
	repos.otRepo.On("ListApprovedOTOnDate", ctx, emp.ID, mock.Anything).Return([]entity.OvertimeRequest{}, nil).Once()
	repos.daRepo.On("Upsert", ctx, mock.MatchedBy(func(da *entity.DailyAttendance) bool {
		return da.AttendanceStatus == "present" && da.FirstIn != nil && da.FirstIn.Hour() == 22 &&
			da.LastOut != nil && da.LastOut.Hour() == 6 && da.WorkingHours == 8
	})).Return(nil).Once()

	processor := buildProcessor(repos.empRepo, repos.shiftRepo, repos.esRepo, repos.leaveRepo, repos.otRepo, repos.daRepo, repos.attRepo, repos.auditRepo, repos.correctionRepo)
	err := processor.ProcessDailyAttendanceForEmployee(ctx, emp.ID, date)

	assert.NoError(t, err)
	repos.attRepo.AssertExpectations(t)
	repos.daRepo.AssertExpectations(t)
}
