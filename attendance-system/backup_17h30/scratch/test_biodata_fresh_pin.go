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

	// 1. Enqueue USERINFO for PIN 772
	var userCmdID int64
	_ = pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&userCmdID)
	userCmd := "DATA UPDATE USERINFO PIN=772\tName=FreshUser\tPri=0\tPasswd=\tCard=\tGrp=1"
	_, _ = pool.Exec(ctx, `
		INSERT INTO device_command_queue (device_id, command_id, command, status)
		VALUES ($1, $2, $3, 'pending')`, deviceID, userCmdID, userCmd)
	fmt.Printf("Enqueued USERINFO (ID=%d)\n", userCmdID)

	realTemplate := "TflTUzIxAAAEur0ECAUHCc7QAAAcu2kBAAAAhGcrvLoiAO0MlACLAFy14wBUABUPVABbutwPtQBiACsP9rpnACkPZwCtAGS1rwB4AFQOKwCFujwPpgCGABAOdLqPAFIP2gBiAEu0uAC3AM0PdgDCuskOwQDGABcOp7rLADEN9QAIAEK1SQDUAEcOsgDSujoP4gDWAIsPsbrZAL8NbAAaADq1iwDhADIPWwDguhINxQDqAKsORrrsAMcLgADLAbG0dQAPAbYOkQAVuyQOlAAVAW4OS7oZATUNVgDnAS+3IgApAbYPiAAuu0kN3QA0ASYPO7pAAZkPswCHAYy0zgBCAQIOgABBu8gPrgBMAUUNybpSAWcPpgCdAXSxtABbAVgNNpbTubrbcXxCbpYEiVSi2OMobYRv8cYw4J4WESqaPBaZVdo/WhIah8oJhTr4asp6CRdf8/+SgIB+6lr7I/YvxAp2ooWOfsMXAb7g/BmfufhM/DUiMAxdBAkWgGhpzJgjqfnj+AZ1gTlHCj8MqfyYClKQIAma4cZ9uPmdVFySKQRGDBf6UZvk+tL09eGcVzFdtLL6Ic6dQIPuHK+LSAPZDY+TzLlLkNIMtPJc/K1lZPle9J+Z4RJwtZDtuOf8+9QI7qfPc+rvHB84FQGd/OHyfRJfxdK56YflTAs1MogbwSZgIQF/UDJAg44Y1ftZEDEoGGSJqLPg6fAZGCDrtVLY2OXSMo07PFW9IUIBApIePwQEIAFiZAcAssVpevcLAMEAcMOhW8W1Ae8Ad8FsBFr7e18FAKUBZwdHCLrQAXTCwsE7dsTsBwECEIZ5BwYFuBiMksANAAUab3hkwllYAwAkHAdFEADcHXfBQ8Fee8HB/8JLCsQDKCrAl8HAiQvEATcpwsHCk8F+2wD3joiSwsGFcAdSxePBw8LDaXfRAQf6jpDDiXDAB8Fe5xMBAkmTklfCYs5xDACQT1yoVsZFdwYAlFJXBWQHuuZVE/8XAcFWmiWTwcHBfMCxgIC9AbVe6f/9OiEBurhi6f0cC8WxYdqJYsHAwg7F7HkTxcXFwsX/B8LEesP/BwDTgrTFx3uIAwECgz06BwRWhDo2OBIAN4UzRFVQQP79/j81AbrSiED/LQPFVpTtwAsAbJBWvcFgPRIAcZJQ/wdkh3nCwGfClgPFxZD1wAYAzJVMBf2atgHbosPBxQ/IxnnAwZoFANZsUMTlCQDcqkk4Ov39lAH/rrf+wQF9wX7DwsLDwMAEwcR7wsDEwsbHAcP6esPBwcH/wwbFzH/CfokTAP1zRsZEVsH+JDP8+QUFuLc9UQQA/zA9JbkBM81twQ/F9NX5/kbAHvz8O8D6swFL2EZuwz9dAbp12T3B/sTDAH5jPIPBCwDeH1NTQfr8/CgNACHaTUTA/v78+/05KgC6aOJAjAQAquI5wAQAnecrxQ3GAbo38lN6BgD89ymdwgMANv03OggU4hMkwf/9++4JFOkUKcHB+fw6Gw6qTh03wvr7Ofr4lgIQJC0pxcEQHo4lnQkQYzWD/vhG+v79/wUQKTuEOwUQfjw9I8IQ9/F7/MHBx//BEBD2LKwEENFTzH8BqvVbd//9yZdCBLFCAQAAC0WXAA=="

	formats := []string{
		// Format 1: BIODATA Pin= (CamelCase), Tmp= (CamelCase)
		fmt.Sprintf("DATA UPDATE BIODATA Pin=772\tNo=1\tIndex=0\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTmp=%s", realTemplate),
		// Format 2: BIODATA PIN= (All Caps), Tmp= (CamelCase)
		fmt.Sprintf("DATA UPDATE BIODATA PIN=772\tNo=1\tIndex=0\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTmp=%s", realTemplate),
		// Format 3: BIODATA PIN= (All Caps), TMP= (All Caps)
		fmt.Sprintf("DATA UPDATE BIODATA PIN=772\tNo=1\tIndex=0\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTMP=%s", realTemplate),
		// Format 4: BIODATA Pin= (CamelCase), TMP= (All Caps)
		fmt.Sprintf("DATA UPDATE BIODATA Pin=772\tNo=1\tIndex=0\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTMP=%s", realTemplate),
	}

	cmdIDs := make([]int64, len(formats))

	for i, format := range formats {
		var nextCmdID int64
		err = pool.QueryRow(ctx, "SELECT nextval('device_command_id_seq')").Scan(&nextCmdID)
		if err != nil {
			log.Fatalf("get nextval failed: %v\n", err)
		}
		cmdIDs[i] = nextCmdID

		_, err = pool.Exec(ctx, `
			INSERT INTO device_command_queue (device_id, command_id, command, status)
			VALUES ($1, $2, $3, 'pending')`, deviceID, nextCmdID, format)
		if err != nil {
			log.Fatalf("enqueue failed: %v\n", err)
		}
		fmt.Printf("Enqueued Fingerprint Format %d (ID=%d)\n", i+1, nextCmdID)
	}

	fmt.Println("\nMonitoring statuses for 35 seconds...")
	for step := 0; step < 35; step++ {
		time.Sleep(1 * time.Second)
		allDone := true
		fmt.Printf("[%s] State:\n", time.Now().Format("15:04:05"))
		var userStatus string
		_ = pool.QueryRow(ctx, "SELECT status FROM device_command_queue WHERE command_id = $1 AND device_id = $2", userCmdID, deviceID).Scan(&userStatus)
		fmt.Printf("  USERINFO (ID=%d): %s\n", userCmdID, userStatus)

		for i, cmdID := range cmdIDs {
			var status string
			var sentAt, ackedAt *time.Time
			err = pool.QueryRow(ctx, "SELECT status, sent_at, acked_at FROM device_command_queue WHERE command_id = $1 AND device_id = $2", cmdID, deviceID).Scan(&status, &sentAt, &ackedAt)
			if err != nil {
				fmt.Printf("  Format %d: Query error: %v\n", i+1, err)
				continue
			}
			fmt.Printf("  Format %d (ID=%d): Status: %s | Sent: %s | Acked: %s\n",
				i+1, cmdID, status, formatTime(sentAt), formatTime(ackedAt))
			if status == "pending" || status == "sent" {
				allDone = false
			}
		}
		if allDone && userStatus == "ack" {
			fmt.Println("All commands processed.")
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
