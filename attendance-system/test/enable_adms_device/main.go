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
		log.Fatalf("DB connect failed: %v", err)
	}
	defer pool.Close()

	// Update device table to enable ADMS
	query := `UPDATE device SET adms_enabled = true, device_type = 'adms' WHERE ip_address = '192.168.11.151' OR serial_number = '8116255100515'`
	tag, err := pool.Exec(ctx, query)
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	fmt.Printf("Updated device table: %d rows affected!\n", tag.RowsAffected())
}
