package usecase

import (
	"strings"
	"testing"
	"time"

	"attendance-system/internal/domain/entity"
)

func TestBuildEmployeeMonthlyCSVIsolatesEmployeeData(t *testing.T) {
	emp := entity.Employee{ID: "emp-a", EmployeeCode: "NV001", FullName: "Nguyễn Văn A"}
	date := "2026-07-20"
	firstIn := time.Date(2026, 7, 20, 8, 0, 0, 0, time.Local)
	matrix := map[string]entity.DailyAttendance{
		date: {EmployeeID: emp.ID, Date: firstIn, FirstIn: &firstIn, WorkingHours: 8, AttendanceStatus: "present"},
	}
	data, summary, err := BuildEmployeeMonthlyCSV(emp, matrix, 2026, 7)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "NV001") || !strings.Contains(text, "Nguyễn Văn A") {
		t.Fatalf("employee data missing from CSV: %s", text)
	}
	if strings.Contains(text, "NV002") || strings.Contains(text, "Nguyễn Văn B") {
		t.Fatalf("CSV contains another employee's data: %s", text)
	}
	if summary.WorkingHours != 8 || summary.PresentDays != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestSafeReportFilename(t *testing.T) {
	if got := safeReportFilename("NV/001 test"); got != "NV001test" {
		t.Fatalf("safeReportFilename = %q", got)
	}
}
