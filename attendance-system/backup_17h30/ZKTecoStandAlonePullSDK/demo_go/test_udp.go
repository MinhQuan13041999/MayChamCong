package main

import (
	"fmt"
	"time"

	"github.com/canhlinh/gozk"
)

func main() {
	ip := "192.168.11.151"
	port := 4370

	fmt.Printf("Testing UDP connection to ZKTeco device at %s:%d (using pure Go gozk)...\n", ip, port)

	// Try UDP connection (WithTCP is false by default if not set, or we can configure it)
	zk := gozk.NewZK(ip, gozk.WithPort(port), gozk.WithTCP(false), gozk.WithTimeout(5*time.Second))
	
	err := zk.Connect()
	if err != nil {
		fmt.Printf("[-] UDP Connection FAILED: %v\n", err)
		
		fmt.Println("\nTesting TCP connection to ZKTeco device...")
		zkTCP := gozk.NewZK(ip, gozk.WithPort(port), gozk.WithTCP(true), gozk.WithTimeout(5*time.Second))
		errTCP := zkTCP.Connect()
		if errTCP != nil {
			fmt.Printf("[-] TCP Connection FAILED: %v\n", errTCP)
		} else {
			fmt.Println("[+] TCP Connection SUCCESS!")
			zkTCP.Disconnect()
		}
		return
	}

	fmt.Println("[+] UDP Connection SUCCESS!")
	
	version, err := zk.GetFirmwareVersion()
	if err == nil {
		fmt.Printf("[+] Firmware Version: %s\n", version)
	} else {
		fmt.Printf("[-] Failed to get firmware version: %v\n", err)
	}

	zk.Disconnect()
}
