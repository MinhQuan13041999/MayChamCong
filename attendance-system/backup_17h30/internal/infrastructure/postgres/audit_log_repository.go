package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"attendance-system/internal/domain/entity"
)

type AuditLogRepository struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

func (r *AuditLogRepository) Create(ctx context.Context, al *entity.AuditLog) error {
	query := `
		INSERT INTO audit_log (user_id, action, object_type, object_id, description, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, al.UserID, al.Action, al.ObjectType, al.ObjectID, al.Description, al.IPAddress).
		Scan(&al.ID, &al.CreatedAt)
}

func (r *AuditLogRepository) List(ctx context.Context, limit int, offset int) ([]entity.AuditLog, error) {
	query := `
		SELECT id, user_id, action, object_type, object_id, COALESCE(description, ''), COALESCE(ip_address, ''), created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []entity.AuditLog
	for rows.Next() {
		var al entity.AuditLog
		var userID *string
		if err := rows.Scan(&al.ID, &userID, &al.Action, &al.ObjectType, &al.ObjectID, &al.Description, &al.IPAddress, &al.CreatedAt); err != nil {
			return nil, err
		}
		al.UserID = userID
		list = append(list, al)
	}
	return list, rows.Err()
}
