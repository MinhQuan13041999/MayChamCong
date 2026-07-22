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

	dummyTemplate := "dGVzdF90ZW1wbGF0ZV9kYXRhX2hlcmVfd2hpY2hfaXNfYmFzZTY0" // "test_template_data_here_which_is_base64"

	formats := []string{
		// Format 1: FINGERTEMPLATE (all caps) + PIN + Template
		fmt.Sprintf("DATA UPDATE FINGERTEMPLATE PIN=771\tFingerID=1\tSize=%d\tVal=1\tTemplate=%s", len(dummyTemplate), dummyTemplate),
		// Format 2: fingertemplate (lowercase) + PIN + Template
		fmt.Sprintf("DATA UPDATE fingertemplate PIN=771\tFingerID=1\tSize=%d\tVal=1\tTemplate=%s", len(dummyTemplate), dummyTemplate),
		// Format 3: templatev10 (lowercase) + PIN + Template
		fmt.Sprintf("DATA UPDATE templatev10 PIN=771\tFingerID=1\tSize=%d\tVal=1\tTemplate=%s", len(dummyTemplate), dummyTemplate),
		// Format 4: templatev10 (lowercase) + PIN + tmp
		fmt.Sprintf("DATA UPDATE templatev10 PIN=771\tFingerID=1\tSize=%d\tVal=1\ttmp=%s", len(dummyTemplate), dummyTemplate),
		// Format 5: BIODATA (all caps) + PIN + tmp
		fmt.Sprintf("DATA UPDATE BIODATA PIN=771\tFingerID=1\tSize=%d\tVal=1\tTemplate=%s", len(dummyTemplate), dummyTemplate),
		// Format 6: fingertemplate (lowercase) + PIN + tmp (instead of Template)
		fmt.Sprintf("DATA UPDATE fingertemplate PIN=771\tFingerID=1\tSize=%d\tVal=1\ttmp=%s", len(dummyTemplate), dummyTemplate),
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
		fmt.Printf("Enqueued Fingerprint Format %d (ID=%d)\n", i+1, nextCmdID)
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
