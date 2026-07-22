package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type ShiftSwapRequestRepository struct {
	pool *pgxpool.Pool
}

func NewShiftSwapRequestRepository(pool *pgxpool.Pool) *ShiftSwapRequestRepository {
	return &ShiftSwapRequestRepository{pool: pool}
}

func (r *ShiftSwapRequestRepository) Create(ctx context.Context, ssr *entity.ShiftSwapRequest) error {
	query := `
		INSERT INTO shift_swap_request (requesting_employee_id, target_employee_id, requesting_date, target_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`
	return r.pool.QueryRow(ctx, query, ssr.RequestingEmployeeID, ssr.TargetEmployeeID, ssr.RequestingDate, ssr.TargetDate).
		Scan(&ssr.ID, &ssr.Status, &ssr.CreatedAt)
}

func (r *ShiftSwapRequestRepository) List(ctx context.Context) ([]entity.ShiftSwapRequest, error) {
	query := `
		SELECT ssr.id, ssr.requesting_employee_id, ssr.target_employee_id, ssr.requesting_date, ssr.target_date, ssr.status, ssr.approved_by, ssr.created_at,
		       e1.full_name AS requesting_employee_name, e2.full_name AS target_employee_name
		FROM shift_swap_request ssr
		JOIN employee e1 ON ssr.requesting_employee_id = e1.id
		JOIN employee e2 ON ssr.target_employee_id = e2.id
		ORDER BY ssr.created_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.ShiftSwapRequest
	for rows.Next() {
		var ssr entity.ShiftSwapRequest
		var approvedBy *string
		if err := rows.Scan(
			&ssr.ID, &ssr.RequestingEmployeeID, &ssr.TargetEmployeeID, &ssr.RequestingDate, &ssr.TargetDate, &ssr.Status, &approvedBy, &ssr.CreatedAt,
			&ssr.RequestingEmployeeName, &ssr.TargetEmployeeName,
		); err != nil {
			return nil, err
		}
		ssr.ApprovedBy = approvedBy
		list = append(list, ssr)
	}
	return list, nil
}

func (r *ShiftSwapRequestRepository) GetByID(ctx context.Context, id string) (*entity.ShiftSwapRequest, error) {
	query := `
		SELECT ssr.id, ssr.requesting_employee_id, ssr.target_employee_id, ssr.requesting_date, ssr.target_date, ssr.status, ssr.approved_by, ssr.created_at,
		       e1.full_name AS requesting_employee_name, e2.full_name AS target_employee_name
		FROM shift_swap_request ssr
		JOIN employee e1 ON ssr.requesting_employee_id = e1.id
		JOIN employee e2 ON ssr.target_employee_id = e2.id
		WHERE ssr.id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var ssr entity.ShiftSwapRequest
	var approvedBy *string
	if err := row.Scan(
		&ssr.ID, &ssr.RequestingEmployeeID, &ssr.TargetEmployeeID, &ssr.RequestingDate, &ssr.TargetDate, &ssr.Status, &approvedBy, &ssr.CreatedAt,
		&ssr.RequestingEmployeeName, &ssr.TargetEmployeeName,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ssr.ApprovedBy = approvedBy
	return &ssr, nil
}

func (r *ShiftSwapRequestRepository) UpdateStatus(ctx context.Context, id string, status string, approvedBy string) error {
	_, err := r.pool.Exec(ctx, `UPDATE shift_swap_request SET status = $1, approved_by = $2 WHERE id = $3`, status, approvedBy, id)
	return err
}

func (r *ShiftSwapRequestRepository) GetApprovedSwapForEmployeeOnDate(ctx context.Context, employeeID string, date time.Time) (*entity.ShiftSwapRequest, error) {
	query := `
		SELECT id, requesting_employee_id, target_employee_id, requesting_date, target_date, status, approved_by, created_at
		FROM shift_swap_request
		WHERE status = 'approved'
		  AND (
		       (requesting_employee_id = $1 AND requesting_date = $2)
		       OR
		       (target_employee_id = $1 AND target_date = $2)
		  )
		LIMIT 1`
	row := r.pool.QueryRow(ctx, query, employeeID, date)
	var ssr entity.ShiftSwapRequest
	var approvedBy *string
	if err := row.Scan(&ssr.ID, &ssr.RequestingEmployeeID, &ssr.TargetEmployeeID, &ssr.RequestingDate, &ssr.TargetDate, &ssr.Status, &approvedBy, &ssr.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ssr.ApprovedBy = approvedBy
	return &ssr, nil
}
