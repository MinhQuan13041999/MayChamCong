package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"attendance-system/internal/infrastructure/postgres"
)

func main() {
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer pool.Close()

	empRepo := postgres.NewEmployeeRepository(pool)
	devRepo := postgres.NewDeviceRepository(pool)

	employees, err := empRepo.List(ctx)
	if err != nil {
		log.Fatalf("List employees failed: %v", err)
	}
	fmt.Printf("Total employees in DB: %d\n", len(employees))
	for i, e := range employees {
		fmt.Printf(" DB Emp #%d | ID: %s | Code: %s | Name: %s\n", i+1, e.ID, e.EmployeeCode, e.FullName)
	}

	devices, err := devRepo.List(ctx)
	if err != nil {
		log.Fatalf("List devices failed: %v", err)
	}
	fmt.Printf("Total devices in DB: %d\n", len(devices))
	for i, d := range devices {
		fmt.Printf(" DB Dev #%d | ID: %s | Name: %s | IP: %s\n", i+1, d.ID, d.Name, d.IPAddress)
	}
}
