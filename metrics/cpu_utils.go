package metrics

import "time"

type AvgCPUUtil struct {
	Timestamp time.Time
	Percent   float64
}

type AvgCPUUtils []*AvgCPUUtil

func (a AvgCPUUtils) After(gte time.Time) AvgCPUUtils {
	for i, cpuUtil := range a {
		if cpuUtil.Timestamp.After(gte) {
			return a[i:]
		}
	}
	return nil
}

func (a AvgCPUUtils) AllGreaterThan(threshold float64) bool {
	for _, cpuUtil := range a {
		if cpuUtil.Percent <= threshold {
			return false
		}
	}
	return true
}

func (a AvgCPUUtils) AllLessThan(threshold float64) bool {
	for _, cpuUtil := range a {
		if cpuUtil.Percent >= threshold {
			return false
		}
	}
	return true
}
