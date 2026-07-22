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

	// Clear old queue
	_, err = pool.Exec(ctx, "DELETE FROM device_command_queue WHERE device_id = $1", deviceID)
	if err != nil {
		log.Fatalf("clear queue failed: %v\n", err)
	}
	fmt.Println("Cleared old queue.")

	formats := []string{
		"DATA UPDATE USERINFO PIN=771\tName=TestInfoPIN\tPri=0\tPasswd=\tCard=\tGrp=1",
		"DATA UPDATE USERINFO Pin=772\tName=TestInfoPin\tPri=0\tPasswd=\tCard=\tGrp=1",
		"DATA UPDATE USER PIN=773\tName=TestUserPIN\tPri=0\tPasswd=\tCard=\tGrp=1",
		"DATA UPDATE USER Pin=774\tName=TestUserPin\tPri=0\tPasswd=\tCard=\tGrp=1",
		"DATA UPDATE user PIN=775\tName=TestuserPIN\tPri=0\tPasswd=\tCard=\tGrp=1",
	}

	cmdIDs := make([]int64, len(formats))

	for i, format := range formats {
		var nextCmdID int64
		err = pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&nextCmdID)
		if err != nil {
			log.Fatalf("get nextval failed: %v\n", err)
		}
		cmdIDs[i] = nextCmdID

		_, err = pool.Exec(ctx, `
			INSERT INTO device_command_queue (device_id, command_id, command, status)
			VALUES ($1, $2, $3, 'pending')`, deviceID, nextCmdID, format)
		if err != nil {
			log.Fatalf("enqueue failed: %v\n", err)
		}
		fmt.Printf("Enqueued Format %d (ID=%d): %s\n", i+1, nextCmdID, format)
	}

	fmt.Println("\nMonitoring statuses for 30 seconds...")
	for step := 0; step < 30; step++ {
		time.Sleep(1 * time.Second)
		allDone := true
		fmt.Printf("[%s] State:\n", time.Now().Format("15:04:05"))
		for i, cmdID := range cmdIDs {
			var status string
			var sentAt, ackedAt *time.Time
			err = pool.QueryRow(ctx, "SELECT status, sent_at, acked_at FROM device_command_queue WHERE command_id = $1 AND device_id = $2", cmdID, deviceID).Scan(&status, &sentAt, &ackedAt)
			if err != nil {
				fmt.Printf("  Format %d: Query error: %v\n", i+1, err)
				continue
			}
			fmt.Printf("  Format %d (ID=%d): Status: %s | Sent: %s | Acked: %s\n",
				i+1, cmdID, status, formatTime(sentAt), formatTime(ackedAt))
			if status == "pending" || status == "sent" {
				allDone = false
			}
		}
		if allDone {
			fmt.Println("All commands processed.")
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
