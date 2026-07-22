//go:build ignore

// This maintenance utility is run explicitly with `go run clean_db_devices.go`.
// Keeping it out of the root package prevents it from colliding with other
// standalone utilities during `go test ./...`.
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

	// 1. Delete all devices NOT having IP '192.168.11.151'
	// Skip delete to avoid FK constraint error
	fmt.Printf("Skipped delete to avoid FK violations\n")

	// 2. Check if ZKTeco Phong Chinh exists
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM device WHERE ip_address = '192.168.11.151'").Scan(&count)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}

	if count == 0 {
		// Insert ZKTeco Phong Chinh
		_, err = pool.Exec(ctx, `
			INSERT INTO device (id, name, device_type, ip_address, port, serial_number, serial_number_adms, adms_enabled, location, status, firmware_version, mac_address)
			VALUES (gen_random_uuid(), 'ZKTeco Phong Chinh', 'zkteco', '192.168.11.151', 8818, '8116255100515', '8116255100515', true, 'Văn phòng chính', 'online', '1.0.0', '00:17:17:17:17:17')
		`)
		if err != nil {
			log.Fatalf("Insert failed: %v\n", err)
		}
		fmt.Println("Inserted ZKTeco Phong Chinh device.")
	} else {
		// Update it to correct values
		_, err = pool.Exec(ctx, `
			UPDATE device 
			SET name = 'ZKTeco Phong Chinh', device_type = 'zkteco', port = 8818, serial_number = '8116255100515', serial_number_adms = '8116255100515', adms_enabled = true, status = 'online'
			WHERE ip_address = '192.168.11.151'
		`)
		if err != nil {
			log.Fatalf("Update failed: %v\n", err)
		}
		fmt.Println("Updated existing ZKTeco Phong Chinh device.")
	}
}
