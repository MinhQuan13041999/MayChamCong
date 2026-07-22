package main

import (
	"context"
	"fmt"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic(err)
	}
	// Đặt timeout kết nối ngắn
	config.ConnConfig.ConnectTimeout = 2 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// Thử query danh sách nhân viên
	var id, code, name string
	err = pool.QueryRow(ctx, "SELECT id, employee_code, full_name FROM employee LIMIT 1").Scan(&id, &code, &name)
	if err != nil {
		fmt.Printf("Error finding employee: %v\n", err)
		return
	}

	fmt.Printf("Found employee: %s (Code: %s, ID: %s)\n", name, code, id)

	// Thử thực hiện xoá trong một transaction và ROLLBACK
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)

	fmt.Println("Attempting to delete employee from 'employee' table...")
	_, err = tx.Exec(ctx, "DELETE FROM employee WHERE id = $1", id)
	if err != nil {
		fmt.Printf("Database DELETE failed: %v\n", err)
	} else {
		fmt.Println("Database DELETE successful (transaction rolled back)!")
	}
}
