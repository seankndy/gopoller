package check

import (
	"time"
)

// Schedule is used by a Check to provide its execution schedule.
type Schedule interface {
	// IsDue returns true if the Check is currently due, otherwise false.
	IsDue(Check) bool

	// DueAt returns a time.Time of the exact point in time the Check will
	// be next due.
	DueAt(Check) time.Time
}

// PeriodicSchedule is a simple scheduler that is due every IntervalSeconds
// seconds
type PeriodicSchedule struct {
	IntervalSeconds int
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
