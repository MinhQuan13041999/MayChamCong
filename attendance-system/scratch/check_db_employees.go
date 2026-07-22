package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// 1. In ra danh sách nhân viên hiện tại
	rows, err := pool.Query(ctx, "SELECT id, employee_code, full_name FROM employee LIMIT 10")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fmt.Println("--- Employees List ---")
	var firstEmpID string
	var firstEmpName string
	for rows.Next() {
		var id, code, name string
		if err := rows.Scan(&id, &code, &name); err != nil {
			panic(err)
		}
		fmt.Printf("ID: %s | Code: %s | Name: %s\n", id, code, name)
		if firstEmpID == "" {
			firstEmpID = id
			firstEmpName = name
		}
	}

	if firstEmpID == "" {
		fmt.Println("No employees found in database.")
		return
	}

	// 2. Thử DELETE (dùng Transaction, ROLLBACK để tránh thực sự xoá nếu chạy check)
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("\nTesting deletion of employee '%s' (ID: %s)...\n", firstEmpName, firstEmpID)
	_, err = tx.Exec(ctx, "DELETE FROM employee WHERE id = $1", firstEmpID)
	if err != nil {
		fmt.Printf("DELETE FAILED with error: %v\n", err)
	} else {
		fmt.Println("DELETE SUCCESSFUL within transaction (rolled back). No constraints violated.")
	}
}
