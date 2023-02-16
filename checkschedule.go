package gollector

import "time"

type CheckSchedule interface {
	IsDue(Check) bool
	DueAt(Check) time.Time
}

// PeriodicSchedule is a simple scheduler that is due every IntervalSeconds seconds
type PeriodicSchedule struct {
	IntervalSeconds int64
}

func (s PeriodicSchedule) IsDue(check Check) bool {
	if check.LastCheck == nil {
		return true
	}

	return time.Now().Sub(*check.LastCheck) >= time.Duration(s.IntervalSeconds)*time.Second
}

func (s PeriodicSchedule) DueAt(check Check) time.Time {
	if check.LastCheck == nil {
		return time.Now()
	}

	return check.LastCheck.Add(time.Duration(s.IntervalSeconds) * time.Second)
}
