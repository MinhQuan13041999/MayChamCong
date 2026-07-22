package main

import (
	"attendance-system/internal/infrastructure/postgres"
	"context"
	"fmt"
)

func main() {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	empRepo := postgres.NewEmployeeRepository(pool)
	mappingRepo := postgres.NewEmployeeDeviceMappingRepository(pool)

	emp, err := empRepo.GetByID(ctx, "c221c9a1-48e8-4f29-a353-064d8a9979bd")
	if err != nil {
		panic(err)
	}
	fmt.Printf("employee_enrolled=%v\n", emp.FingerprintEnrolled)

	m, err := mappingRepo.GetByEmployeeAndDevice(ctx, "c221c9a1-48e8-4f29-a353-064d8a9979bd", "dba701f3-fc74-4d79-8a43-5f989c29622d")
	if err != nil {
		panic(err)
	}
	fmt.Printf("mapping_exists=%v\n", m != nil)
	fmt.Printf("mapping_enrolled=%v\n", m != nil && m.FingerprintEnrolled)
	if m != nil {
		fmt.Printf("mapping_user=%s\n", m.DeviceUserID)
	}
}
