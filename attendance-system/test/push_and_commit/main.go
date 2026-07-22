package main

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 1. Fetch Employees and Fingerprints from Postgres
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer pool.Close()

	type empInfo struct {
		code string
		name string
	}
	type fpInfo struct {
		empCode   string
		fingerIdx int
		template  string
	}

	rowsE, err := pool.Query(ctx, `SELECT employee_code, full_name FROM employee WHERE status = 'active'`)
	if err != nil {
		log.Fatalf("Query employees failed: %v", err)
	}
	var employees []empInfo
	for rowsE.Next() {
		var e empInfo
		_ = rowsE.Scan(&e.code, &e.name)
		employees = append(employees, e)
	}
	rowsE.Close()

	rowsF, err := pool.Query(ctx, `
		SELECT e.employee_code, ef.finger_index, ef.template_data
		FROM employee e
		JOIN employee_fingerprint ef ON ef.employee_id = e.id
		WHERE ef.template_data != ''
	`)
	if err != nil {
		log.Fatalf("Query fingerprints failed: %v", err)
	}
	var fingerprints []fpInfo
	for rowsF.Next() {
		var f fpInfo
		_ = rowsF.Scan(&f.empCode, &f.fingerIdx, &f.template)
		fingerprints = append(fingerprints, f)
	}
	rowsF.Close()

	fmt.Printf("Loaded from DB -> Employees: %d, Fingerprints: %d\n", len(employees), len(fingerprints))

	// 2. Connect to Device via COM SDK
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
	const dwMachineNumber int32 = 1

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, port)
	if err != nil || res == nil || res.Val == 0 {
		log.Fatalf("Connect failed: %v", err)
	}
	defer oleutil.CallMethod(zkem, "Disconnect")

	fmt.Println("Connected to device successfully!")

	// Disable device during update
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, false)

	// Push Employees
	userPushed := 0
	for _, e := range employees {
		resUser, err := oleutil.CallMethod(zkem, "SSR_SetUserInfoMX",
			dwMachineNumber, e.code, e.name, "", 0, true,
		)
		if err == nil && resUser != nil && resUser.Val != 0 {
			userPushed++
			fmt.Printf(" [User Pushed] Code: '%s' | Name: '%s'\n", e.code, e.name)
		} else {
			// Fallback SetUserInfo
			resUser2, _ := oleutil.CallMethod(zkem, "SetUserInfo",
				dwMachineNumber, int32(1), e.name, "", 0, true,
			)
			fmt.Printf(" [User Fallback] Code: '%s' res: %v\n", e.code, resUser2)
		}
	}

	// Push Fingerprints
	fpPushed := 0
	for _, f := range fingerprints {
		resFp, err := oleutil.CallMethod(zkem, "SSR_SetUserTmpStr",
			dwMachineNumber, f.empCode, int32(f.fingerIdx), f.template,
		)
		if err == nil && resFp != nil && resFp.Val != 0 {
			fpPushed++
			fmt.Printf("   [Fingerprint Pushed] Code: '%s', Index: %d\n", f.empCode, f.fingerIdx)
		} else {
			fmt.Printf("   [Fingerprint Fail] Code: '%s', Index: %d, err: %v\n", f.empCode, f.fingerIdx, err)
		}
	}

	// COMMIT TO HARDWARE FLASH MEMORY
	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	oleutil.CallMethod(zkem, "EnableDevice", dwMachineNumber, true)

	fmt.Printf("\nPush Complete! Pushed Users: %d, Fingerprints: %d\n", userPushed, fpPushed)

	// Verify User Count via GetDeviceStatus
	var vUsers = ole.NewVariant(ole.VT_I4, 0)
	var vFingers = ole.NewVariant(ole.VT_I4, 0)
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(1), &vUsers)
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(2), &vFingers)
	fmt.Printf("Hardware Verified Status -> Users in Flash: %v | Fingerprints in Flash: %v\n", vUsers.Value(), vFingers.Value())
}
