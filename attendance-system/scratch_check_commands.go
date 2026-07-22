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

	fmt.Println("=== DEVICE COMMAND QUEUE ===")
	rows, err := pool.Query(ctx, "SELECT id, device_id, command_id, command, status, created_at, sent_at, acked_at FROM device_command_queue ORDER BY id DESC LIMIT 20")
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, commandID int64
		var devID, command, status string
		var created time.Time
		var sent, acked *time.Time
		err = rows.Scan(&id, &devID, &commandID, &command, &status, &created, &sent, &acked)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		var sentVal, ackedVal string
		if sent != nil { sentVal = sent.Format("2006-01-02 15:04:05") }
		if acked != nil { ackedVal = acked.Format("2006-01-02 15:04:05") }
		fmt.Printf("ID: %d | DevID: %s | CmdID: %d | Cmd: %s | Status: %s | Sent: %s | Acked: %s\n",
			id, devID, commandID, command, status, sentVal, ackedVal)
	}
}
