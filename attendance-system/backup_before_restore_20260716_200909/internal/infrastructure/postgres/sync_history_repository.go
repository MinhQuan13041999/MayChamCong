package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type SyncHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewSyncHistoryRepository(pool *pgxpool.Pool) *SyncHistoryRepository {
	return &SyncHistoryRepository{pool: pool}
}

func (r *SyncHistoryRepository) Create(ctx context.Context, h *entity.SyncHistory) error {
	query := `
		INSERT INTO sync_history (device_id, sync_type, trigger_type, status, record_count, started_at)
		VALUES ($1, $2, $3, 'partial', 0, $4)
		RETURNING id`
	var id int64
	if err := r.pool.QueryRow(ctx, query, h.DeviceID, h.SyncType, h.TriggerType, h.StartedAt).Scan(&id); err != nil {
		return err
	}
	h.ID = id
	return nil
}

func (r *SyncHistoryRepository) Update(ctx context.Context, h *entity.SyncHistory) error {
	query := `
		UPDATE sync_history
		SET status = $1, record_count = $2, error_message = $3, finished_at = $4
		WHERE id = $5`
	_, err := r.pool.Exec(ctx, query, h.Status, h.RecordCount, h.ErrorMessage, h.FinishedAt, h.ID)
	return err
}

func (r *SyncHistoryRepository) List(ctx context.Context, deviceID, status string) ([]entity.SyncHistory, error) {
	query := `
		SELECT id, device_id, sync_type, trigger_type, status, record_count,
		       COALESCE(error_message, ''), started_at, finished_at
		FROM sync_history
		WHERE ($1 = '' OR device_id::text = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY started_at DESC`
	rows, err := r.pool.Query(ctx, query, deviceID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.SyncHistory
	for rows.Next() {
		var h entity.SyncHistory
		var devID *string
		var finished *time.Time
		if err := rows.Scan(&h.ID, &devID, &h.SyncType, &h.TriggerType, &h.Status,
			&h.RecordCount, &h.ErrorMessage, &h.StartedAt, &finished); err != nil {
			return nil, err
		}
		if devID != nil {
			h.DeviceID = *devID
		}
		if finished != nil {
			h.FinishedAt = *finished
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

func (r *SyncHistoryRepository) GetByID(ctx context.Context, id string) (*entity.SyncHistory, error) {
	query := `
		SELECT id, device_id, sync_type, trigger_type, status, record_count,
		       COALESCE(error_message, ''), started_at, finished_at
		FROM sync_history WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	var h entity.SyncHistory
	var devID *string
	var finished *time.Time
	if err := row.Scan(&h.ID, &devID, &h.SyncType, &h.TriggerType, &h.Status,
		&h.RecordCount, &h.ErrorMessage, &h.StartedAt, &finished); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if devID != nil {
		h.DeviceID = *devID
	}
	if finished != nil {
		h.FinishedAt = *finished
	}
	return &h, nil
}
