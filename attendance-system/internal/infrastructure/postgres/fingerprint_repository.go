package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type FingerprintRepository struct {
	pool *pgxpool.Pool
}

func NewFingerprintRepository(pool *pgxpool.Pool) *FingerprintRepository {
	return &FingerprintRepository{pool: pool}
}

func (r *FingerprintRepository) Upsert(ctx context.Context, fp *entity.EmployeeFingerprint) error {
	query := `
		INSERT INTO employee_fingerprint (employee_id, finger_index, template_data, template_size, algo_version, source_device_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (employee_id, finger_index) DO UPDATE
		SET template_data = EXCLUDED.template_data,
		    template_size = EXCLUDED.template_size,
		    algo_version = EXCLUDED.algo_version,
		    source_device_id = EXCLUDED.source_device_id,
		    updated_at = now()
		RETURNING id, created_at, updated_at`

	sourceDeviceIDParam := interface{}(nil)
	if fp.SourceDeviceID != "" {
		sourceDeviceIDParam = fp.SourceDeviceID
	}

	return r.pool.QueryRow(ctx, query,
		fp.EmployeeID, fp.FingerIndex, fp.TemplateData, fp.TemplateSize, fp.AlgoVersion, sourceDeviceIDParam,
	).Scan(&fp.ID, &fp.CreatedAt, &fp.UpdatedAt)
}

func (r *FingerprintRepository) ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeFingerprint, error) {
	query := `
		SELECT id, employee_id, finger_index, template_data, template_size, algo_version, COALESCE(source_device_id::text, ''), created_at, updated_at
		FROM employee_fingerprint
		WHERE employee_id = $1
		ORDER BY finger_index ASC`

	rows, err := r.pool.Query(ctx, query, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fps []entity.EmployeeFingerprint
	for rows.Next() {
		var f entity.EmployeeFingerprint
		if err := rows.Scan(&f.ID, &f.EmployeeID, &f.FingerIndex, &f.TemplateData, &f.TemplateSize, &f.AlgoVersion, &f.SourceDeviceID, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		fps = append(fps, f)
	}
	return fps, rows.Err()
}

func (r *FingerprintRepository) GetByEmployeeAndFinger(ctx context.Context, employeeID string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	query := `
		SELECT id, employee_id, finger_index, template_data, template_size, algo_version, COALESCE(source_device_id::text, ''), created_at, updated_at
		FROM employee_fingerprint
		WHERE employee_id = $1 AND finger_index = $2`

	row := r.pool.QueryRow(ctx, query, employeeID, fingerIndex)
	var f entity.EmployeeFingerprint
	if err := row.Scan(&f.ID, &f.EmployeeID, &f.FingerIndex, &f.TemplateData, &f.TemplateSize, &f.AlgoVersion, &f.SourceDeviceID, &f.CreatedAt, &f.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *FingerprintRepository) Delete(ctx context.Context, employeeID string, fingerIndex int) error {
	query := `DELETE FROM employee_fingerprint WHERE employee_id = $1 AND finger_index = $2`
	_, err := r.pool.Exec(ctx, query, employeeID, fingerIndex)
	return err
}
