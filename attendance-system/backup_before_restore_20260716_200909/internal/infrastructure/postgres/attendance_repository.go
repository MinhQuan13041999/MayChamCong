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
			(device_id, employee_code, check_time, check_type, verify_mode, raw_payload, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (device_id, employee_code, check_time) DO NOTHING`

	for _, l := range logs {
		tag, err := tx.Exec(ctx, query,
			nullableUUID(l.DeviceID), l.EmployeeCode, l.CheckTime, l.CheckType, l.VerifyMode, l.RawPayload, l.SyncedAt)
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
