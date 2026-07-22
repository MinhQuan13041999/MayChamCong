//go:build ignore

// This maintenance utility is run explicitly with `go run read_logs.go`.
// Keeping it out of the root package prevents it from colliding with other
// standalone utilities during `go test ./...`.
package main

import (
	"fmt"
	"log"

	"github.com/canhlinh/gozk"
)

func main() {
	zk := gozk.NewZK("192.168.11.151", gozk.WithPort(8818), gozk.WithTCP(true))
	if err := zk.Connect(); err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer zk.Disconnect()

	t, err := zk.GetTime()
	if err != nil {
		fmt.Printf("GetTime failed: %v\n", err)
	} else {
		fmt.Printf("Device current time: %s\n", t.Format("2006-01-02 15:04:05"))
	}

	events, err := zk.GetAllScannedEvents()
	if err != nil {
		log.Fatalf("GetAllScannedEvents failed: %v", err)
	}

	fmt.Printf("Total raw events on device: %d\n", len(events))
	for i, e := range events {
		fmt.Printf("[%d] UserID: %d, Time: %s, Error: %v\n", i, e.UserID, e.Timestamp.Format("2006-01-02 15:04:05"), e.Error)
	}
}
