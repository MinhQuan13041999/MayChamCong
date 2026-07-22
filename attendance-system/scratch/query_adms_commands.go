package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("ATTENDANCE_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	}
	serial := os.Getenv("ADMS_SERIAL")
	if serial == "" {
		serial = "8116255100515"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer pool.Close()

	var deviceID string
	err = pool.QueryRow(ctx, `SELECT id FROM device WHERE serial_number_adms = $1 LIMIT 1`, serial).Scan(&deviceID)
	if err != nil {
		log.Fatalf("device lookup failed: %v", err)
	}
	fmt.Printf("device_id=%s\n", deviceID)

	var pendingCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM device_command_queue WHERE device_id=$1 AND status='pending'`, deviceID).Scan(&pendingCount)
	if err != nil {
		log.Fatalf("pending count query failed: %v", err)
	}
	fmt.Printf("pending_count=%d\n", pendingCount)

	var enrollCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM device_command_queue WHERE device_id=$1 AND command ILIKE '%ENROLL_FP%'`, deviceID).Scan(&enrollCount)
	if err != nil {
		log.Fatalf("enroll count query failed: %v", err)
	}
	fmt.Printf("enroll_fp_count=%d\n", enrollCount)

	rows, err := pool.Query(ctx, `SELECT id, command_id, command, status, created_at, sent_at, acked_at FROM device_command_queue WHERE device_id=$1 ORDER BY command_id DESC LIMIT 50`, deviceID)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Command queue:")
	var count int
	for rows.Next() {
		var id, cmdID int64
		var command, status string
		var createdAt, sentAt, ackedAt *time.Time
		if err := rows.Scan(&id, &cmdID, &command, &status, &createdAt, &sentAt, &ackedAt); err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("id=%d cmd_id=%d status=%s created=%v sent=%v ack=%v\n  %s\n", id, cmdID, status, createdAt, sentAt, ackedAt, command)
		count++
	}
	if count == 0 {
		fmt.Println("(no commands)")
	}
}
