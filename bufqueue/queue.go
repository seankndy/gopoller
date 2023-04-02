package bufqueue

import (
	"context"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/memqueue"
	"sync"
	"time"
)

type CheckProvider interface {
	// Provide is responsible for fetching check.Check structs from a backend and returning them as a slice of pointers.
	Provide() []*check.Check
}

type CheckEnqueuer interface {
	// Enqueue receives a slice of pointers to check.Check structs and is responsible for storing those checks
	// into a backend.
	Enqueue([]*check.Check)
}

// Queue is a buffered check.Queue.  Enqueue() stores checks in a buffer that is processed by a separate goroutine
// every N seconds by the CheckEnqueuer.  Dequeue() returns checks directly from a buffer.  If the buffer is empty, it
// is filled by the CheckProvider.  By implementing a CheckEnqueuer and CheckProvider, this bufqueue abstracts the
// details of managing the queue from whatever your 'Check' backend is.
//
// The outgoing queue/buffer size is controlled by the number of Checks are returned from the CheckProvider.
// The incoming queue/buffer size is controlled by the enqueuer interval duration.
type Queue struct {
	// queue is a memqueue.Queue that outgoing checks are stored in
	queue *memqueue.Queue

	// pendingEnqueued is a slice of pointers to check.Check structs that are awaiting official enquement by the
	// CheckEnqueuer.
	pendingEnqueued []*check.Check
	// mu guards pendingEnqueued
	mu sync.Mutex

	// CheckProvider provides the check.Check structs when the outgoing check.Check queue/buffer is empty during a
	// Dequeue() call.
	CheckProvider CheckProvider

	// CheckEnqueuer is responsible for "enqueueing" the check.Check structs into their permanent backend.  This means
	// the last check times, result, metrics and incident would be saved so that in a future CheckProvider.Provide()
	// call, the Check can be reconstructed to be executed once again.
	CheckEnqueuer CheckEnqueuer

	// CheckEnqueuerInterval is a time.Duration indicating how often to execute the CheckEnqueuer.
	CheckEnqueuerInterval time.Duration
}

func NewQueue(
	checkProvider CheckProvider,
	checkEnqueuer CheckEnqueuer,
	checkEnqueuerInterval time.Duration,
	ctx context.Context,
) *Queue {
	q := &Queue{
		queue:                 memqueue.NewQueue(),
		CheckProvider:         checkProvider,
		CheckEnqueuer:         checkEnqueuer,
		CheckEnqueuerInterval: checkEnqueuerInterval,
	}

	q.runCheckEnqueuer(ctx)

	return q
}

func (q *Queue) Enqueue(chk *check.Check) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pendingEnqueued = append(q.pendingEnqueued, chk)
}

func (q *Queue) Dequeue() *check.Check {
	if q.queue.Count() == 0 { // underlying memqueue is empty, so fill it with checks from the provider
		for _, chk := range q.CheckProvider.Provide() {
			q.queue.Enqueue(chk)
		}
	}

	return q.queue.Dequeue()
}

func (q *Queue) Count() uint64 {
	return q.queue.Count()
}

func (q *Queue) Flush() {
	// enqueue the pending-enqueuement checks
	q.enqueuePending()
	// enqueue the checks in the underlying queue that never ran
	chks := q.queue.All()
	q.queue.Flush()
	if len(chks) > 0 {
		q.CheckEnqueuer.Enqueue(chks)
	}
}

// runCheckEnqueuer kicks off goroutine to periodically call the CheckEnqueuer
func (q *Queue) runCheckEnqueuer(ctx context.Context) {
	ticker := time.NewTicker(q.CheckEnqueuerInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				q.enqueuePending()
			}
		}
	}()
}

func (q *Queue) enqueuePending() {
	q.mu.Lock()
	chks := q.pendingEnqueued
	if len(chks) > 0 {
		// clear pendingEnqueued by creating a new slice allocation. pre-allocate half the checks we just
		// had to reduce memory allocations required as the queue fills
		q.pendingEnqueued = make([]*check.Check, 0, len(chks)/2)
	}
	q.mu.Unlock()

	if len(chks) > 0 {
		q.CheckEnqueuer.Enqueue(chks)
	}
}
