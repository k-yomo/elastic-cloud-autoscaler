package clock

import (
	"testing"
	"time"
)

func MockTime(t *testing.T, tm time.Time) {
	t.Helper()
	Now = func() time.Time {
		return tm
	}
}
