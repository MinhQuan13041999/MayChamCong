package main

import (
	"fmt"
	"log"
	"runtime"
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
	oleutil.CallMethod(zkem, "RegEvent", dwMachineNumber, int32(65535))

	fmt.Println("Connected & Registered Event! Press/Scan fingerprint now...")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(20 * time.Second)

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			resReadRT, _ := oleutil.CallMethod(zkem, "ReadRTLog", dwMachineNumber)
			if resReadRT == nil || resReadRT.Val == 0 {
				continue
			}

			// Try Method 1: GetGeneralLogDataStr
			var (
				vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
				vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
				vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
				vTimeStr      = ole.NewVariant(ole.VT_BSTR, 0)
			)
			res1, err1 := oleutil.CallMethod(zkem, "GetGeneralLogDataStr",
				dwMachineNumber, &vEnrollNumber, &vVerifyMode, &vInOutMode, &vTimeStr,
			)
			fmt.Printf("GetGeneralLogDataStr -> res: %v, err: %v | PIN: '%v' Time: '%v'\n",
				res1.Value(), err1, vEnrollNumber.Value(), vTimeStr.Value())

			// Try Method 2: GetRTLogExt
			res2, err2 := oleutil.CallMethod(zkem, "GetRTLogExt", dwMachineNumber)
			fmt.Printf("GetRTLogExt -> res: %v, err: %v\n", res2, err2)

			// Try Method 3: GetGeneralExtLogData
			res3, err3 := oleutil.CallMethod(zkem, "GetGeneralExtLogData", dwMachineNumber)
			fmt.Printf("GetGeneralExtLogData -> res: %v, err: %v\n", res3, err3)

			vEnrollNumber.Clear()
			vVerifyMode.Clear()
			vInOutMode.Clear()
			vTimeStr.Clear()
		}
	}
}
