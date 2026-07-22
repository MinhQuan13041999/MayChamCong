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
	fpRepo := postgres.NewFingerprintRepository(pool)

	employees, err := empRepo.List(ctx)
	if err != nil {
		log.Fatalf("List employees failed: %v", err)
	}

	var dwMachineNumber int32 = 1
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, false)

	totalFPs := 0
	pushedFPs := 0

	for _, e := range employees {
		fps, fpErr := fpRepo.ListByEmployee(ctx, e.ID)
		if fpErr != nil || len(fps) == 0 {
			fmt.Printf("Employee Code: '%s' (%s) has 0 fingerprints in DB\n", e.EmployeeCode, e.FullName)
			continue
		}

		for _, fp := range fps {
			totalFPs++
			// Call SSR_SetUserTmpStr(dwMachineNumber int, dwEnrollNumber string, FingerIndex int, TmpData string)
			resFP, err := oleutil.CallMethod(zkem, "SSR_SetUserTmpStr",
				dwMachineNumber, e.EmployeeCode, int32(fp.FingerIndex), fp.TemplateData,
			)
			if err == nil && resFP != nil && resFP.Val != 0 {
				pushedFPs++
				fmt.Printf(" Pushed FP FingerIndex: %d for Code: '%s' (%s)\n", fp.FingerIndex, e.EmployeeCode, e.FullName)
			} else {
				fmt.Printf(" Failed to push FP FingerIndex: %d for Code: '%s': %v\n", fp.FingerIndex, e.EmployeeCode, err)
			}
		}
	}

	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, true)

	fmt.Printf("Summary: Pushed %d / %d fingerprint templates from DB to device!\n", pushedFPs, totalFPs)
}
