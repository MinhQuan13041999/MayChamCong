//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://postgres:123456@127.0.0.1:5432/attendance_db?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect failed: %v\n", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT id, name, ip_address, serial_number, serial_number_adms, adms_enabled, status FROM device")
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("Devices in database:")
	for rows.Next() {
		var id, name, ip, sn, snAdms, status string
		var adms bool
		err = rows.Scan(&id, &name, &ip, &sn, &snAdms, &adms, &status)
		if err != nil {
			log.Fatalf("scan failed: %v\n", err)
		}
		fmt.Printf("ID: %s, Name: %s, IP: %s, SN: %s, SN_ADMS: %s, ADMS: %t, Status: %s\n", id, name, ip, sn, snAdms, adms, status)
	}
}
