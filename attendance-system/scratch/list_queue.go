//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

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

	rows, err := pool.Query(ctx, `SELECT id, command_id, command, status FROM device_command_queue ORDER BY id DESC LIMIT 15`)
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("Queue:")
	for rows.Next() {
		var id, cmdID int64
		var command, status string
		err := rows.Scan(&id, &cmdID, &command, &status)
		if err != nil {
			log.Fatalf("scan failed: %v\n", err)
		}
		fmt.Printf("ID: %d | CmdID: %d | Cmd: %s | Status: %s\n", id, cmdID, command, status)
	}
}
