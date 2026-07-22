package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type LeaveRequestRepository struct {
	pool *pgxpool.Pool
}

func NewLeaveRequestRepository(pool *pgxpool.Pool) *LeaveRequestRepository {
	return &LeaveRequestRepository{pool: pool}
}

func (r *LeaveRequestRepository) Create(ctx context.Context, lr *entity.LeaveRequest) error {
	query := `
		INSERT INTO leave_request (employee_id, leave_type, start_date, end_date, reason)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at`
	return r.pool.QueryRow(ctx, query, lr.EmployeeID, lr.LeaveType, lr.StartDate, lr.EndDate, lr.Reason).
		Scan(&lr.ID, &lr.Status, &lr.CreatedAt)
}

func (r *LeaveRequestRepository) UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error {
	query := `
		UPDATE leave_request
		SET status = $1, approved_by = $2
		WHERE id = $3`
	var approvedByPtr *string
	if approvedBy != "" {
		approvedByPtr = &approvedBy
	}
	_, err := r.pool.Exec(ctx, query, status, approvedByPtr, id)
	return err
}

func (r *LeaveRequestRepository) List(ctx context.Context, employeeID string, status string) ([]entity.LeaveRequest, error) {
	query := `
		SELECT id, employee_id, leave_type, start_date, end_date, COALESCE(reason, ''), status, approved_by, created_at
		FROM leave_request
		WHERE ($1 = '' OR employee_id::text = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, employeeID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.LeaveRequest
	for rows.Next() {
		var lr entity.LeaveRequest
		var approvedBy *string
		if err := rows.Scan(&lr.ID, &lr.EmployeeID, &lr.LeaveType, &lr.StartDate, &lr.EndDate, &lr.Reason,
			&lr.Status, &approvedBy, &lr.CreatedAt); err != nil {
			return nil, err
		}
		lr.ApprovedBy = approvedBy
		list = append(list, lr)
	}
	return list, rows.Err()
}

func (r *LeaveRequestRepository) GetByID(ctx context.Context, id string) (*entity.LeaveRequest, error) {
	query := `
		SELECT id, employee_id, leave_type, start_date, end_date, COALESCE(reason, ''), status, approved_by, created_at
		FROM leave_request WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var lr entity.LeaveRequest
	var approvedBy *string
	if err := row.Scan(&lr.ID, &lr.EmployeeID, &lr.LeaveType, &lr.StartDate, &lr.EndDate, &lr.Reason,
		&lr.Status, &approvedBy, &lr.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	lr.ApprovedBy = approvedBy
	return &lr, nil
}

func (r *LeaveRequestRepository) CheckLeaveOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.LeaveRequest, error) {
	query := `
		SELECT id, employee_id, leave_type, start_date, end_date, COALESCE(reason, ''), status, approved_by, created_at
		FROM leave_request
		WHERE employee_id = $1
		  AND status = 'approved'
		  AND $2 BETWEEN start_date AND end_date
		LIMIT 1`
	row := r.pool.QueryRow(ctx, query, employeeID, date)
	var lr entity.LeaveRequest
	var approvedBy *string
	if err := row.Scan(&lr.ID, &lr.EmployeeID, &lr.LeaveType, &lr.StartDate, &lr.EndDate, &lr.Reason,
		&lr.Status, &approvedBy, &lr.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	lr.ApprovedBy = approvedBy
	return &lr, nil
}
