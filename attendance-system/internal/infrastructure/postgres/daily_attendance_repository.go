package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type DailyAttendanceRepository struct {
	pool *pgxpool.Pool
}

func NewDailyAttendanceRepository(pool *pgxpool.Pool) *DailyAttendanceRepository {
	return &DailyAttendanceRepository{pool: pool}
}

func (r *DailyAttendanceRepository) Upsert(ctx context.Context, da *entity.DailyAttendance) error {
	query := `
		INSERT INTO daily_attendance (employee_id, date, shift_id, first_in, last_out, late_minutes, early_minutes, working_hours, attendance_status, overtime_minutes, regular_working_minutes, leave_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (employee_id, date) DO UPDATE
		SET shift_id = EXCLUDED.shift_id,
		    first_in = EXCLUDED.first_in,
		    last_out = EXCLUDED.last_out,
		    late_minutes = EXCLUDED.late_minutes,
		    early_minutes = EXCLUDED.early_minutes,
		    working_hours = EXCLUDED.working_hours,
		    attendance_status = EXCLUDED.attendance_status,
		    overtime_minutes = EXCLUDED.overtime_minutes,
		    regular_working_minutes = EXCLUDED.regular_working_minutes,
		    leave_id = EXCLUDED.leave_id,
		    updated_at = now()
		RETURNING id, created_at, updated_at`
	var shiftIDPtr *string
	if da.ShiftID != nil && *da.ShiftID != "" {
		shiftIDPtr = da.ShiftID
	}
	var leaveIDPtr *string
	if da.LeaveID != nil && *da.LeaveID != "" {
		leaveIDPtr = da.LeaveID
	}
	return r.pool.QueryRow(ctx, query,
		da.EmployeeID, da.Date, shiftIDPtr, da.FirstIn, da.LastOut,
		da.LateMinutes, da.EarlyMinutes, da.WorkingHours, da.AttendanceStatus,
		da.OvertimeMinutes, da.RegularWorkingMinutes, leaveIDPtr,
	).Scan(&da.ID, &da.CreatedAt, &da.UpdatedAt)
}

func (r *DailyAttendanceRepository) Query(ctx context.Context, employeeID string, from, to time.Time) ([]entity.DailyAttendance, error) {
	query := `
		SELECT id, employee_id, date, shift_id, first_in, last_out, late_minutes, early_minutes, working_hours, attendance_status, overtime_minutes, regular_working_minutes, leave_id, created_at, updated_at
		FROM daily_attendance
		WHERE ($1 = '' OR employee_id::text = $1)
		  AND date BETWEEN $2 AND $3
		ORDER BY date ASC`
	rows, err := r.pool.Query(ctx, query, employeeID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.DailyAttendance
	for rows.Next() {
		var da entity.DailyAttendance
		var shiftID *string
		var leaveID *string
		var firstIn, lastOut *time.Time
		if err := rows.Scan(&da.ID, &da.EmployeeID, &da.Date, &shiftID, &firstIn, &lastOut,
			&da.LateMinutes, &da.EarlyMinutes, &da.WorkingHours, &da.AttendanceStatus, &da.OvertimeMinutes, &da.RegularWorkingMinutes, &leaveID, &da.CreatedAt, &da.UpdatedAt); err != nil {
			return nil, err
		}
		da.ShiftID = shiftID
		da.LeaveID = leaveID
		da.FirstIn = firstIn
		da.LastOut = lastOut
		list = append(list, da)
	}
	return list, rows.Err()
}
