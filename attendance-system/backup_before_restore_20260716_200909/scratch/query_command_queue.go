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
		log.Fatalf("connect failed: %v", err)
	}
	defer pool.Close()

	deviceID := "dba701f3-fc74-4d79-8a43-5f989c29622d"
	rows, err := pool.Query(ctx, `SELECT id, command_id, command, status, sent_at, acked_at FROM device_command_queue WHERE device_id=$1 ORDER BY command_id DESC LIMIT 30`, deviceID)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Command Queue for device:")
	for rows.Next() {
		var id, cmdID int64
		var command, status string
		var sentAt, ackedAt *time.Time
		if err := rows.Scan(&id, &cmdID, &command, &status, &sentAt, &ackedAt); err != nil {
			log.Fatalf("scan failed: %v", err)
		}
		fmt.Printf("ID:%d CmdID:%d Status:%s Sent:%v Ack:%v\n  Cmd: %s\n", id, cmdID, status, sentAt, ackedAt, command)
	}

	var mappingRows int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM employee_device_mapping WHERE device_id=$1`, deviceID).Scan(&mappingRows)
	if err != nil {
		log.Fatalf("mapping count failed: %v", err)
	}
	fmt.Printf("\nEmployee mappings on device: %d\n", mappingRows)
}
