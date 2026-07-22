package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type EmployeeShiftRepository struct {
	pool *pgxpool.Pool
}

func NewEmployeeShiftRepository(pool *pgxpool.Pool) *EmployeeShiftRepository {
	return &EmployeeShiftRepository{pool: pool}
}

func (r *EmployeeShiftRepository) Create(ctx context.Context, es *entity.EmployeeShift) error {
	query := `
		INSERT INTO employee_shift (employee_id, shift_id, rotation_pattern_id, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, es.EmployeeID, es.ShiftID, es.RotationPatternID, es.StartDate, es.EndDate).Scan(&es.ID, &es.CreatedAt)
}

func (r *EmployeeShiftRepository) GetActiveShiftForEmployee(ctx context.Context, employeeID string, date time.Time) (*entity.EmployeeShift, error) {
	query := `
		SELECT id, employee_id, shift_id, rotation_pattern_id, start_date, end_date, created_at
		FROM employee_shift
		WHERE employee_id = $1
		  AND start_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
		ORDER BY start_date DESC LIMIT 1`
	row := r.pool.QueryRow(ctx, query, employeeID, date)
	var es entity.EmployeeShift
	var endDate *time.Time
	if err := row.Scan(&es.ID, &es.EmployeeID, &es.ShiftID, &es.RotationPatternID, &es.StartDate, &endDate, &es.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	es.EndDate = endDate
	return &es, nil
}

func (r *EmployeeShiftRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM employee_shift WHERE id = $1`, id)
	return err
}

func (r *EmployeeShiftRepository) List(ctx context.Context) ([]entity.EmployeeShift, error) {
	query := `
		SELECT id, employee_id, shift_id, rotation_pattern_id, start_date, end_date, created_at
		FROM employee_shift
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.EmployeeShift
	for rows.Next() {
		var es entity.EmployeeShift
		var endDate *time.Time
		if err := rows.Scan(&es.ID, &es.EmployeeID, &es.ShiftID, &es.RotationPatternID, &es.StartDate, &endDate, &es.CreatedAt); err != nil {
			return nil, err
		}
		es.EndDate = endDate
		list = append(list, es)
	}
	return list, nil
}
