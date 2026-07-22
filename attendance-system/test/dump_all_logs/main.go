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
	resRead, err := oleutil.CallMethod(zkem, "ReadGeneralLogData", dwMachineNumber)
	fmt.Printf("ReadGeneralLogData result: %v, err: %v\n", resRead.Value(), err)

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
			dwMachineNumber, &vEnrollNumber, &vVerifyMode, &vInOutMode,
			&vYear, &vMonth, &vDay, &vHour, &vMinute, &vSecond, &vWorkCode,
		)
		if err != nil || resLoop == nil || resLoop.Val == 0 {
			break
		}
		count++
		pin := variantString(&vEnrollNumber)
		y, _ := variantInt32(&vYear)
		m, _ := variantInt32(&vMonth)
		d, _ := variantInt32(&vDay)
		h, _ := variantInt32(&vHour)
		min, _ := variantInt32(&vMinute)
		s, _ := variantInt32(&vSecond)

		fmt.Printf("Log #%d | PIN: '%s' | Time: %04d-%02d-%02d %02d:%02d:%02d\n", count, pin, y, m, d, h, min, s)
	}

	fmt.Printf("Total logs dumped: %d\n", count)
	fmt.Printf("Current PC Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
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

func variantInt32(v *ole.VARIANT) (int32, bool) {
	if v == nil || v.Value() == nil {
		return 0, false
	}
	switch val := v.Value().(type) {
	case int32:
		return val, true
	case int64:
		return int32(val), true
	case int:
		return int32(val), true
	case float64:
		return int32(val), true
	default:
		return 0, false
	}
}
