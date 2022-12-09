package timeutil

import "time"

const RFC3339Milli = "2006-01-02T15:04:05.999Z07:00"

func MaxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	} else {
		return b
	}
}
