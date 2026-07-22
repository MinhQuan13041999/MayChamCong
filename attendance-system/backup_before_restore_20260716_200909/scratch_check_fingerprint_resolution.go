package main

import (
	"context"
	"fmt"

	"attendance-system/internal/config"
	"attendance-system/internal/infrastructure/postgres"
)

func main() {
	cfg, err := config.Load(".")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	employeeRepo := postgres.NewEmployeeRepository(pool)
	mappingRepo := postgres.NewEmployeeDeviceMappingRepository(pool)

	candidates := []string{"24", "EMP24", "EMP024", "00024", "001", "EMP001"}
	for _, code := range candidates {
		emp, err := employeeRepo.GetByCode(ctx, code)
		fmt.Printf("GetByCode(%q) err=%v emp=%#v\n", code, err, emp)
	}

	m, err := mappingRepo.GetByDeviceUserID(ctx, "dba701f3-fc74-4d79-8a43-5f989c29622d", "24")
	fmt.Printf("mapping24 err=%v mapping=%#v\n", err, m)
}
