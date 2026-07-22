package main

import (
	"attendance-system/internal/domain/entity"
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

	repo := postgres.NewFingerprintRepository(pool)
	fp := &entity.EmployeeFingerprint{
		EmployeeID:     "c221c9a1-48e8-4f29-a353-064d8a9979bd",
		FingerIndex:    1,
		TemplateData:   "TMP_VERIFY",
		TemplateSize:   123,
		AlgoVersion:    "10.0",
		SourceDeviceID: "dba701f3-fc74-4d79-8a43-5f989c29622d",
	}
	if err := repo.Upsert(ctx, fp); err != nil {
		panic(err)
	}
	fmt.Printf("inserted id=%d created=%v updated=%v\n", fp.ID, fp.CreatedAt, fp.UpdatedAt)

	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM employee_fingerprint WHERE employee_id=$1 AND finger_index=$2", fp.EmployeeID, fp.FingerIndex).Scan(&count); err != nil {
		panic(err)
	}
	fmt.Printf("row_count=%d\n", count)
}
