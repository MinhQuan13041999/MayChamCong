package main

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"attendance-system/internal/infrastructure/postgres"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		log.Fatalf("OLE init failed: %v", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		log.Fatalf("CreateObject failed: %v", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		log.Fatalf("QueryInterface failed: %v", err)
	}
	defer zkem.Release()

	ip := "192.168.11.151"
	port := int32(4370)

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, port)
	if err != nil || res == nil || res.Val == 0 {
		log.Fatalf("Connect failed: %v", err)
	}
	defer oleutil.CallMethod(zkem, "Disconnect")

	fmt.Println("Connected to device successfully!")

	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer pool.Close()

	empRepo := postgres.NewEmployeeRepository(pool)
	employees, err := empRepo.List(ctx)
	if err != nil {
		log.Fatalf("List employees failed: %v", err)
	}

	var dwMachineNumber int32 = 1
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, false)

	pushedCount := 0
	for _, e := range employees {
		// Call SSR_SetUserInfo
		// SSR_SetUserInfo(dwMachineNumber int, dwEnrollNumber string, Name string, Password string, Privilege int, Enabled bool)
		resSet, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo",
			dwMachineNumber, e.EmployeeCode, e.FullName, "", int32(0), true,
		)
		if err == nil && resSet != nil && resSet.Val != 0 {
			pushedCount++
			fmt.Printf("Pushed employee Code: '%s' | Name: '%s' to device\n", e.EmployeeCode, e.FullName)
		} else {
			fmt.Printf("Failed to push Code: '%s': %v\n", e.EmployeeCode, err)
		}
	}

	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, true)

	fmt.Printf("Successfully pushed %d / %d employees to device memory!\n", pushedCount, len(employees))
}
