package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type ShiftRepository struct {
	pool *pgxpool.Pool
}

func NewShiftRepository(pool *pgxpool.Pool) *ShiftRepository {
	return &ShiftRepository{pool: pool}
}

func (r *ShiftRepository) Create(ctx context.Context, s *entity.Shift) error {
	query := `
		INSERT INTO shift (name, start_time, end_time, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone, color_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, s.Name, s.StartTime, s.EndTime, s.BreakMinutes, s.LateGraceMinutes, s.EarlyGraceMinutes, s.MaxWorkingMinutes, s.Timezone, s.ColorCode).Scan(&s.ID, &s.CreatedAt)
}

func (r *ShiftRepository) GetByID(ctx context.Context, id string) (*entity.Shift, error) {
	query := `
		SELECT id, name, start_time::text, end_time::text, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone, COALESCE(color_code, ''), created_at
		FROM shift WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var s entity.Shift
	if err := row.Scan(&s.ID, &s.Name, &s.StartTime, &s.EndTime, &s.BreakMinutes, &s.LateGraceMinutes, &s.EarlyGraceMinutes, &s.MaxWorkingMinutes, &s.Timezone, &s.ColorCode, &s.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *ShiftRepository) List(ctx context.Context) ([]entity.Shift, error) {
	query := `
		SELECT id, name, start_time::text, end_time::text, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone, COALESCE(color_code, ''), created_at
		FROM shift ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.Shift
	for rows.Next() {
		var s entity.Shift
		if err := rows.Scan(&s.ID, &s.Name, &s.StartTime, &s.EndTime, &s.BreakMinutes, &s.LateGraceMinutes, &s.EarlyGraceMinutes, &s.MaxWorkingMinutes, &s.Timezone, &s.ColorCode, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *ShiftRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM shift WHERE id = $1`, id)
	return err
}
