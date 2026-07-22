package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
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

	const dwMachineNumber int32 = 1
	oleutil.CallMethod(zkem, "ReadAllUserData", dwMachineNumber)

	var (
		vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
		vName         = ole.NewVariant(ole.VT_BSTR, 0)
		vPassword     = ole.NewVariant(ole.VT_BSTR, 0)
		vPrivilege    = ole.NewVariant(ole.VT_I4, 0)
		vEnabled      = ole.NewVariant(ole.VT_BOOL, 0)
	)

	count := 0
	for {
		resLoop, err := oleutil.CallMethod(zkem, "SSR_GetAllUserInfo",
			dwMachineNumber, &vEnrollNumber, &vName, &vPassword, &vPrivilege, &vEnabled,
		)
		if err != nil || resLoop == nil || resLoop.Val == 0 {
			break
		}
		count++
		fmt.Printf("User #%d | Code: '%v' | Name: '%v'\n", count, vEnrollNumber.Value(), vName.Value())
	}
	fmt.Printf("Total users on device after ReadAllUserData: %d\n", count)
}
