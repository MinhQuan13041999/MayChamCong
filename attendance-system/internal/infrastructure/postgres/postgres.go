package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool khởi tạo connection pool tới PostgreSQL
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

// MigrateDatabase runs basic ALTER TABLE schema updates for enterprise features
func MigrateDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS email VARCHAR(100) UNIQUE;`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS phone VARCHAR(20);`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS zalo_user_id VARCHAR(100);`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS gender VARCHAR(10) CHECK (gender IN ('male', 'female', 'other'));`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS dob DATE;`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS join_date DATE DEFAULT CURRENT_DATE;`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS job_title VARCHAR(100);`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(255);`,

		`ALTER TABLE department ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES department(id) ON DELETE SET NULL;`,

		`ALTER TABLE device ADD COLUMN IF NOT EXISTS firmware_version VARCHAR(50);`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS last_online_at TIMESTAMPTZ;`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS mac_address VARCHAR(50) UNIQUE;`,

		`ALTER TABLE daily_attendance ADD COLUMN IF NOT EXISTS overtime_minutes INTEGER DEFAULT 0;`,
		`ALTER TABLE daily_attendance ADD COLUMN IF NOT EXISTS leave_id UUID REFERENCES leave_request(id) ON DELETE SET NULL;`,

		`ALTER TABLE shift ADD COLUMN IF NOT EXISTS color_code VARCHAR(7) DEFAULT '#4F46E5';`,
		`CREATE TABLE IF NOT EXISTS employee_device_mapping (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE, device_id UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE, device_user_id VARCHAR(100) NOT NULL, sync_status VARCHAR(20) NOT NULL DEFAULT 'pending', fingerprint_enrolled BOOLEAN NOT NULL DEFAULT false, fingerprint_enrolled_at TIMESTAMPTZ, last_synced_at TIMESTAMPTZ, last_error TEXT, UNIQUE (employee_id, device_id), UNIQUE (device_id, device_user_id));`,
		`INSERT INTO department (name) SELECT name FROM (VALUES
			('Hành chính - Nhân sự'),
			('Kỹ thuật / CNTT'),
			('Kinh doanh / Bán hàng'),
			('Marketing & Truyền thông'),
			('Kế toán / Tài chính'),
			('Nghiên cứu & Phát triển'),
			('Vận hành / Sản xuất'),
			('Chăm sóc khách hàng')
		) AS temp(name) WHERE NOT EXISTS (SELECT 1 FROM department WHERE LOWER(department.name) = LOWER(temp.name));`,
		`CREATE TABLE IF NOT EXISTS attendance_correction (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			corrected_time TIMESTAMPTZ NOT NULL,
			check_type VARCHAR(10) NOT NULL,
			reason TEXT,
			status VARCHAR(20) DEFAULT 'pending',
			approved_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		);`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS serial_number_adms VARCHAR(100);`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ;`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS adms_enabled BOOLEAN NOT NULL DEFAULT false;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uix_device_serial_adms ON device(serial_number_adms) WHERE serial_number_adms IS NOT NULL;`,
		`CREATE TABLE IF NOT EXISTS device_command_queue (
			id          BIGSERIAL PRIMARY KEY,
			device_id   UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE,
			command_id  BIGINT NOT NULL,
			command     TEXT NOT NULL,
			status      VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			sent_at     TIMESTAMPTZ,
			acked_at    TIMESTAMPTZ,
			UNIQUE(device_id, command_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_cmd_queue_device_status ON device_command_queue(device_id, status);`,
		`CREATE SEQUENCE IF NOT EXISTS device_command_id_seq START 1;`,
		`CREATE TABLE IF NOT EXISTS employee_fingerprint (
			id               BIGSERIAL PRIMARY KEY,
			employee_id      UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
			finger_index     INT  NOT NULL CHECK (finger_index BETWEEN 0 AND 9),
			template_data    TEXT NOT NULL,
			template_size    INT  NOT NULL DEFAULT 0,
			algo_version     VARCHAR(20) NOT NULL DEFAULT '10.0',
			source_device_id UUID REFERENCES device(id) ON DELETE SET NULL,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(employee_id, finger_index)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_fingerprint_employee ON employee_fingerprint(employee_id);`,
		`UPDATE employee SET fingerprint_enrolled = true
		 WHERE EXISTS (SELECT 1 FROM employee_fingerprint fp WHERE fp.employee_id = employee.id);`,
		`UPDATE employee_device_mapping edm
		 SET fingerprint_enrolled = true,
		     fingerprint_enrolled_at = COALESCE(edm.fingerprint_enrolled_at, now())
		 WHERE EXISTS (
		     SELECT 1 FROM employee_fingerprint fp
		     WHERE fp.employee_id = edm.employee_id
		       AND fp.source_device_id = edm.device_id
		 );`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS username VARCHAR(100) DEFAULT '';`,
		`ALTER TABLE device ADD COLUMN IF NOT EXISTS password VARCHAR(100) DEFAULT '';`,
		`ALTER TABLE attendance_log ADD COLUMN IF NOT EXISTS is_valid BOOLEAN NOT NULL DEFAULT true;`,
		`ALTER TABLE attendance_log ADD COLUMN IF NOT EXISTS invalid_reason VARCHAR(255) NOT NULL DEFAULT '';`,
		`ALTER TABLE attendance_log ADD COLUMN IF NOT EXISTS work_segment VARCHAR(20) NOT NULL DEFAULT 'regular';`,
		`ALTER TABLE attendance_log ADD COLUMN IF NOT EXISTS overtime_request_id UUID REFERENCES overtime_request(id) ON DELETE SET NULL;`,
		`ALTER TABLE daily_attendance ADD COLUMN IF NOT EXISTS regular_working_minutes INTEGER NOT NULL DEFAULT 0;`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_log_work_segment ON attendance_log(work_segment);`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_log_overtime_request ON attendance_log(overtime_request_id) WHERE overtime_request_id IS NOT NULL;`,
		`ALTER TABLE employee_shift ALTER COLUMN shift_id DROP NOT NULL;`,
		`ALTER TABLE employee_shift ADD COLUMN IF NOT EXISTS rotation_pattern_id UUID;`,
		`CREATE TABLE IF NOT EXISTS rotation_pattern (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL,
			pattern_sequence JSONB NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		);`,
		`CREATE TABLE IF NOT EXISTS shift_swap_request (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			requesting_employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
			target_employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
			requesting_date DATE NOT NULL,
			target_date DATE NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			approved_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		);`,
		`ALTER TABLE employee ADD COLUMN IF NOT EXISTS face_enrolled BOOLEAN NOT NULL DEFAULT false;`,
		`CREATE TABLE IF NOT EXISTS employee_face (
			id BIGSERIAL PRIMARY KEY,
			employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
			face_descriptor TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now(),
			CONSTRAINT unique_employee_face UNIQUE (employee_id)
		);`,
	}

	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("failed to run migration [%s]: %w", m, err)
		}
	}
	return nil
}
