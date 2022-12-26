package clock

import (
	"testing"
	"time"
)

// MockTime is not concurrency safe, so this can't be used with t.Parallel
func MockTime(t *testing.T, tm time.Time) (reset func()) {
	t.Helper()
	Now = func() time.Time {
		return tm
	}
	return func() {
		Now = time.Now
	}
}
