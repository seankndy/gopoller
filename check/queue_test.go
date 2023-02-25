package check

import (
	"testing"
	"time"
)

func TestMemoryCheckQueueEnqueuesAndDequeues(t *testing.T) {
	q := NewMemoryCheckQueue()

	sixtySecAgo := time.Now().Add(-(60 * time.Second))
	ninetySecAgo := time.Now().Add(-(90 * time.Second))
	thirtySecAgo := time.Now().Add(-(30 * time.Second))

	check1 := Check{Id: "12345", Schedule: &PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &sixtySecAgo}
	check2 := Check{Id: "54321", Schedule: &PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &ninetySecAgo}
	check3 := Check{Id: "56789", Schedule: &PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &thirtySecAgo} // not due yet

	q.Enqueue(check1)
	q.Enqueue(check2)
	q.Enqueue(check3)

	var c *Check
	c = q.Dequeue()
	if c.Id != check2.Id {
		t.Errorf("Dequeue(): expected check with ID %v, got %v", check2.Id, c.Id)
	}
	c = q.Dequeue()
	if c.Id != check1.Id {
		t.Errorf("Dequeue(): expected check with ID %v, got %v", check1.Id, c.Id)
	}
	c = q.Dequeue()
	if c != nil {
		t.Errorf("Dequeue(): expected check to be nil, got %v", c)
	}
	cnt := q.Count()
	if cnt != 1 {
		t.Errorf("Count(): expected queue to be 1, got %v", cnt)
	}
}
