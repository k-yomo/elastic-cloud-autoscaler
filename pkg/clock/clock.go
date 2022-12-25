package clock

import (
	"time"

	"github.com/pkg/errors"
)

var Now = time.Now

func SetTimeZone(timeZone string) error {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return errors.Wrap(err, "load timezone")
	}
	time.Local = loc
	return nil
}
