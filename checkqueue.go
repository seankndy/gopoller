package gollector

import (
	"math"
	"sync"
)

type CheckQueue interface {
	Enqueue(check Check)
	Dequeue() *Check
	Flush()
	Count() uint64
}

type memoryCheckQueue struct {
	checks      map[int64][]Check
	total       uint64
	priorities  map[int64]int64
	minPriority int64
	mu          sync.RWMutex
}

func NewMemoryCheckQueue() *memoryCheckQueue {
	return &memoryCheckQueue{
		checks:      make(map[int64][]Check),
		priorities:  make(map[int64]int64),
		minPriority: math.MaxInt64,
	}
}

func (m *memoryCheckQueue) Enqueue(check Check) {
	priority := check.DueAt().Unix()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.checks[priority] = append(m.checks[priority], check)
	m.total++

	_, ok := m.priorities[priority]
	if !ok {
		m.priorities[priority] = priority
		m.minPriority = int64(math.Min(float64(priority), float64(m.minPriority)))
	}
}

func (m *memoryCheckQueue) Dequeue() *Check {
	m.mu.Lock()
	defer m.mu.Unlock()

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
git
func (m *memoryCheckQueue) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checks = make(map[int64][]Check)
	m.priorities = make(map[int64]int64)
	m.minPriority = math.MaxInt64
	m.total = 0
}

func (m *memoryCheckQueue) Count() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.total
}
