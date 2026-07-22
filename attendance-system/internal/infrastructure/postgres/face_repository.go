package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type FaceRepository struct {
	pool *pgxpool.Pool
}

func NewFaceRepository(pool *pgxpool.Pool) *FaceRepository {
	return &FaceRepository{pool: pool}
}

func (r *FaceRepository) Upsert(ctx context.Context, employeeID string, faceDescriptor string) error {
	query := `
		INSERT INTO employee_face (employee_id, face_descriptor, created_at)
		VALUES ($1, $2, now())
		ON CONFLICT (employee_id) DO UPDATE
		SET face_descriptor = EXCLUDED.face_descriptor`
	_, err := r.pool.Exec(ctx, query, employeeID, faceDescriptor)
	return err
}

func (r *FaceRepository) GetByEmployee(ctx context.Context, employeeID string) (*entity.EmployeeFace, error) {
	query := `SELECT id, employee_id, face_descriptor, created_at FROM employee_face WHERE employee_id = $1`
	row := r.pool.QueryRow(ctx, query, employeeID)
	var f entity.EmployeeFace
	if err := row.Scan(&f.ID, &f.EmployeeID, &f.FaceDescriptor, &f.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *FaceRepository) ListAll(ctx context.Context) ([]entity.EmployeeFace, error) {
	query := `SELECT id, employee_id, face_descriptor, created_at FROM employee_face`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var faces []entity.EmployeeFace
	for rows.Next() {
		var f entity.EmployeeFace
		if err := rows.Scan(&f.ID, &f.EmployeeID, &f.FaceDescriptor, &f.CreatedAt); err != nil {
			return nil, err
		}
		faces = append(faces, f)
	}
	return faces, rows.Err()
}

func (r *FaceRepository) Delete(ctx context.Context, employeeID string) error {
	query := `DELETE FROM employee_face WHERE employee_id = $1`
	_, err := r.pool.Exec(ctx, query, employeeID)
	return err
}
