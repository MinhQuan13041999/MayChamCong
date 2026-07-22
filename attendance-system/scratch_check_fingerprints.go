package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	// Check total fingerprints
	var totalFP int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM employee_fingerprint").Scan(&totalFP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to count fingerprints: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Total fingerprints in database: %d\n", totalFP)

	// Check fingerprints by employee
	rows, err := conn.Query(ctx, `
		SELECT employee_id, COUNT(*) as fp_count
		FROM employee_fingerprint
		GROUP BY employee_id
		ORDER BY employee_id
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query fingerprints by employee: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Println("\nFingerprints by employee:")
	for rows.Next() {
		var empID string
		var count int
		if err := rows.Scan(&empID, &count); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			continue
		}
		fmt.Printf("  %s: %d fingerprints\n", empID, count)
	}

	// Check employees
	var totalEmps int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM employee WHERE is_active = true").Scan(&totalEmps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to count employees: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nTotal active employees: %d\n", totalEmps)

	// Check ADMS devices
	var totalADMS int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM device WHERE adms_enabled = true").Scan(&totalADMS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to count ADMS devices: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Total ADMS devices: %d\n", totalADMS)

	// Check command queue
	var queueCount int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM device_command_queue WHERE status = 'pending'").Scan(&queueCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to count pending commands: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pending commands in queue: %d\n", queueCount)
}
