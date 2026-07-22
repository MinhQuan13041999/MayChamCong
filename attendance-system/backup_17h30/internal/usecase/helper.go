package usecase

import (
	"time"
)

func isDeviceOnline(lastHeartbeatAt *time.Time, threshold time.Duration) bool {
	if lastHeartbeatAt == nil {
		return false
	}
	const layout = "2006-01-02 15:04:05"
	utc1, err1 := time.ParseInLocation(layout, lastHeartbeatAt.Format(layout), time.UTC)
	utc2, err2 := time.ParseInLocation(layout, time.Now().Format(layout), time.UTC)
	if err1 != nil || err2 != nil {
		diff := time.Since(*lastHeartbeatAt)
		if diff < 0 {
			diff = -diff
		}
		return diff <= threshold
	}
	diff := utc2.Unix() - utc1.Unix()
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(threshold.Seconds())
}
