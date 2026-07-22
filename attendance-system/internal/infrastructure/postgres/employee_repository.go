package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type EmployeeRepository struct {
	pool *pgxpool.Pool
}

func NewEmployeeRepository(pool *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{pool: pool}
}

func (r *EmployeeRepository) getOrCreateDepartmentByName(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", nil
	}

	var id string
	if len(name) == 36 {
		err := r.pool.QueryRow(ctx, `SELECT id FROM department WHERE id = $1`, name).Scan(&id)
		if err == nil {
			return id, nil
		}
	}

	err := r.pool.QueryRow(ctx, `SELECT id FROM department WHERE LOWER(name) = LOWER($1)`, name).Scan(&id)
	if err == nil {
		return id, nil
	}

	err = r.pool.QueryRow(ctx, `INSERT INTO department (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (r *EmployeeRepository) Create(ctx context.Context, e *entity.Employee) error {
	deptID, err := r.getOrCreateDepartmentByName(ctx, e.DepartmentID)
	if err != nil {
		return err
	}

	if e.JoinDate.IsZero() {
		e.JoinDate = time.Now()
	}

	query := `
		INSERT INTO employee (employee_code, full_name, department_id, card_no,
		                       fingerprint_enrolled, face_enrolled, status,
		                       email, phone, zalo_user_id, gender, dob, join_date, job_title, avatar_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query,
		e.EmployeeCode, e.FullName, nullableUUID(deptID), e.CardNo,
		e.FingerprintEnrolled, e.FaceEnrolled, e.Status,
		nullableStr(e.Email), nullableStr(e.Phone), nullableStr(e.ZaloUserID), nullableStr(e.Gender),
		e.Dob, e.JoinDate, nullableStr(e.JobTitle), nullableStr(e.AvatarURL),
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EmployeeRepository) Update(ctx context.Context, e *entity.Employee) error {
	deptID, err := r.getOrCreateDepartmentByName(ctx, e.DepartmentID)
	if err != nil {
		return err
	}

	if e.JoinDate.IsZero() {
		e.JoinDate = time.Now()
	}

	query := `
		UPDATE employee
		SET full_name = $1, department_id = $2, card_no = $3,
		    fingerprint_enrolled = $4, face_enrolled = $5, status = $6,
		    email = $7, phone = $8, zalo_user_id = $9, gender = $10, dob = $11, join_date = $12,
		    job_title = $13, avatar_url = $14, updated_at = now()
		WHERE id = $15`
	_, err = r.pool.Exec(ctx, query,
		e.FullName, nullableUUID(deptID), e.CardNo, e.FingerprintEnrolled, e.FaceEnrolled, e.Status,
		nullableStr(e.Email), nullableStr(e.Phone), nullableStr(e.ZaloUserID), nullableStr(e.Gender),
		e.Dob, e.JoinDate, nullableStr(e.JobTitle), nullableStr(e.AvatarURL), e.ID)
	return err
}

func (r *EmployeeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM employee WHERE id = $1`, id)
	return err
}

func (r *EmployeeRepository) DeleteAll(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM employee`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *EmployeeRepository) GetByID(ctx context.Context, id string) (*entity.Employee, error) {
	return r.scanOne(ctx, `
		SELECT e.id, e.employee_code, e.full_name, d.name AS department_id, e.card_no,
		       (e.fingerprint_enrolled OR EXISTS (SELECT 1 FROM employee_fingerprint fp WHERE fp.employee_id = e.id AND fp.template_data <> '')), e.face_enrolled, e.status, 
		       COALESCE(e.email, ''), COALESCE(e.phone, ''), COALESCE(e.zalo_user_id, ''), COALESCE(e.gender, ''),
		       e.dob, e.join_date, COALESCE(e.job_title, ''), COALESCE(e.avatar_url, ''),
		       e.created_at, e.updated_at
		FROM employee e
		LEFT JOIN department d ON e.department_id = d.id
		WHERE e.id = $1`, id)
}

func (r *EmployeeRepository) GetByCode(ctx context.Context, code string) (*entity.Employee, error) {
	return r.scanOne(ctx, `
		SELECT e.id, e.employee_code, e.full_name, d.name AS department_id, e.card_no,
		       (e.fingerprint_enrolled OR EXISTS (SELECT 1 FROM employee_fingerprint fp WHERE fp.employee_id = e.id AND fp.template_data <> '')), e.face_enrolled, e.status,
		       COALESCE(e.email, ''), COALESCE(e.phone, ''), COALESCE(e.zalo_user_id, ''), COALESCE(e.gender, ''),
		       e.dob, e.join_date, COALESCE(e.job_title, ''), COALESCE(e.avatar_url, ''),
		       e.created_at, e.updated_at
		FROM employee e
		LEFT JOIN department d ON e.department_id = d.id
		WHERE e.employee_code = $1`, code)
}

func (r *EmployeeRepository) scanOne(ctx context.Context, query string, arg any) (*entity.Employee, error) {
	row := r.pool.QueryRow(ctx, query, arg)
	var e entity.Employee
	var deptID *string
	if err := row.Scan(&e.ID, &e.EmployeeCode, &e.FullName, &deptID, &e.CardNo,
		&e.FingerprintEnrolled, &e.FaceEnrolled, &e.Status,
		&e.Email, &e.Phone, &e.ZaloUserID, &e.Gender, &e.Dob, &e.JoinDate, &e.JobTitle, &e.AvatarURL,
		&e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if deptID != nil {
		e.DepartmentID = *deptID
	}
	return &e, nil
}

func (r *EmployeeRepository) List(ctx context.Context) ([]entity.Employee, error) {
	return r.query(ctx, `
		SELECT e.id, e.employee_code, e.full_name, d.name AS department_id, e.card_no,
		       (e.fingerprint_enrolled OR EXISTS (SELECT 1 FROM employee_fingerprint fp WHERE fp.employee_id = e.id AND fp.template_data <> '')), e.face_enrolled, e.status,
		       COALESCE(e.email, ''), COALESCE(e.phone, ''), COALESCE(e.zalo_user_id, ''), COALESCE(e.gender, ''),
		       e.dob, e.join_date, COALESCE(e.job_title, ''), COALESCE(e.avatar_url, ''),
		       e.created_at, e.updated_at
		FROM employee e
		LEFT JOIN department d ON e.department_id = d.id
		ORDER BY e.created_at DESC`)
}

func (r *EmployeeRepository) ListActive(ctx context.Context) ([]entity.Employee, error) {
	return r.query(ctx, `
		SELECT e.id, e.employee_code, e.full_name, d.name AS department_id, e.card_no,
		       (e.fingerprint_enrolled OR EXISTS (SELECT 1 FROM employee_fingerprint fp WHERE fp.employee_id = e.id AND fp.template_data <> '')), e.face_enrolled, e.status,
		       COALESCE(e.email, ''), COALESCE(e.phone, ''), COALESCE(e.zalo_user_id, ''), COALESCE(e.gender, ''),
		       e.dob, e.join_date, COALESCE(e.job_title, ''), COALESCE(e.avatar_url, ''),
		       e.created_at, e.updated_at
		FROM employee e
		LEFT JOIN department d ON e.department_id = d.id
		WHERE e.status = 'active'
		ORDER BY e.created_at DESC`)
}

func (r *EmployeeRepository) query(ctx context.Context, query string) ([]entity.Employee, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []entity.Employee
	for rows.Next() {
		var e entity.Employee
		var deptID *string
		if err := rows.Scan(&e.ID, &e.EmployeeCode, &e.FullName, &deptID, &e.CardNo,
			&e.FingerprintEnrolled, &e.FaceEnrolled, &e.Status,
			&e.Email, &e.Phone, &e.ZaloUserID, &e.Gender, &e.Dob, &e.JoinDate, &e.JobTitle, &e.AvatarURL,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if deptID != nil {
			e.DepartmentID = *deptID
		}
		employees = append(employees, e)
	}
	return employees, rows.Err()
}
