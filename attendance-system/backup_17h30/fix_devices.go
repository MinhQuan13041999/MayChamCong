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

	// Thiết lập adms_enabled = true cho cả hai thiết bị để tránh background scheduler cố kéo SDK từ thiết bị offline
	tag1, err := pool.Exec(ctx, `UPDATE device SET adms_enabled = true WHERE ip_address = '192.168.1.101'`)
	if err != nil {
		log.Printf("Update old device failed: %v\n", err)
	} else {
		fmt.Printf("Đặt adms_enabled = true cho 192.168.1.101: %d row(s) updated\n", tag1.RowsAffected())
	}

	tag2, err := pool.Exec(ctx, `UPDATE device SET serial_number_adms = '8116255100515', serial_number = '8116255100515', adms_enabled = true, port = 8818 WHERE ip_address = '192.168.11.151'`)
	if err != nil {
		log.Printf("Update main device failed: %v\n", err)
	} else {
		fmt.Printf("Cập nhật SN thiết bị chính 192.168.11.151: %d row(s) updated\n", tag2.RowsAffected())
	}

	fmt.Println("Done!")
}
