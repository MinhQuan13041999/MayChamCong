package main

import (
	"attendance-system/internal/infrastructure/postgres"
	"context"
	"fmt"
	"time"
)

func main() {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT employee_id, finger_index, template_size, created_at FROM employee_fingerprint ORDER BY employee_id, finger_index")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fmt.Println("List of enrolled fingerprints:")
	fmt.Println("----------------------------------------------------------------------")
	fmt.Printf("%-38s | %-12s | %-13s | %-20s\n", "Employee ID", "Finger Index", "Template Size", "Created At")
	fmt.Println("----------------------------------------------------------------------")
	for rows.Next() {
		var empID string
		var fingerIndex int
		var size int
		var createdAt time.Time
		if err := rows.Scan(&empID, &fingerIndex, &size, &createdAt); err != nil {
			panic(err)
		}
		fmt.Printf("%-38s | %-12d | %-13d | %-20s\n", empID, fingerIndex, size, createdAt.Format("2006-01-02 15:04:05"))
	}
}
