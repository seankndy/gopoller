package check

import (
	"math"
	"sync"
)

// Queue is used by a server.Server to feed it work (Checks to execute).
type Queue interface {
	Enqueue(check Check)
	Dequeue() *Check
	Flush()
	Count() uint64
}

// MemoryQueue is a min priority queue that stores its checks in a map.
// Priorities are derived from the Check's DueAt() timestamp so that the
// checks with the oldest timestamps come out first.
type MemoryQueue struct {
	checks      map[int64][]Check
	total       uint64
	priorities  map[int64]int64
	minPriority int64
	sync.RWMutex
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		checks:      make(map[int64][]Check),
		priorities:  make(map[int64]int64),
		minPriority: math.MaxInt64,
	}
}

func (m *MemoryQueue) Enqueue(check Check) {
	priority := check.DueAt().Unix()

	m.Lock()
	defer m.Unlock()

	m.checks[priority] = append(m.checks[priority], check)
	m.total++

	_, ok := m.priorities[priority]
	if !ok {
		m.priorities[priority] = priority
		m.minPriority = int64(math.Min(float64(priority), float64(m.minPriority)))
	}
}

func (m *MemoryQueue) Dequeue() *Check {
	m.Lock()
	defer m.Unlock()

	_, ok := m.checks[m.minPriority]
	if !ok {
		return nil
	}

	// if top-most Check is not due, then nothing is due.
	check := m.checks[m.minPriority][0]
	if !check.IsDue() {
		return nil
	}

	// check is due, delete it from the queue
	m.checks[m.minPriority] = m.checks[m.minPriority][1:]
	m.total--

	// if there are no checks left at this priority, remove the priority
	// and set minPriority to the next in line
	if len(m.checks[m.minPriority]) == 0 {
		delete(m.priorities, m.minPriority)
		delete(m.checks, m.minPriority)

		m.minPriority = math.MaxInt64
		for p, _ := range m.priorities {
			if p < m.minPriority {
				m.minPriority = p
			}
		}
	}

	return &check
}

func (m *MemoryQueue) Flush() {
	m.Lock()
	defer m.Unlock()

	m.checks = make(map[int64][]Check)
	m.priorities = make(map[int64]int64)
	m.minPriority = math.MaxInt64
	m.total = 0
}

func (m *MemoryQueue) Count() uint64 {
	m.RLock()
	defer m.RUnlock()

	return m.total
}
