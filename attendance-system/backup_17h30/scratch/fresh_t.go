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
		log.Fatalf("connect failed: %v\n", err)
	}
	defer pool.Close()

	deviceID := "dba701f3-fc74-4d79-8a43-5f989c29622d"

	// 1. Clear old queue
	_, err = pool.Exec(ctx, "DELETE FROM device_command_queue WHERE device_id = $1", deviceID)
	if err != nil {
		log.Fatalf("clear queue failed: %v\n", err)
	}
	fmt.Println("Cleared old queue.")

	// Get next command ID from seq
	var nextCmdID int64
	err = pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&nextCmdID)
	if err != nil {
		log.Fatalf("get nextval failed: %v\n", err)
	}

	// 2. Enqueue clean test command
	testCmd := "DATA UPDATE user Pin=777\tName=TestFresh\tPri=0\tPasswd=\tCard=\tGrp=1"
	_, err = pool.Exec(ctx, `
		INSERT INTO device_command_queue (device_id, command_id, command, status)
		VALUES ($1, $2, $3, 'pending')`, deviceID, nextCmdID, testCmd)
	if err != nil {
		log.Fatalf("enqueue failed: %v\n", err)
	}
	fmt.Printf("Enqueued command ID=%d: %s\n", nextCmdID, testCmd)

	// 3. Monitor status for 20 seconds
	fmt.Println("Monitoring queue status for 20 seconds...")
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		var status string
		var sentAt, ackedAt *time.Time
		err = pool.QueryRow(ctx, "SELECT status, sent_at, acked_at FROM device_command_queue WHERE command_id = $1 AND device_id = $2", nextCmdID, deviceID).Scan(&status, &sentAt, &ackedAt)
		if err != nil {
			fmt.Printf("Query error: %v\n", err)
			continue
		}
		fmt.Printf("[%s] Status: %s | Sent: %s | Acked: %s\n",
			time.Now().Format("15:04:05"), status, formatTime(sentAt), formatTime(ackedAt))
		if status == "ack" || status == "failed" {
			break
		}
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format("15:04:05")
}
