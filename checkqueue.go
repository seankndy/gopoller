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

// CachingCheckQueue is a CheckQueue that fills itself with Checks from an external source when
// the queue is empty and Dequeue() is called. Subsequent Dequeue() calls draw from this cache.
// When Checks are Enqueue()'d, they are not added back into the queue, but placed in a separate
// cache that is flushed at a given interval or when full, whichever comes first.
//
// This allows you to provide a few methods to handle reading and writing your checks from/to any external
// database without having to worry about managing the queue logic itself.  It also provides the means
// to not hit your database with every enqueue and dequeue operation.  Your job is to implement methods to
// fill checks: query out the Checks, build the Check structs, and return them
// write checks: take a slice of Checks, write them as efficiently as possible
type CachingCheckQueue struct {
	// WriteCacheSize is the number of Checks to cache prior to writing
	WriteCacheSize uint64
	// WriteCacheIntervalSeconds is the number of seconds before we write
	WriteCacheIntervalSeconds uint64
	// Where is the read cache size?  That's up you -- however many checks you return during the fill
	// cycle is how many this will cache.
}
