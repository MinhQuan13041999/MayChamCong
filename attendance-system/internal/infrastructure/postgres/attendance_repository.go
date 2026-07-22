package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type AttendanceLogRepository struct {
	pool *pgxpool.Pool
}

func NewAttendanceLogRepository(pool *pgxpool.Pool) *AttendanceLogRepository {
	return &AttendanceLogRepository{pool: pool}
}

// BulkInsert chèn hàng loạt attendance log, tận dụng UNIQUE(device_id, employee_code,
// check_time) tại DB để de-duplicate (ON CONFLICT DO NOTHING).
func (r *AttendanceLogRepository) BulkInsert(ctx context.Context, logs []entity.AttendanceLog) (int, error) {
	if len(logs) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	inserted := 0
	query := `
		INSERT INTO attendance_log
			(device_id, employee_code, check_time, check_type, verify_mode, raw_payload, synced_at, is_valid, invalid_reason, work_segment, overtime_request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (device_id, employee_code, check_time) DO NOTHING`

	for _, l := range logs {
		// Log mới luôn hợp lệ cho tới khi người dùng chạy tính công. Trạng thái
		// không hợp lệ chỉ được ghi khi có lý do cụ thể từ bộ xử lý ca làm việc.
		isValid := l.IsValid
		if l.InvalidReason == "" {
			isValid = true
		}
		workSegment := l.WorkSegment
		if workSegment == "" {
			workSegment = "regular"
		}
		tag, err := tx.Exec(ctx, query,
			nullableUUID(l.DeviceID), l.EmployeeCode, l.CheckTime, l.CheckType, l.VerifyMode, l.RawPayload, l.SyncedAt,
			isValid, l.InvalidReason, workSegment, l.OvertimeRequestID)
		if err != nil {
			return inserted, err
		}
		inserted += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return inserted, err
	}
	return inserted, nil
}

func (r *AttendanceLogRepository) Query(ctx context.Context, from, to time.Time, employeeCode, deviceID string) ([]entity.AttendanceLog, error) {
	query := `
		SELECT 
			al.id, 
			al.device_id, 
			al.employee_code, 
			al.check_time, 
			al.check_type, 
			al.verify_mode, 
			al.raw_payload, 
			al.synced_at,
			al.is_valid,
			al.invalid_reason,
			COALESCE(al.work_segment, 'regular'),
			al.overtime_request_id,
			COALESCE(e.full_name, ''),
			COALESCE(d.name, '')
		FROM attendance_log al
		LEFT JOIN employee e ON al.employee_code = e.employee_code
		LEFT JOIN device d ON al.device_id = d.id
		WHERE al.check_time BETWEEN $1 AND $2
		  AND ($3 = '' OR al.employee_code = $3)
		  AND ($4 = '' OR al.device_id::text = $4)
		ORDER BY al.check_time DESC`

	rows, err := r.pool.Query(ctx, query, from, to, employeeCode, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []entity.AttendanceLog
	for rows.Next() {
		var l entity.AttendanceLog
		var devID *string
		if err := rows.Scan(
			&l.ID, &devID, &l.EmployeeCode, &l.CheckTime,
			&l.CheckType, &l.VerifyMode, &l.RawPayload, &l.SyncedAt,
			&l.IsValid, &l.InvalidReason,
			&l.WorkSegment, &l.OvertimeRequestID,
			&l.EmployeeName, &l.DeviceName,
		); err != nil {
			return nil, err
		}
		if devID != nil {
			l.DeviceID = *devID
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (r *AttendanceLogRepository) UpdateValidity(ctx context.Context, id int64, isValid bool, reason string) error {
	workSegment := "regular"
	if !isValid {
		workSegment = "invalid"
	}
	return r.UpdateClassification(ctx, id, isValid, reason, workSegment, nil)
}

func (r *AttendanceLogRepository) UpdateClassification(ctx context.Context, id int64, isValid bool, reason, workSegment string, overtimeRequestID *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE attendance_log
		SET is_valid = $2,
		    invalid_reason = $3,
		    work_segment = $4,
		    overtime_request_id = $5
		WHERE id = $1`, id, isValid, reason, workSegment, overtimeRequestID)
	return err
}
