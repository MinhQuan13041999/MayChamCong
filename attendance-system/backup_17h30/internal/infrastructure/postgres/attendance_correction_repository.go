package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type AttendanceCorrectionRepository struct {
	pool *pgxpool.Pool
}

func NewAttendanceCorrectionRepository(pool *pgxpool.Pool) *AttendanceCorrectionRepository {
	return &AttendanceCorrectionRepository{pool: pool}
}

func (r *AttendanceCorrectionRepository) Create(ctx context.Context, ac *entity.AttendanceCorrection) error {
	query := `
		INSERT INTO attendance_correction (employee_id, date, corrected_time, check_type, reason)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at`
	return r.pool.QueryRow(ctx, query, ac.EmployeeID, ac.Date, ac.CorrectedTime, ac.CheckType, ac.Reason).
		Scan(&ac.ID, &ac.Status, &ac.CreatedAt)
}

func (r *AttendanceCorrectionRepository) UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error {
	query := `
		UPDATE attendance_correction
		SET status = $1, approved_by = $2
		WHERE id = $3`
	var approvedByPtr *string
	if approvedBy != "" {
		approvedByPtr = &approvedBy
	}
	_, err := r.pool.Exec(ctx, query, status, approvedByPtr, id)
	return err
}

func (r *AttendanceCorrectionRepository) List(ctx context.Context, employeeID string, status string) ([]entity.AttendanceCorrection, error) {
	query := `
		SELECT id, employee_id, date, corrected_time, check_type, COALESCE(reason, ''), status, approved_by, created_at
		FROM attendance_correction
		WHERE ($1 = '' OR employee_id::text = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, employeeID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.AttendanceCorrection
	for rows.Next() {
		var ac entity.AttendanceCorrection
		var approvedBy *string
		if err := rows.Scan(&ac.ID, &ac.EmployeeID, &ac.Date, &ac.CorrectedTime, &ac.CheckType, &ac.Reason,
			&ac.Status, &approvedBy, &ac.CreatedAt); err != nil {
			return nil, err
		}
		ac.ApprovedBy = approvedBy
		list = append(list, ac)
	}
	return list, rows.Err()
}

func (r *AttendanceCorrectionRepository) GetByID(ctx context.Context, id string) (*entity.AttendanceCorrection, error) {
	query := `
		SELECT id, employee_id, date, corrected_time, check_type, COALESCE(reason, ''), status, approved_by, created_at
		FROM attendance_correction WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var ac entity.AttendanceCorrection
	var approvedBy *string
	if err := row.Scan(&ac.ID, &ac.EmployeeID, &ac.Date, &ac.CorrectedTime, &ac.CheckType, &ac.Reason,
		&ac.Status, &approvedBy, &ac.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ac.ApprovedBy = approvedBy
	return &ac, nil
}
