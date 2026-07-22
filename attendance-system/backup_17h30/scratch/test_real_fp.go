//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://postgres:123456@localhost:5432/attendance_db?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect failed: %v\n", err)
	}
	defer pool.Close()

	deviceID := "dba701f3-fc74-4d79-8a43-5f989c29622d"

	// Clear old queue
	_, err = pool.Exec(ctx, "DELETE FROM device_command_queue WHERE device_id = $1", deviceID)
	if err != nil {
		log.Fatalf("clear queue failed: %v\n", err)
	}
	fmt.Println("Cleared old queue.")

	realTemplate := "TflTUzIxAAAEur0ECAUHCc7QAAAcu2kBAAAAhGcrvLoiAO0MlACLAFy14wBUABUPVABbutwPtQBiACsP9rpnACkPZwCtAGS1rwB4AFQOKwCFujwPpgCGABAOdLqPAFIP2gBiAEu0uAC3AM0PdgDCuskOwQDGABcOp7rLADEN9QAIAEK1SQDUAEcOsgDSujoP4gDWAIsPsbrZAL8NbAAaADq1iwDhADIPWwDguhINxQDqAKsORrrsAMcLgADLAbG0dQAPAbYOkQAVuyQOlAAVAW4OS7oZATUNVgDnAS+3IgApAbYPiAAuu0kN3QA0ASYPO7pAAZkPswCHAYy0zgBCAQIOgABBu8gPrgBMAUUNybpSAWcPpgCdAXSxtABbAVgNNpbTubrbcXxCbpYEiVSi2OMobYRv8cYw4J4WESqaPBaZVdo/WhIah8oJhTr4asp6CRdf8/+SgIB+6lr7I/YvxAp2ooWOfsMXAb7g/BmfufhM/DUiMAxdBAkWgGhpzJgjqfnj+AZ1gTlHCj8MqfyYClKQIAma4cZ9uPmdVFySKQRGDBf6UZvk+tL09eGcVzFdtLL6Ic6dQIPuHK+LSAPZDY+TzLlLkNIMtPJc/K1lZPle9J+Z4RJwtZDtuOf8+9QI7qfPc+rvHB84FQGd/OHyfRJfxdK56YflTAs1MogbwSZgIQF/UDJAg44Y1ftZEDEoGGSJqLPg6fAZGCDrtVLY2OXSMo07PFW9IUIBApIePwQEIAFiZAcAssVpevcLAMEAcMOhW8W1Ae8Ad8FsBFr7e18FAKUBZwdHCLrQAXTCwsE7dsTsBwECEIZ5BwYFuBiMksANAAUab3hkwllYAwAkHAdFEADcHXfBQ8Fee8HB/8JLCsQDKCrAl8HAiQvEATcpwsHCk8F+2wD3joiSwsGFcAdSxePBw8LDaXfRAQf6jpDDiXDAB8Fe5xMBAkmTklfCYs5xDACQT1yoVsZFdwYAlFJXBWQHuuZVE/8XAcFWmiWTwcHBfMCxgIC9AbVe6f/9OiEBurhi6f0cC8WxYdqJYsHAwg7F7HkTxcXFwsX/B8LEesP/BwDTgrTFx3uIAwECgz06BwRWhDo2OBIAN4UzRFVQQP79/j81AbrSiED/LQPFVpTtwAsAbJBWvcFgPRIAcZJQ/wdkh3nCwGfClgPFxZD1wAYAzJVMBf2atgHbosPBxQ/IxnnAwZoFANZsUMTlCQDcqkk4Ov39lAH/rrf+wQF9wX7DwsLDwMAEwcR7wsDEwsbHAcP6esPBwcH/wwbFzH/CfokTAP1zRsZEVsH+JDP8+QUFuLc9UQQA/zA9JbkBM81twQ/F9NX5/kbAHvz8O8D6swFL2EZuwz9dAbp12T3B/sTDAH5jPIPBCwDeH1NTQfr8/CgNACHaTUTA/v78+/05KgC6aOJAjAQAquI5wAQAnecrxQ3GAbo38lN6BgD89ymdwgMANv03OggU4hMkwf/9++4JFOkUKcHB+fw6Gw6qTh03wvr7Ofr4lgIQJC0pxcEQHo4lnQkQYzWD/vhG+v79/wUQKTuEOwUQfjw9I8IQ9/F7/MHBx//BEBD2LKwEENFTzH8BqvVbd//9yZdCBLFCAQAAC0WXAA=="

	// Test the FINGERTEMPLATE format with correct fields
	cmdStr := fmt.Sprintf("DATA UPDATE FINGERTEMPLATE PIN=1015\tFingerID=6\tSize=1616\tVal=1\tTemplate=%s", realTemplate)

	var nextCmdID int64
	err = pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&nextCmdID)
	if err != nil {
		log.Fatalf("get nextval failed: %v\n", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO device_command_queue (device_id, command_id, command, status)
		VALUES ($1, $2, $3, 'pending')`, deviceID, nextCmdID, cmdStr)
	if err != nil {
		log.Fatalf("enqueue failed: %v\n", err)
	}
	fmt.Printf("Enqueued command ID=%d\n", nextCmdID)

	fmt.Println("Monitoring command status for 20 seconds...")
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		var status string
		var sentAt, ackedAt *time.Time
		err = pool.QueryRow(ctx, "SELECT status, sent_at, acked_at FROM device_command_queue WHERE command_id = $1 AND device_id = $2", nextCmdID, deviceID).Scan(&status, &sentAt, &ackedAt)
		if err != nil {
			fmt.Printf("Query error: %v\n", err)
			continue
		}
		fmt.Printf("[%s] Status: %s | Sent: %s | Acked: %s\n",
			time.Now().Format("15:04:05"), status, formatTime(sentAt), formatTime(ackedAt))
		if status == "ack" || status == "failed" {
			break
		}
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format("15:04:05")
}
