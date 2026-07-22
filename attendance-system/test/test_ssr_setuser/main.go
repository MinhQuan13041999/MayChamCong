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
	const dwMachineNumber int32 = 1

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, port)
	if err != nil || res == nil || res.Val == 0 {
		log.Fatalf("Connect failed: %v", err)
	}
	defer oleutil.CallMethod(zkem, "Disconnect")

	fmt.Println("Testing SSR_SetUserInfo...")

	// Try SSR_SetUserInfo
	resUser, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo",
		dwMachineNumber, "1", "Minh Quan", "", int32(0), true,
	)
	fmt.Printf("SSR_SetUserInfo result: %v, err: %v\n", resUser.Value(), err)

	// Refresh
	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)

	// Check total users
	var vUsers = ole.NewVariant(ole.VT_I4, 0)
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(1), &vUsers)
	fmt.Printf("Users in Flash after SSR_SetUserInfo: %v\n", vUsers.Value())
}
