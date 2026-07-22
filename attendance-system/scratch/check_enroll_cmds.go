//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	query := `
		SELECT id, device_id, command_id, command, status, created_at, sent_at, acked_at
		FROM device_command_queue
		WHERE command LIKE '%ENROLL%'
		ORDER BY id DESC
		LIMIT 15`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("=== ENROLL COMMANDS ===")
	for rows.Next() {
		var id, cmdID int64
		var devID, command, status string
		var createdAt time.Time
		var sentAt, ackedAt *time.Time
		err = rows.Scan(&id, &devID, &cmdID, &command, &status, &createdAt, &sentAt, &ackedAt)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		sentStr := "never"
		if sentAt != nil {
			sentStr = sentAt.Format(time.RFC3339)
		}
		ackedStr := "never"
		if ackedAt != nil {
			ackedStr = ackedAt.Format(time.RFC3339)
		}
		fmt.Printf("ID: %d | CmdID: %d | Cmd: %s | Status: %s | Created: %s | Sent: %s | Acked: %s\n",
			id, cmdID, command, status, createdAt.Format(time.RFC3339), sentStr, ackedStr)
	}
}
