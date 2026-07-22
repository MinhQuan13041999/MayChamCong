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

	// Query device information
	fmt.Println("=== DEVICES IN DATABASE ===")
	rows, err := pool.Query(ctx, "SELECT id, name, device_type, ip_address, serial_number_adms, adms_enabled, last_heartbeat_at, status FROM device")
	if err != nil {
		log.Fatalf("Query device failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, devType, ip string
		var snAdms *string
		var admsEnabled bool
		var lastHb *time.Time
		var status string
		err = rows.Scan(&id, &name, &devType, &ip, &snAdms, &admsEnabled, &lastHb, &status)
		if err != nil {
			log.Fatalf("Scan device failed: %v\n", err)
		}
		hbStr := "never"
		if lastHb != nil {
			hbStr = lastHb.Format(time.RFC3339)
		}
		snStr := "NULL"
		if snAdms != nil {
			snStr = *snAdms
		}
		fmt.Printf("ID: %s | Name: %s | Type: %s | IP: %s | ADMS SN: %s | ADMS Enabled: %v | Last Heartbeat: %s | Status: %s\n",
			id, name, devType, ip, snStr, admsEnabled, hbStr, status)
	}
}
