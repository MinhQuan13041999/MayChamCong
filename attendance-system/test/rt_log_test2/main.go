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

	fmt.Println("Connected! Testing GetRTLog(1)...")
	const dwMachineNumber int32 = 1

	resReadRT, err := oleutil.CallMethod(zkem, "ReadRTLog", dwMachineNumber)
	if resReadRT != nil {
		fmt.Printf("ReadRTLog: %v err: %v\n", resReadRT.Value(), err)
	}

	resGetRT, err := oleutil.CallMethod(zkem, "GetRTLog", dwMachineNumber)
	if resGetRT != nil {
		fmt.Printf("GetRTLog(1) return: %v, err: %v\n", resGetRT.Value(), err)
	} else {
		fmt.Printf("GetRTLog(1) nil, err: %v\n", err)
	}
}
