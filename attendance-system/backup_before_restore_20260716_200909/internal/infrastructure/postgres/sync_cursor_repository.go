package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type SyncCursorRepository struct{ pool *pgxpool.Pool }

func NewSyncCursorRepository(pool *pgxpool.Pool) *SyncCursorRepository {
	return &SyncCursorRepository{pool: pool}
}
func (r *SyncCursorRepository) GetAttendanceCursor(ctx context.Context, deviceID string) (*time.Time, error) {
	var t time.Time
	err := r.pool.QueryRow(ctx, `SELECT attendance_cursor FROM sync_cursor WHERE device_id=$1`, deviceID).Scan(&t)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
func (r *SyncCursorRepository) SetAttendanceCursor(ctx context.Context, deviceID string, cursor time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO sync_cursor(device_id, attendance_cursor) VALUES ($1,$2) ON CONFLICT(device_id) DO UPDATE SET attendance_cursor=EXCLUDED.attendance_cursor, updated_at=now()`, deviceID, cursor)
	return err
}
