package check

import (
	"testing"
	"time"
)

type testScheduler struct {
	dueAt time.Time
}

func (s testScheduler) DueAt(*Check) time.Time {
	return s.dueAt
}

func TestCheck_DueAt(t *testing.T) {
	d := time.Date(2022, 4, 1, 15, 30, 0, 0, time.UTC)
	c := &Check{Schedule: &testScheduler{dueAt: d}}
	want := d
	got := c.DueAt()
	if got.Compare(want) != 0 {
		t.Errorf("DueAt(): expected %v, got %v", want, got)
	}
}

func TestCheck_IsDue(t *testing.T) {
	{
		c := &Check{Schedule: &testScheduler{dueAt: time.Now()}}
		if c.IsDue() != true {
			t.Error("expected IsDue() to be true, was false")
		}
	}
	{
		c := &Check{Schedule: &testScheduler{dueAt: time.Now().Add(10 * time.Second)}}
		if c.IsDue() != false {
			t.Error("expected IsDue() to be false, was true")
		}
	}
}

func TestPeriodicSchedule_DueAt(t *testing.T) {
	{
		intervalSeconds := 10
		lastCheck := time.Now().Add(-5 * time.Second)

		c := &Check{LastCheck: &lastCheck}
		s := PeriodicSchedule{IntervalSeconds: intervalSeconds}

		want := lastCheck.Add(time.Duration(intervalSeconds) * time.Second)
		got := s.DueAt(c)
		if got.Compare(want) != 0 {
			t.Errorf("DueAt(): expected %v, got %v", want, got)
		}
	}
	{
		intervalSeconds := 10
		lastCheck := time.Now()

		c := &Check{LastCheck: &lastCheck}
		s := PeriodicSchedule{IntervalSeconds: intervalSeconds}

		want := lastCheck.Add(time.Duration(intervalSeconds) * time.Second)
		got := s.DueAt(c)
		if got.Compare(want) != 0 {
			t.Errorf("DueAt(): expected %v, got %v", want, got)
		}
	}
	{
		intervalSeconds := 10

		c := &Check{LastCheck: nil}
		s := PeriodicSchedule{IntervalSeconds: intervalSeconds}

		got := s.DueAt(c)
		want := time.Now()
		if got.Compare(want) > 0 {
			t.Errorf("DueAt(): expected to be <= %v, but was %v", want, got)
		}
	}
}
