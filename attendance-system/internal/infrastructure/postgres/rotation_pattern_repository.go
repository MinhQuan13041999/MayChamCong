package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type RotationPatternRepository struct {
	pool *pgxpool.Pool
}

func NewRotationPatternRepository(pool *pgxpool.Pool) *RotationPatternRepository {
	return &RotationPatternRepository{pool: pool}
}

func (r *RotationPatternRepository) Create(ctx context.Context, rp *entity.RotationPattern) error {
	query := `
		INSERT INTO rotation_pattern (name, pattern_sequence)
		VALUES ($1, $2)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, rp.Name, rp.PatternSequence).Scan(&rp.ID, &rp.CreatedAt)
}

func (r *RotationPatternRepository) List(ctx context.Context) ([]entity.RotationPattern, error) {
	query := `
		SELECT id, name, pattern_sequence, created_at
		FROM rotation_pattern
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.RotationPattern
	for rows.Next() {
		var rp entity.RotationPattern
		if err := rows.Scan(&rp.ID, &rp.Name, &rp.PatternSequence, &rp.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, rp)
	}
	return list, nil
}

func (r *RotationPatternRepository) GetByID(ctx context.Context, id string) (*entity.RotationPattern, error) {
	query := `
		SELECT id, name, pattern_sequence, created_at
		FROM rotation_pattern
		WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var rp entity.RotationPattern
	if err := row.Scan(&rp.ID, &rp.Name, &rp.PatternSequence, &rp.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rp, nil
}

func (r *RotationPatternRepository) Update(ctx context.Context, rp *entity.RotationPattern) error {
	query := `
		UPDATE rotation_pattern
		SET name = $1, pattern_sequence = $2
		WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, rp.Name, rp.PatternSequence, rp.ID)
	return err
}

func (r *RotationPatternRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM rotation_pattern WHERE id = $1`, id)
	return err
}
