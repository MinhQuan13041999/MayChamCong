//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PatternItem struct {
	ShiftID  *string `json:"shift_id"`
	Duration int     `json:"duration"`
}

func main() {
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	fmt.Println("=== START SEEDING SHIFT MANAGEMENT DATA ===")

	// 1. Ensure we have at least 2 employees to work with
	var emp1ID, emp1Name, emp2ID, emp2Name string

	err = pool.QueryRow(ctx, "SELECT id, full_name FROM employee ORDER BY created_at ASC LIMIT 1").Scan(&emp1ID, &emp1Name)
	if err != nil {
		// No employees exist, let's create two
		fmt.Println("No employees found. Creating test employees...")
		err = pool.QueryRow(ctx, `
			INSERT INTO employee (employee_code, full_name, card_no, status)
			VALUES ('EMP101', 'Nguyen Hoang Nam', 'CARDF101', 'active')
			RETURNING id, full_name`).Scan(&emp1ID, &emp1Name)
		if err != nil {
			log.Fatalf("Failed to create test employee 1: %v\n", err)
		}
		err = pool.QueryRow(ctx, `
			INSERT INTO employee (employee_code, full_name, card_no, status)
			VALUES ('EMP102', 'Tran Minh Thu', 'CARDF102', 'active')
			RETURNING id, full_name`).Scan(&emp2ID, &emp2Name)
		if err != nil {
			log.Fatalf("Failed to create test employee 2: %v\n", err)
		}
	} else {
		// We have at least 1, let's find or create a second one
		err = pool.QueryRow(ctx, "SELECT id, full_name FROM employee WHERE id != $1 ORDER BY created_at ASC LIMIT 1", emp1ID).Scan(&emp2ID, &emp2Name)
		if err != nil {
			fmt.Println("Creating second test employee...")
			err = pool.QueryRow(ctx, `
				INSERT INTO employee (employee_code, full_name, card_no, status)
				VALUES ('EMP102', 'Tran Minh Thu', 'CARDF102', 'active')
				RETURNING id, full_name`).Scan(&emp2ID, &emp2Name)
			if err != nil {
				log.Fatalf("Failed to create test employee 2: %v\n", err)
			}
		}
	}
	fmt.Printf("Using Employee 1: %s (%s)\n", emp1Name, emp1ID)
	fmt.Printf("Using Employee 2: %s (%s)\n", emp2Name, emp2ID)

	// Clean existing shift data to avoid duplicates
	_, _ = pool.Exec(ctx, "DELETE FROM shift_swap_request")
	_, _ = pool.Exec(ctx, "DELETE FROM employee_shift")
	_, _ = pool.Exec(ctx, "DELETE FROM rotation_pattern")
	_, _ = pool.Exec(ctx, "DELETE FROM shift")

	// 2. Create Shifts
	var sHanhChinhID, sSangID, sChieuID string

	fmt.Println("Creating shifts...")
	err = pool.QueryRow(ctx, `
		INSERT INTO shift (name, start_time, end_time, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone)
		VALUES ('Ca Hanh Chinh', '08:00', '17:00', 60, 15, 15, 480, 'Asia/Ho_Chi_Minh')
		RETURNING id`).Scan(&sHanhChinhID)
	if err != nil {
		log.Fatalf("Failed to create Ca Hanh Chinh: %v\n", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO shift (name, start_time, end_time, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone)
		VALUES ('Ca Sang', '06:00', '14:00', 30, 10, 10, 480, 'Asia/Ho_Chi_Minh')
		RETURNING id`).Scan(&sSangID)
	if err != nil {
		log.Fatalf("Failed to create Ca Sang: %v\n", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO shift (name, start_time, end_time, break_minutes, late_grace_minutes, early_grace_minutes, max_working_minutes, timezone)
		VALUES ('Ca Chieu', '14:00', '22:00', 30, 10, 10, 480, 'Asia/Ho_Chi_Minh')
		RETURNING id`).Scan(&sChieuID)
	if err != nil {
		log.Fatalf("Failed to create Ca Chieu: %v\n", err)
	}

	fmt.Printf("Created Shifts: Ca Hanh Chinh (%s), Ca Sang (%s), Ca Chieu (%s)\n", sHanhChinhID, sSangID, sChieuID)

	// 3. Create Rotation Pattern
	var rotID string
	fmt.Println("Creating rotation pattern...")

	seq := []PatternItem{
		{ShiftID: &sSangID, Duration: 2},
		{ShiftID: &sChieuID, Duration: 2},
		{ShiftID: nil, Duration: 2}, // OFF days
	}
	seqJSON, err := json.Marshal(seq)
	if err != nil {
		log.Fatalf("Failed to marshal sequence: %v\n", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO rotation_pattern (name, pattern_sequence)
		VALUES ('Chu ky xoay 2-2-2 (Sang-Chieu-Off)', $1)
		RETURNING id`, string(seqJSON)).Scan(&rotID)
	if err != nil {
		log.Fatalf("Failed to create rotation pattern: %v\n", err)
	}
	fmt.Printf("Created Rotation Pattern: %s\n", rotID)

	// 4. Assign Shifts to Employees
	fmt.Println("Assigning shifts...")
	
	// Employee 1 gets fixed shift (Ca Hanh Chinh)
	startDate1 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	_, err = pool.Exec(ctx, `
		INSERT INTO employee_shift (employee_id, shift_id, start_date)
		VALUES ($1, $2, $3)`, emp1ID, sHanhChinhID, startDate1)
	if err != nil {
		log.Fatalf("Failed to assign fixed shift to Employee 1: %v\n", err)
	}

	// Employee 2 gets rotation pattern
	startDate2 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	_, err = pool.Exec(ctx, `
		INSERT INTO employee_shift (employee_id, rotation_pattern_id, start_date)
		VALUES ($1, $2, $3)`, emp2ID, rotID, startDate2)
	if err != nil {
		log.Fatalf("Failed to assign rotation pattern to Employee 2: %v\n", err)
	}
	fmt.Println("Assigned shifts successfully.")

	// 5. Create Shift Swap Request
	fmt.Println("Creating shift swap request...")
	reqDate := time.Date(2026, 7, 15, 0, 0, 0, 0, time.Local)
	tarDate := time.Date(2026, 7, 16, 0, 0, 0, 0, time.Local)

	_, err = pool.Exec(ctx, `
		INSERT INTO shift_swap_request (requesting_employee_id, target_employee_id, requesting_date, target_date, status)
		VALUES ($1, $2, $3, $4, 'pending')`, emp1ID, emp2ID, reqDate, tarDate)
	if err != nil {
		log.Fatalf("Failed to create shift swap request: %v\n", err)
	}
	fmt.Println("Created shift swap request successfully.")

	fmt.Println("=== SEEDING COMPLETED SUCCESSFULLY ===")
}
