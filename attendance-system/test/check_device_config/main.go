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

	devRepo := postgres.NewDeviceRepository(pool)
	devices, err := devRepo.List(ctx)
	if err != nil {
		log.Fatalf("List devices failed: %v", err)
	}

	for i, d := range devices {
		fmt.Printf("Device #%d:\n", i+1)
		fmt.Printf("  ID: %s\n", d.ID)
		fmt.Printf("  Name: %s\n", d.Name)
		fmt.Printf("  Type: %s\n", d.DeviceType)
		fmt.Printf("  IP: %s | Port: %d\n", d.IPAddress, d.Port)
		fmt.Printf("  Serial (Standalone): '%s'\n", d.SerialNumber)
		fmt.Printf("  Serial (ADMS): '%s'\n", d.SerialNumberADMS)
		fmt.Printf("  ADMS Enabled: %v\n", d.ADMSEnabled)
		if d.LastHeartbeatAt != nil {
			fmt.Printf("  Last Heartbeat: %s\n", d.LastHeartbeatAt.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("  Last Heartbeat: <nil>\n")
		}
	}
}
