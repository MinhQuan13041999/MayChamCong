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
	oleutil.CallMethod(zkem, "RegEvent", dwMachineNumber, int32(65535))

	fmt.Println("Connected! Reading via GetGeneralExtLogData for 20s...")

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

			var (
				vEnrollNumber = ole.NewVariant(ole.VT_I4, 0)
				vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
				vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
				vYear         = ole.NewVariant(ole.VT_I4, 0)
				vMonth        = ole.NewVariant(ole.VT_I4, 0)
				vDay          = ole.NewVariant(ole.VT_I4, 0)
				vHour         = ole.NewVariant(ole.VT_I4, 0)
				vMinute       = ole.NewVariant(ole.VT_I4, 0)
				vSecond       = ole.NewVariant(ole.VT_I4, 0)
			)

			resLoop, err := oleutil.CallMethod(zkem, "GetGeneralExtLogData",
				dwMachineNumber, &vEnrollNumber, &vVerifyMode, &vInOutMode,
				&vYear, &vMonth, &vDay, &vHour, &vMinute, &vSecond,
			)
			if err == nil && resLoop != nil && resLoop.Val != 0 {
				fmt.Printf("🎉 GetGeneralExtLogData DETECTED! PIN: %v | Time: %v-%v-%v %v:%v:%v\n",
					vEnrollNumber.Value(), vYear.Value(), vMonth.Value(), vDay.Value(), vHour.Value(), vMinute.Value(), vSecond.Value())
			} else if err != nil {
				fmt.Printf("GetGeneralExtLogData err: %v\n", err)
			}

			vEnrollNumber.Clear()
			vVerifyMode.Clear()
			vInOutMode.Clear()
			vYear.Clear()
			vMonth.Clear()
			vDay.Clear()
			vHour.Clear()
			vMinute.Clear()
			vSecond.Clear()
		}
	}
}

func variantString(v *ole.VARIANT) string {
	if v == nil || v.Value() == nil {
		return ""
	}
	if s, ok := v.Value().(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v.Value()))
}
