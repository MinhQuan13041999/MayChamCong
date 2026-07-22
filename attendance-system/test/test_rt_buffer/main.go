package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

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

	// Call RefreshData
	oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)

	// Check total attendance count
	var vValue = ole.NewVariant(ole.VT_I4, 0)
	resStat, errStat := oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(6), &vValue)
	fmt.Printf("GetDeviceStatus(6) [Attendance Record Count]: res=%v, val=%v, err=%v\n",
		resStat.Value(), vValue.Value(), errStat)

	// Check total user count
	var vUsers = ole.NewVariant(ole.VT_I4, 0)
	resUsers, _ := oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(1), &vUsers)
	fmt.Printf("GetDeviceStatus(1) [Total User Count]: val=%v\n", vUsers.Value())

	// Check total fingerprint count
	var vFingers = ole.NewVariant(ole.VT_I4, 0)
	resFingers, _ := oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(2), &vFingers)
	fmt.Printf("GetDeviceStatus(2) [Total Fingerprint Count]: val=%v\n", vFingers.Value())

	vValue.Clear()
	vUsers.Clear()
	vFingers.Clear()
}
