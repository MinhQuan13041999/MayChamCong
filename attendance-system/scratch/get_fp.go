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

	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM employee_fingerprint").Scan(&count)
	fmt.Printf("Total fingerprint templates: %d\n", count)

	rows, err := pool.Query(ctx, "SELECT employee_id, finger_index, template_size, template_data FROM employee_fingerprint LIMIT 5")
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var empID string
		var fingerIndex, size int
		var templateData string
		_ = rows.Scan(&empID, &fingerIndex, &size, &templateData)
		fmt.Printf("EmpID: %s | Index: %d | Size: %d | Template (first 30 chars): %s...\n",
			empID, fingerIndex, size, templateData[:min(30, len(templateData))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
