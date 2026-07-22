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
		log.Fatalf("connect failed: %v\n", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `SELECT id, name, device_type, ip_address, port, serial_number, serial_number_adms, adms_enabled, last_heartbeat_at, status FROM device`)
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("Devices in database:")
	for rows.Next() {
		var id, name, deviceType, ipAddress string
		var port int
		var serialNumber, serialNumberAdms, status *string
		var admsEnabled bool
		var lastHeartbeatAt *time.Time
		err := rows.Scan(&id, &name, &deviceType, &ipAddress, &port, &serialNumber, &serialNumberAdms, &admsEnabled, &lastHeartbeatAt, &status)
		if err != nil {
			log.Fatalf("scan failed: %v\n", err)
		}
		var heartbeatStr string
		if lastHeartbeatAt != nil {
			heartbeatStr = lastHeartbeatAt.Format("2006-01-02 15:04:05")
		} else {
			heartbeatStr = "NULL"
		}
		sn := "NULL"
		if serialNumber != nil {
			sn = *serialNumber
		}
		snAdms := "NULL"
		if serialNumberAdms != nil {
			snAdms = *serialNumberAdms
		}
		st := "NULL"
		if status != nil {
			st = *status
		}
		fmt.Printf("ID: %s, Name: %s, Type: %s, IP: %s, Port: %d, SN: %s, SN_ADMS: %s, ADMS: %t, Heartbeat: %s, Status: %s\n",
			id, name, deviceType, ipAddress, port, sn, snAdms, admsEnabled, heartbeatStr, st)
	}
}
