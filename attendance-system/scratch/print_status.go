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

	rows, err := pool.Query(ctx, "SELECT command_id, command, status FROM device_command_queue WHERE command_id >= 710 ORDER BY command_id")
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var cmd, status string
		_ = rows.Scan(&id, &cmd, &status)
		cmdTrunc := cmd
		if len(cmdTrunc) > 40 {
			cmdTrunc = cmdTrunc[:40] + "..."
		}
		fmt.Printf("ID: %d | Status: %s | Command: %s\n", id, status, cmdTrunc)
	}
}
