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

	fmt.Println("Connected to device successfully!")
	const dwMachineNumber int32 = 1

	// 1. GetDeviceStatus (Status Code 6 = Record Count)
	var recordCount int32 = -1
	var vStatus = ole.NewVariant(ole.VT_I4, 0)
	resStat, err := oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(6), &vStatus)
	if err == nil && resStat != nil {
		recordCount = vStatus.Value().(int32)
	}
	fmt.Printf("[1] GetDeviceStatus(6) [Total Attendance Record Count in Hardware]: %d\n", recordCount)

	// 2. Test GetDeviceStatus for other codes
	var userCnt, fpCnt, faceCnt int32 = -1, -1, -1
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(2), &vStatus)
	if vStatus.Value() != nil { userCnt = vStatus.Value().(int32) }
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(3), &vStatus)
	if vStatus.Value() != nil { fpCnt = vStatus.Value().(int32) }
	oleutil.CallMethod(zkem, "GetDeviceStatus", dwMachineNumber, int32(21), &vStatus)
	if vStatus.Value() != nil { faceCnt = vStatus.Value().(int32) }

	fmt.Printf("[2] Hardware Stats -> Users: %d | Fingerprints: %d | Faces: %d\n", userCnt, fpCnt, faceCnt)

	// 3. Test ReadGeneralLogData + GetGeneralLogData (legacy non-SSR)
	resRead, _ := oleutil.CallMethod(zkem, "ReadGeneralLogData", dwMachineNumber)
	fmt.Printf("[3] ReadGeneralLogData return: %v\n", resRead.Value())

	var (
		vDW           = ole.NewVariant(ole.VT_I4, 0)
		vEnrollNumber = ole.NewVariant(ole.VT_I4, 0)
		vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
		vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
		vY            = ole.NewVariant(ole.VT_I4, 0)
		vM            = ole.NewVariant(ole.VT_I4, 0)
		vD            = ole.NewVariant(ole.VT_I4, 0)
		vH            = ole.NewVariant(ole.VT_I4, 0)
		vMi           = ole.NewVariant(ole.VT_I4, 0)
		vS            = ole.NewVariant(ole.VT_I4, 0)
		vWorkCode     = ole.NewVariant(ole.VT_I4, 0)
	)

	legacyCount := 0
	for {
		resLoop, err := oleutil.CallMethod(zkem, "GetGeneralLogData",
			dwMachineNumber, &vDW, &vEnrollNumber, &vVerifyMode, &vInOutMode,
			&vY, &vM, &vD, &vH, &vMi, &vS, &vWorkCode,
		)
		if err != nil || resLoop == nil || resLoop.Val == 0 {
			break
		}
		legacyCount++
		if legacyCount <= 5 {
			fmt.Printf("   Legacy Log #%d: DW=%v PIN=%v Time=%v-%v-%v %v:%v:%v\n",
				legacyCount, vDW.Value(), vEnrollNumber.Value(), vY.Value(), vM.Value(), vD.Value(), vH.Value(), vMi.Value(), vS.Value())
		}
	}
	fmt.Printf("   Legacy GetGeneralLogData count: %d\n", legacyCount)

	// 4. Test GetSuperLogData
	superCount := 0
	var (
		vSNumber = ole.NewVariant(ole.VT_I4, 0)
		vSEnroll = ole.NewVariant(ole.VT_I4, 0)
		vSGLog   = ole.NewVariant(ole.VT_I4, 0)
	)
	for {
		resLoop, err := oleutil.CallMethod(zkem, "GetSuperLogData",
			dwMachineNumber, &vSNumber, &vSEnroll, &vSGLog,
			&vVerifyMode, &vInOutMode, &vY, &vM, &vD, &vH, &vMi,
		)
		if err != nil || resLoop == nil || resLoop.Val == 0 {
			break
		}
		superCount++
		if superCount <= 5 {
			fmt.Printf("   Super Log #%d: PIN=%v Time=%v-%v-%v %v:%v\n",
				superCount, vSEnroll.Value(), vY.Value(), vM.Value(), vD.Value(), vH.Value(), vMi.Value())
		}
	}
	fmt.Printf("[4] GetSuperLogData count: %d\n", superCount)
}
