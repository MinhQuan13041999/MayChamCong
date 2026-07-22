package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type DeviceCommandRepository struct {
	pool *pgxpool.Pool
}

func NewDeviceCommandRepository(pool *pgxpool.Pool) *DeviceCommandRepository {
	return &DeviceCommandRepository{pool: pool}
}

func (r *DeviceCommandRepository) Enqueue(ctx context.Context, deviceID string, command string) (*entity.DeviceCommandQueue, error) {
	// Lấy command_id tiếp theo từ sequence
	var cmdID int64
	err := r.pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&cmdID)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO device_command_queue (device_id, command_id, command, status, created_at)
		VALUES ($1, $2, $3, 'pending', now())
		RETURNING id, created_at`

	cmd := &entity.DeviceCommandQueue{
		DeviceID:  deviceID,
		CommandID: cmdID,
		Command:   command,
		Status:    "pending",
	}

	err = r.pool.QueryRow(ctx, query, deviceID, cmdID, command).Scan(&cmd.ID, &cmd.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Debug log for troubleshooting enqueue operations
	fmt.Printf("[CMD_QUEUE] Enqueued cmd id=%d device=%s len=%d preview=%q\n", cmdID, deviceID, len(command), func() string {
		if len(command) > 80 {
			return command[:80] + "..."
		}
		return command
	}())

	return cmd, nil
}

func (r *DeviceCommandRepository) GetPending(ctx context.Context, deviceID string) ([]entity.DeviceCommandQueue, error) {
	query := `
		SELECT id, device_id, command_id, command, status, created_at, sent_at, acked_at
		FROM device_command_queue
		WHERE device_id = $1 AND status = 'pending'
		ORDER BY command_id ASC`

	rows, err := r.pool.Query(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []entity.DeviceCommandQueue
	for rows.Next() {
		var c entity.DeviceCommandQueue
		if err := rows.Scan(&c.ID, &c.DeviceID, &c.CommandID, &c.Command, &c.Status, &c.CreatedAt, &c.SentAt, &c.AckedAt); err != nil {
			return nil, err
		}
		cmds = append(cmds, c)
	}
	return cmds, rows.Err()
}

func (r *DeviceCommandRepository) MarkSent(ctx context.Context, commandID int64) error {
	query := `
		UPDATE device_command_queue
		SET status = 'sent', sent_at = now()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, commandID)
	return err
}

func (r *DeviceCommandRepository) Ack(ctx context.Context, deviceID string, commandID int64) error {
	query := `
		UPDATE device_command_queue
		SET status = 'ack', acked_at = now()
		WHERE device_id = $1 AND command_id = $2`
	_, err := r.pool.Exec(ctx, query, deviceID, commandID)
	return err
}

func (r *DeviceCommandRepository) GetByDeviceIDAndCommandID(ctx context.Context, deviceID string, commandID int64) (*entity.DeviceCommandQueue, error) {
	query := `
		SELECT id, device_id, command_id, command, status, created_at, sent_at, acked_at
		FROM device_command_queue
		WHERE device_id = $1 AND command_id = $2
		LIMIT 1`

	row := r.pool.QueryRow(ctx, query, deviceID, commandID)
	var c entity.DeviceCommandQueue
	if err := row.Scan(&c.ID, &c.DeviceID, &c.CommandID, &c.Command, &c.Status, &c.CreatedAt, &c.SentAt, &c.AckedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *DeviceCommandRepository) MarkFailed(ctx context.Context, commandID int64) error {
	query := `
		UPDATE device_command_queue
		SET status = 'failed'
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, commandID)
	return err
}

func (r *DeviceCommandRepository) MarkFailedByDeviceCmdID(ctx context.Context, deviceID string, commandID int64) error {
	query := `
		UPDATE device_command_queue
		SET status = 'failed'
		WHERE device_id = $1 AND command_id = $2`
	_, err := r.pool.Exec(ctx, query, deviceID, commandID)
	return err
}

func (r *DeviceCommandRepository) CancelPendingByDevice(ctx context.Context, deviceID string) (int, error) {
	query := `
		UPDATE device_command_queue
		SET status = 'failed'
		WHERE device_id = $1 AND status = 'pending'`
	result, err := r.pool.Exec(ctx, query, deviceID)
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}
