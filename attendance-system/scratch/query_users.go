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

	rows, err := pool.Query(ctx, `SELECT u.id, u.username, u.password_hash, r.name FROM "user" u LEFT JOIN role r ON u.role_id = r.id`)
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("Users in database:")
	for rows.Next() {
		var id, username, pwHash string
		var roleName *string
		err := rows.Scan(&id, &username, &pwHash, &roleName)
		if err != nil {
			log.Fatalf("scan failed: %v\n", err)
		}
		rName := "NULL"
		if roleName != nil {
			rName = *roleName
		}
		fmt.Printf("ID: %s | Username: %s | Role: %s | Hash: %s\n", id, username, rName, pwHash)
	}
}
