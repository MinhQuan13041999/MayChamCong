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

	fmt.Println("=== 1. DIAGNOSING USERS ON DEVICE ===")
	const dwMachineNumber int32 = 1
	oleutil.CallMethod(zkem, "ReadAllUserID", dwMachineNumber)

	var (
		vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
		vName         = ole.NewVariant(ole.VT_BSTR, 0)
		vPassword     = ole.NewVariant(ole.VT_BSTR, 0)
		vPrivilege    = ole.NewVariant(ole.VT_I4, 0)
		vEnabled      = ole.NewVariant(ole.VT_BOOL, 0)
	)

	userCount := 0
	for {
		resUser, err := oleutil.CallMethod(zkem, "SSR_GetAllUserInfo",
			dwMachineNumber, &vEnrollNumber, &vName, &vPassword, &vPrivilege, &vEnabled,
		)
		if err != nil || resUser == nil || resUser.Val == 0 {
			break
		}
		userCount++
		fmt.Printf("User #%d: PIN '%s', Name '%s', Privilege %v, Enabled %v\n",
			userCount, variantString(&vEnrollNumber), variantString(&vName), vPrivilege.Value(), vEnabled.Value())
	}
	fmt.Printf("Total users found on device: %d\n\n", userCount)

	vEnrollNumber.Clear()
	vName.Clear()
	vPassword.Clear()
	vPrivilege.Clear()
	vEnabled.Clear()

	fmt.Println("=== 2. TESTING LOG RETRIEVAL METHODS ===")

	// Test Method 1: SSR_GetGeneralLogData
	resRead1, _ := oleutil.CallMethod(zkem, "ReadGeneralLogData", dwMachineNumber)
	fmt.Printf("Method 1: ReadGeneralLogData return: %v\n", resRead1.Value())

	var (
		vPin        = ole.NewVariant(ole.VT_BSTR, 0)
		vVerifyMode = ole.NewVariant(ole.VT_I4, 0)
		vInOutMode  = ole.NewVariant(ole.VT_I4, 0)
		vYear       = ole.NewVariant(ole.VT_I4, 0)
		vMonth      = ole.NewVariant(ole.VT_I4, 0)
		vDay        = ole.NewVariant(ole.VT_I4, 0)
		vHour       = ole.NewVariant(ole.VT_I4, 0)
		vMinute     = ole.NewVariant(ole.VT_I4, 0)
		vSecond     = ole.NewVariant(ole.VT_I4, 0)
		vWorkCode   = ole.NewVariant(ole.VT_I4, 0)
	)

	cnt1 := 0
	for {
		resL, err := oleutil.CallMethod(zkem, "SSR_GetGeneralLogData",
			dwMachineNumber, &vPin, &vVerifyMode, &vInOutMode,
			&vYear, &vMonth, &vDay, &vHour, &vMinute, &vSecond, &vWorkCode,
		)
		if err != nil || resL == nil || resL.Val == 0 {
			break
		}
		cnt1++
		p := variantString(&vPin)
		y, _ := variantInt32(&vYear)
		m, _ := variantInt32(&vMonth)
		d, _ := variantInt32(&vDay)
		h, _ := variantInt32(&vHour)
		min, _ := variantInt32(&vMinute)
		s, _ := variantInt32(&vSecond)
		fmt.Printf("   [SSR Log #%d] PIN '%s' at %04d-%02d-%02d %02d:%02d:%02d\n", cnt1, p, y, m, d, h, min, s)
	}
	fmt.Printf("Method 1 (SSR_GetGeneralLogData) total logs: %d\n\n", cnt1)

	vPin.Clear()
	vVerifyMode.Clear()
	vInOutMode.Clear()
	vYear.Clear()
	vMonth.Clear()
	vDay.Clear()
	vHour.Clear()
	vMinute.Clear()
	vSecond.Clear()
	vWorkCode.Clear()

	// Test Method 2: GetGeneralLogData (Legacy numerical dwEnrollNumber)
	oleutil.CallMethod(zkem, "ReadGeneralLogData", dwMachineNumber)
	var (
		vDwEnrollNum = ole.NewVariant(ole.VT_I4, 0)
		vVerifyM     = ole.NewVariant(ole.VT_I4, 0)
		vInOutM      = ole.NewVariant(ole.VT_I4, 0)
		vY           = ole.NewVariant(ole.VT_I4, 0)
		vM           = ole.NewVariant(ole.VT_I4, 0)
		vD           = ole.NewVariant(ole.VT_I4, 0)
		vH           = ole.NewVariant(ole.VT_I4, 0)
		vMin         = ole.NewVariant(ole.VT_I4, 0)
		vSec         = ole.NewVariant(ole.VT_I4, 0)
	)

	cnt2 := 0
	for {
		resL2, err := oleutil.CallMethod(zkem, "GetGeneralLogData",
			dwMachineNumber, &vDwEnrollNum, &vVerifyM, &vInOutM,
			&vY, &vM, &vD, &vH, &vMin, &vSec,
		)
		if err != nil || resL2 == nil || resL2.Val == 0 {
			break
		}
		cnt2++
		en, _ := variantInt32(&vDwEnrollNum)
		y, _ := variantInt32(&vY)
		m, _ := variantInt32(&vM)
		d, _ := variantInt32(&vD)
		h, _ := variantInt32(&vH)
		min, _ := variantInt32(&vMin)
		s, _ := variantInt32(&vSec)
		fmt.Printf("   [Legacy Log #%d] EnrollID %d at %04d-%02d-%02d %02d:%02d:%02d\n", cnt2, en, y, m, d, h, min, s)
	}
	fmt.Printf("Method 2 (GetGeneralLogData) total logs: %d\n\n", cnt2)

	vDwEnrollNum.Clear()
	vVerifyM.Clear()
	vInOutM.Clear()
	vY.Clear()
	vM.Clear()
	vD.Clear()
	vH.Clear()
	vMin.Clear()
	vSec.Clear()

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
