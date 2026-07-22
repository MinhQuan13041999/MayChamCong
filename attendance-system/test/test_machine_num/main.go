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

	for m := int32(0); m <= 5; m++ {
		resRead, err := oleutil.CallMethod(zkem, "ReadGeneralLogData", m)
		fmt.Printf("ReadGeneralLogData(Machine=%d) return: %v, err: %v\n", m, resRead.Value(), err)

		var (
			vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
			vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
			vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
			vYear         = ole.NewVariant(ole.VT_I4, 0)
			vMonth        = ole.NewVariant(ole.VT_I4, 0)
			vDay          = ole.NewVariant(ole.VT_I4, 0)
			vHour         = ole.NewVariant(ole.VT_I4, 0)
			vMinute       = ole.NewVariant(ole.VT_I4, 0)
			vSecond       = ole.NewVariant(ole.VT_I4, 0)
			vWorkCode     = ole.NewVariant(ole.VT_I4, 0)
		)

		count := 0
		for {
			resLoop, err := oleutil.CallMethod(zkem, "SSR_GetGeneralLogData",
				m, &vEnrollNumber, &vVerifyMode, &vInOutMode,
				&vYear, &vMonth, &vDay, &vHour, &vMinute, &vSecond, &vWorkCode,
			)
			if err != nil || resLoop == nil || resLoop.Val == 0 {
				break
			}
			count++
		}
		fmt.Printf("  -> Logs count for Machine %d: %d\n", m, count)
	}
}
