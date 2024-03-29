package memqueue

import (
	"github.com/seankndy/gopoller/check"
	"testing"
	"time"
)

func TestMemoryCheckQueueEnqueuesAndDequeues(t *testing.T) {
	q := NewQueue()

	sixtySecAgo := time.Now().Add(-(60 * time.Second))
	ninetySecAgo := time.Now().Add(-(90 * time.Second))
	thirtySecAgo := time.Now().Add(-(30 * time.Second))

	check1 := &check.Check{Id: "12345", Schedule: &check.PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &sixtySecAgo}
	check2 := &check.Check{Id: "54321", Schedule: &check.PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &ninetySecAgo}
	check3 := &check.Check{Id: "56789", Schedule: &check.PeriodicSchedule{IntervalSeconds: 60}, LastCheck: &thirtySecAgo} // not due yet

	q.Enqueue(check1)
	q.Enqueue(check2)
	q.Enqueue(check3)

	var c *check.Check
	c = q.Dequeue()
	if c == nil {
		t.Fatalf("Dequeue(): expected a Check, got nil")
	}
	if c.Id != check2.Id {
		t.Errorf("Dequeue(): expected check with ID %v, got %v", check2.Id, c.Id)
	}
	c = q.Dequeue()
	if c == nil {
		t.Fatalf("Dequeue(): expected a Check, got nil")
	}
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
