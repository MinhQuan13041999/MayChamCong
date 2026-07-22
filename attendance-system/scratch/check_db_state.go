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
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	// Query employee information
	fmt.Println("=== EMPLOYEES IN DATABASE ===")
	rows, err := pool.Query(ctx, "SELECT id, employee_code, full_name, card_no FROM employee")
	if err != nil {
		log.Fatalf("Query employee failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, code, name, cardNo string
		err = rows.Scan(&id, &code, &name, &cardNo)
		if err != nil {
			log.Fatalf("Scan employee failed: %v\n", err)
		}
		fmt.Printf("ID: %s | Code: %s | Name: %s | Card: %s\n", id, code, name, cardNo)
	}
}
