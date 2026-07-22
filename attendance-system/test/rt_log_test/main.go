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

	fmt.Println("Connected to device! Testing ReadRTLog / GetRTLog for 30s...")
	const dwMachineNumber int32 = 1

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-timeout:
			fmt.Println("Finished 30s test.")
			return
		case <-ticker.C:
			resReadRT, err := oleutil.CallMethod(zkem, "ReadRTLog", dwMachineNumber)
			if err == nil && resReadRT != nil && resReadRT.Val != 0 {
				var (
					vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
					vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
					vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
					vTimeStr      = ole.NewVariant(ole.VT_BSTR, 0)
				)

				resGetRT, err := oleutil.CallMethod(zkem, "GetRTLog",
					dwMachineNumber, &vEnrollNumber, &vVerifyMode, &vInOutMode, &vTimeStr,
				)
				fmt.Printf("[%s] ReadRTLog: true | GetRTLog: %v err: %v | PIN: '%v' Time: '%v'\n",
					time.Now().Format("15:04:05"), resGetRT.Value(), err, vEnrollNumber.Value(), vTimeStr.Value())

				vEnrollNumber.Clear()
				vVerifyMode.Clear()
				vInOutMode.Clear()
				vTimeStr.Clear()
			} else {
				fmt.Printf("[%s] ReadRTLog returned false\n", time.Now().Format("15:04:05"))
			}
		}
	}
}
