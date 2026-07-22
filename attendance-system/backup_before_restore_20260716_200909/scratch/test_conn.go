//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func isDeviceOnline(lastHeartbeatAt *time.Time, threshold time.Duration) bool {
	if lastHeartbeatAt == nil {
		return false
	}
	const layout = "2006-01-02 15:04:05"
	utc1, err1 := time.ParseInLocation(layout, lastHeartbeatAt.Format(layout), time.UTC)
	utc2, err2 := time.ParseInLocation(layout, time.Now().Format(layout), time.UTC)
	if err1 != nil || err2 != nil {
		diff := time.Since(*lastHeartbeatAt)
		if diff < 0 {
			diff = -diff
		}
		return diff <= threshold
	}
	diff := utc2.Unix() - utc1.Unix()
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(threshold.Seconds())
}

func main() {
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect failed: %v\n", err)
	}
	defer pool.Close()

	var lastHb *time.Time
	var admsEnabled bool
	var status string
	err = pool.QueryRow(ctx, "SELECT adms_enabled, last_heartbeat_at, status FROM device WHERE id = 'dba701f3-fc74-4d79-8a43-5f989c29622d'").Scan(&admsEnabled, &lastHb, &status)
	if err != nil {
		log.Fatalf("query failed: %v\n", err)
	}

	fmt.Printf("Device ADMS: %t, DB Status: %s, Last Heartbeat: %v\n", admsEnabled, status, lastHb)
	if lastHb != nil {
		fmt.Printf("Formatted Heartbeat: %s\n", lastHb.Format("2006-01-02 15:04:05"))
		fmt.Printf("Formatted Now:       %s\n", time.Now().Format("2006-01-02 15:04:05"))
		online := isDeviceOnline(lastHb, 10*time.Minute)
		fmt.Printf("isDeviceOnline(10m): %t\n", online)
	}
}
