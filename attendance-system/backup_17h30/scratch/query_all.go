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

	fmt.Println("=== DEVICES ===")
	devRows, err := pool.Query(ctx, "SELECT id, name, serial_number, serial_number_adms, adms_enabled, status, last_heartbeat_at FROM device")
	if err == nil {
		for devRows.Next() {
			var id, name, sn string
			var snAdms, status *string
			var admsEnabled bool
			var lastHb *time.Time
			_ = devRows.Scan(&id, &name, &sn, &snAdms, &admsEnabled, &status, &lastHb)
			fmt.Printf("ID: %s | Name: %s | SN: %s | SN_ADMS: %v | ADMS: %t | Status: %v | LastHb: %v\n",
				id, name, sn, deref(snAdms), admsEnabled, deref(status), lastHb)
		}
		devRows.Close()
	}

	fmt.Println("\n=== LATEST SYNC HISTORY ===")
	histRows, err := pool.Query(ctx, "SELECT id, device_id, sync_type, trigger_type, status, record_count, error_message, started_at, finished_at FROM sync_history ORDER BY id DESC LIMIT 10")
	if err == nil {
		for histRows.Next() {
			var id int64
			var devID, syncType, triggerType, status string
			var recordCount int
			var errMsg *string
			var started, finished time.Time
			_ = histRows.Scan(&id, &devID, &syncType, &triggerType, &status, &recordCount, &errMsg, &started, &finished)
			fmt.Printf("ID: %d | DevID: %s | Type: %s | Trig: %s | Status: %s | Count: %d | Err: %v | Started: %s\n",
				id, devID, syncType, triggerType, status, recordCount, deref(errMsg), started.Format("15:04:05"))
		}
		histRows.Close()
	}

	fmt.Println("\n=== LATEST COMMANDS ===")
	cmdRows, err := pool.Query(ctx, "SELECT id, command_id, command, status, created_at, sent_at, acked_at FROM device_command_queue ORDER BY id DESC LIMIT 15")
	if err == nil {
		for cmdRows.Next() {
			var id, cmdID int64
			var command, status string
			var created time.Time
			var sent, acked *time.Time
			_ = cmdRows.Scan(&id, &cmdID, &command, &status, &created, &sent, &acked)
			fmt.Printf("ID: %d | CmdID: %d | Cmd: %s | Status: %s | Created: %s | Sent: %s | Acked: %s\n",
				id, cmdID, command, status, created.Format("15:04:05"), formatTime(sent), formatTime(acked))
		}
		cmdRows.Close()
	}
}

func deref(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format("15:04:05")
}
