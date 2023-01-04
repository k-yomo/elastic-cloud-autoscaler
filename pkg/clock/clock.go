package clock

import (
	"fmt"
	"time"
)

var Now = time.Now

func SetTimeZone(timeZone string) error {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}
	time.Local = loc
	return nil
}
