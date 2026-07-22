package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type EmployeeDeviceMappingRepository struct{ pool *pgxpool.Pool }

func NewEmployeeDeviceMappingRepository(pool *pgxpool.Pool) *EmployeeDeviceMappingRepository {
	return &EmployeeDeviceMappingRepository{pool: pool}
}

func (r *EmployeeDeviceMappingRepository) Upsert(ctx context.Context, m *entity.EmployeeDeviceMapping) error {
	q := `INSERT INTO employee_device_mapping (employee_id, device_id, device_user_id, sync_status, fingerprint_enrolled, fingerprint_enrolled_at, last_synced_at, last_error)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	ON CONFLICT (employee_id, device_id) DO UPDATE SET device_user_id=EXCLUDED.device_user_id, sync_status=EXCLUDED.sync_status, fingerprint_enrolled=EXCLUDED.fingerprint_enrolled, fingerprint_enrolled_at=EXCLUDED.fingerprint_enrolled_at, last_synced_at=EXCLUDED.last_synced_at, last_error=EXCLUDED.last_error
	RETURNING id`
	return r.pool.QueryRow(ctx, q, m.EmployeeID, m.DeviceID, m.DeviceUserID, m.SyncStatus, m.FingerprintEnrolled, m.FingerprintAt, m.LastSyncedAt, m.LastError).Scan(&m.ID)
}

func (r *EmployeeDeviceMappingRepository) ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeDeviceMapping, error) {
	rows, err := r.pool.Query(ctx, `SELECT edm.id, edm.employee_id, edm.device_id, edm.device_user_id, edm.sync_status, edm.fingerprint_enrolled, edm.fingerprint_enrolled_at, edm.last_synced_at, COALESCE(edm.last_error,''), e.employee_code, e.full_name FROM employee_device_mapping edm JOIN employee e ON e.id=edm.employee_id WHERE edm.employee_id=$1 ORDER BY edm.device_id`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []entity.EmployeeDeviceMapping
	for rows.Next() {
		var m entity.EmployeeDeviceMapping
		if err := rows.Scan(&m.ID, &m.EmployeeID, &m.DeviceID, &m.DeviceUserID, &m.SyncStatus, &m.FingerprintEnrolled, &m.FingerprintAt, &m.LastSyncedAt, &m.LastError, &m.EmployeeCode, &m.EmployeeName); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *EmployeeDeviceMappingRepository) ListByDevice(ctx context.Context, deviceID string) ([]entity.EmployeeDeviceMapping, error) {
	rows, err := r.pool.Query(ctx, `SELECT edm.id, edm.employee_id, edm.device_id, edm.device_user_id, edm.sync_status, edm.fingerprint_enrolled, edm.fingerprint_enrolled_at, edm.last_synced_at, COALESCE(edm.last_error,''), e.employee_code, e.full_name FROM employee_device_mapping edm JOIN employee e ON e.id=edm.employee_id WHERE edm.device_id=$1 ORDER BY edm.device_user_id`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []entity.EmployeeDeviceMapping
	for rows.Next() {
		var m entity.EmployeeDeviceMapping
		if err := rows.Scan(&m.ID, &m.EmployeeID, &m.DeviceID, &m.DeviceUserID, &m.SyncStatus, &m.FingerprintEnrolled, &m.FingerprintAt, &m.LastSyncedAt, &m.LastError, &m.EmployeeCode, &m.EmployeeName); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *EmployeeDeviceMappingRepository) GetByEmployeeAndDevice(ctx context.Context, employeeID, deviceID string) (*entity.EmployeeDeviceMapping, error) {
	row := r.pool.QueryRow(ctx, `SELECT edm.id, edm.employee_id, edm.device_id, edm.device_user_id, edm.sync_status, edm.fingerprint_enrolled, edm.fingerprint_enrolled_at, edm.last_synced_at, COALESCE(edm.last_error,''), e.employee_code, e.full_name FROM employee_device_mapping edm JOIN employee e ON e.id=edm.employee_id WHERE edm.employee_id=$1 AND edm.device_id=$2`, employeeID, deviceID)
	var m entity.EmployeeDeviceMapping
	if err := row.Scan(&m.ID, &m.EmployeeID, &m.DeviceID, &m.DeviceUserID, &m.SyncStatus, &m.FingerprintEnrolled, &m.FingerprintAt, &m.LastSyncedAt, &m.LastError, &m.EmployeeCode, &m.EmployeeName); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *EmployeeDeviceMappingRepository) GetByDeviceUserID(ctx context.Context, deviceID, userID string) (*entity.EmployeeDeviceMapping, error) {
	mappings, err := r.pool.Query(ctx, `SELECT edm.id, edm.employee_id, edm.device_id, edm.device_user_id, edm.sync_status, edm.fingerprint_enrolled, edm.fingerprint_enrolled_at, edm.last_synced_at, COALESCE(edm.last_error,''), e.employee_code, e.full_name FROM employee_device_mapping edm JOIN employee e ON e.id=edm.employee_id WHERE edm.device_id=$1 AND edm.device_user_id=$2`, deviceID, userID)
	if err != nil {
		return nil, err
	}
	defer mappings.Close()
	if !mappings.Next() {
		return nil, nil
	}
	var m entity.EmployeeDeviceMapping
	if err := mappings.Scan(&m.ID, &m.EmployeeID, &m.DeviceID, &m.DeviceUserID, &m.SyncStatus, &m.FingerprintEnrolled, &m.FingerprintAt, &m.LastSyncedAt, &m.LastError, &m.EmployeeCode, &m.EmployeeName); err != nil {
		return nil, err
	}
	return &m, mappings.Err()
}

func (r *EmployeeDeviceMappingRepository) MarkFingerprintEnrolled(ctx context.Context, employeeID, deviceID string, at time.Time) error {
	result, err := r.pool.Exec(ctx, `UPDATE employee_device_mapping SET fingerprint_enrolled=true, fingerprint_enrolled_at=$1 WHERE employee_id=$2 AND device_id=$3`, at, employeeID, deviceID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
