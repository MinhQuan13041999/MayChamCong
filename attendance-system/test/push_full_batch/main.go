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

	const dwMachineNumber int32 = 1

	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, false)
	// Start batch update mode
	oleutil.CallMethod(zkem, "BatchUpdate", dwMachineNumber, int32(1))

	userCount := 0
	fpCount := 0

	for _, e := range employees {
		// Set User Info
		resUser, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo",
			dwMachineNumber, e.EmployeeCode, e.FullName, "", int32(0), true,
		)
		if err == nil && resUser != nil && resUser.Val != 0 {
			userCount++
			fmt.Printf(" [User] Pushed Code: '%s' | Name: '%s'\n", e.EmployeeCode, e.FullName)
		} else {
			fmt.Printf(" [User Failed] Code: '%s': %v\n", e.EmployeeCode, err)
		}

		// Set Fingerprints
		fps, _ := fpRepo.ListByEmployee(ctx, e.ID)
		for _, fp := range fps {
			resFP, err := oleutil.CallMethod(zkem, "SSR_SetUserTmpStr",
				dwMachineNumber, e.EmployeeCode, int32(fp.FingerIndex), fp.TemplateData,
			)
			if err == nil && resFP != nil && resFP.Val != 0 {
				fpCount++
				fmt.Printf("   [Fingerprint] Pushed Index %d for Code '%s'\n", fp.FingerIndex, e.EmployeeCode)
			} else {
				fmt.Printf("   [Fingerprint Failed] Index %d for Code '%s': %v\n", fp.FingerIndex, e.EmployeeCode, err)
			}
		}
	}

	// Commit batch update mode to hardware flash memory
	oleutil.CallMethod(zkem, "BatchUpdate", dwMachineNumber, int32(0))
	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, true)

	fmt.Printf("Commit complete! Pushed Users: %d, Fingerprints: %d\n", userCount, fpCount)

	// Verify Device Status after commit
	var vStatus = ole.NewVariant(ole.VT_I4, 0)
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(2), &vStatus)
	uStat := vStatus.Value()
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(3), &vStatus)
	fStat := vStatus.Value()

	fmt.Printf("Hardware Verified Status -> Users in Flash: %v | Fingerprints in Flash: %v\n", uStat, fStat)
}
