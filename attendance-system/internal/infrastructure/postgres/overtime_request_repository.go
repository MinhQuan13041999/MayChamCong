package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type OvertimeRequestRepository struct {
	pool *pgxpool.Pool
}

func NewOvertimeRequestRepository(pool *pgxpool.Pool) *OvertimeRequestRepository {
	return &OvertimeRequestRepository{pool: pool}
}

func (r *OvertimeRequestRepository) Create(ctx context.Context, ot *entity.OvertimeRequest) error {
	query := `
		INSERT INTO overtime_request (employee_id, date, start_time, end_time)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`
	return r.pool.QueryRow(ctx, query, ot.EmployeeID, ot.Date, ot.StartTime, ot.EndTime).
		Scan(&ot.ID, &ot.Status, &ot.CreatedAt)
}

func (r *OvertimeRequestRepository) UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error {
	query := `
		UPDATE overtime_request
		SET status = $1, approved_by = $2
		WHERE id = $3`
	var approvedByPtr *string
	if approvedBy != "" {
		approvedByPtr = &approvedBy
	}
	_, err := r.pool.Exec(ctx, query, status, approvedByPtr, id)
	return err
}

func (r *OvertimeRequestRepository) List(ctx context.Context, employeeID string, status string) ([]entity.OvertimeRequest, error) {
	query := `
		SELECT id, employee_id, date, start_time::text, end_time::text, status, approved_by, created_at
		FROM overtime_request
		WHERE ($1 = '' OR employee_id::text = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, employeeID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.OvertimeRequest
	for rows.Next() {
		var ot entity.OvertimeRequest
		var approvedBy *string
		if err := rows.Scan(&ot.ID, &ot.EmployeeID, &ot.Date, &ot.StartTime, &ot.EndTime,
			&ot.Status, &approvedBy, &ot.CreatedAt); err != nil {
			return nil, err
		}
		ot.ApprovedBy = approvedBy
		list = append(list, ot)
	}
	return list, rows.Err()
}

func (r *OvertimeRequestRepository) GetByID(ctx context.Context, id string) (*entity.OvertimeRequest, error) {
	query := `
		SELECT id, employee_id, date, start_time::text, end_time::text, status, approved_by, created_at
		FROM overtime_request WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var ot entity.OvertimeRequest
	var approvedBy *string
	if err := row.Scan(&ot.ID, &ot.EmployeeID, &ot.Date, &ot.StartTime, &ot.EndTime,
		&ot.Status, &approvedBy, &ot.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ot.ApprovedBy = approvedBy
	return &ot, nil
}

func (r *OvertimeRequestRepository) GetApprovedOTOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.OvertimeRequest, error) {
	query := `
		SELECT id, employee_id, date, start_time::text, end_time::text, status, approved_by, created_at
		FROM overtime_request
		WHERE employee_id = $1
		  AND date = $2
		  AND status = 'approved'
		LIMIT 1`
	row := r.pool.QueryRow(ctx, query, employeeID, date)
	var ot entity.OvertimeRequest
	var approvedBy *string
	if err := row.Scan(&ot.ID, &ot.EmployeeID, &ot.Date, &ot.StartTime, &ot.EndTime,
		&ot.Status, &approvedBy, &ot.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ot.ApprovedBy = approvedBy
	return &ot, nil
}

func (r *OvertimeRequestRepository) ListApprovedOTOnDate(ctx context.Context, employeeID string, date time.Time) ([]entity.OvertimeRequest, error) {
	query := `
		SELECT id, employee_id, date, start_time::text, end_time::text, status, approved_by, created_at
		FROM overtime_request
		WHERE employee_id = $1
		  AND date = $2
		  AND status = 'approved'
		ORDER BY start_time ASC, created_at ASC`
	rows, err := r.pool.Query(ctx, query, employeeID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.OvertimeRequest
	for rows.Next() {
		var ot entity.OvertimeRequest
		var approvedBy *string
		if err := rows.Scan(&ot.ID, &ot.EmployeeID, &ot.Date, &ot.StartTime, &ot.EndTime,
			&ot.Status, &approvedBy, &ot.CreatedAt); err != nil {
			return nil, err
		}
		ot.ApprovedBy = approvedBy
		list = append(list, ot)
	}
	return list, rows.Err()
}
