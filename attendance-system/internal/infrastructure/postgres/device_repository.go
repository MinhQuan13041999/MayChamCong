package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type DeviceRepository struct {
	pool *pgxpool.Pool
}

func NewDeviceRepository(pool *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{pool: pool}
}

func (r *DeviceRepository) Create(ctx context.Context, d *entity.Device) error {
	query := `
		INSERT INTO device (name, device_type, ip_address, port, serial_number, serial_number_adms, adms_enabled, status, location,
		                    firmware_version, last_online_at, mac_address, username, password)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query,
		d.Name, d.DeviceType, d.IPAddress, d.Port, d.SerialNumber, nullableStr(d.SerialNumberADMS), d.ADMSEnabled, d.Status, d.Location,
		d.FirmwareVersion, d.LastOnlineAt, d.MacAddress, d.Username, d.Password,
	).Scan(&d.ID, &d.CreatedAt)
}

func (r *DeviceRepository) Update(ctx context.Context, d *entity.Device) error {
	query := `
		UPDATE device
		SET name = $1, device_type = $2, ip_address = $3, port = $4,
		    serial_number = $5, serial_number_adms = $6, adms_enabled = $7, location = $8, firmware_version = $9,
		    last_online_at = $10, mac_address = $11, username = $12, password = $13
		WHERE id = $14`
	_, err := r.pool.Exec(ctx, query,
		d.Name, d.DeviceType, d.IPAddress, d.Port, d.SerialNumber, nullableStr(d.SerialNumberADMS), d.ADMSEnabled, d.Location,
		d.FirmwareVersion, d.LastOnlineAt, d.MacAddress, d.Username, d.Password, d.ID)
	return err
}

func (r *DeviceRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Raw attendance and synchronization history are business records. Keep
	// them for reporting/audit purposes, but detach them from the device being
	// removed. Operational tables such as mappings, cursors and ADMS commands
	// already use ON DELETE CASCADE; fingerprint source_device_id uses SET NULL.
	if _, err := tx.Exec(ctx, `UPDATE attendance_log SET device_id = NULL WHERE device_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sync_history SET device_id = NULL WHERE device_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM device WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *DeviceRepository) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	query := `
		SELECT id, name, device_type, ip_address, port, serial_number, COALESCE(serial_number_adms, ''), adms_enabled, status,
		       last_checked_at, last_heartbeat_at, location, COALESCE(firmware_version, ''), last_online_at,
		       COALESCE(mac_address, ''), COALESCE(username, ''), COALESCE(password, ''), created_at
		FROM device WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var d entity.Device
	if err := row.Scan(&d.ID, &d.Name, &d.DeviceType, &d.IPAddress, &d.Port, &d.SerialNumber, &d.SerialNumberADMS, &d.ADMSEnabled,
		&d.Status, &d.LastCheckedAt, &d.LastHeartbeatAt, &d.Location, &d.FirmwareVersion, &d.LastOnlineAt,
		&d.MacAddress, &d.Username, &d.Password, &d.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) GetBySerialADMS(ctx context.Context, sn string) (*entity.Device, error) {
	query := `
		SELECT id, name, device_type, ip_address, port, serial_number, COALESCE(serial_number_adms, ''), adms_enabled, status,
		       last_checked_at, last_heartbeat_at, location, COALESCE(firmware_version, ''), last_online_at,
		       COALESCE(mac_address, ''), COALESCE(username, ''), COALESCE(password, ''), created_at
		FROM device WHERE serial_number_adms = $1`
	row := r.pool.QueryRow(ctx, query, sn)
	var d entity.Device
	if err := row.Scan(&d.ID, &d.Name, &d.DeviceType, &d.IPAddress, &d.Port, &d.SerialNumber, &d.SerialNumberADMS, &d.ADMSEnabled,
		&d.Status, &d.LastCheckedAt, &d.LastHeartbeatAt, &d.Location, &d.FirmwareVersion, &d.LastOnlineAt,
		&d.MacAddress, &d.Username, &d.Password, &d.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) List(ctx context.Context) ([]entity.Device, error) {
	query := `
		SELECT id, name, device_type, ip_address, port, serial_number, COALESCE(serial_number_adms, ''), adms_enabled, status,
		       last_checked_at, last_heartbeat_at, location, COALESCE(firmware_version, ''), last_online_at,
		       COALESCE(mac_address, ''), COALESCE(username, ''), COALESCE(password, ''), created_at
		FROM device ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []entity.Device
	for rows.Next() {
		var d entity.Device
		if err := rows.Scan(&d.ID, &d.Name, &d.DeviceType, &d.IPAddress, &d.Port, &d.SerialNumber, &d.SerialNumberADMS, &d.ADMSEnabled,
			&d.Status, &d.LastCheckedAt, &d.LastHeartbeatAt, &d.Location, &d.FirmwareVersion, &d.LastOnlineAt,
			&d.MacAddress, &d.Username, &d.Password, &d.CreatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (r *DeviceRepository) UpdateStatus(ctx context.Context, id string, status string, checkedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE device SET status = $1, last_checked_at = $2, last_online_at = CASE WHEN $1 = 'online' THEN $2 ELSE last_online_at END WHERE id = $3`,
		status, checkedAt, id)
	return err
}

func (r *DeviceRepository) UpdateHeartbeat(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE device SET last_heartbeat_at = $1, last_online_at = $1, status = 'online', last_checked_at = $1 WHERE id = $2`,
		at, id)
	return err
}
