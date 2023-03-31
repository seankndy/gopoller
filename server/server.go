package server

import (
	"context"
	"fmt"
	"github.com/seankndy/gopoller/check"
	"os"
	"sync"
	"time"
)

// Server loops forever, reading Checks from a check.Queue and executing them.
type Server struct {
	checkQueue check.Queue

	// Should server re-enqueue checks back to the check queue after they finish running
	AutoReEnqueue bool

	// The maximum number of concurrently executing checks
	MaxRunningChecks int

	// Callback triggerred just prior to check execution (useful for logging)
	OnCheckExecuting func(chk *check.Check)

	// Callback triggerred if check command errors (useful for logging)
	OnCheckErrored func(chk *check.Check, err error)

	// Callback triggered just after a check finishes execution (useful for logging)
	OnCheckFinished func(chk *check.Check, runDuration time.Duration)
}

type Option func(*Server)

func New(checkQueue check.Queue, options ...Option) *Server {
	server := &Server{
		checkQueue:       checkQueue,
		MaxRunningChecks: 100,
		AutoReEnqueue:    true,
	}

	for _, option := range options {
		option(server)
	}

	return server
}

func WithoutAutoReEnqueue() Option {
	return func(s *Server) {
		s.AutoReEnqueue = false
	}
}

func WithMaxRunningChecks(n int) Option {
	return func(s *Server) {
		s.MaxRunningChecks = n
	}
}

// Run starts the server.  ctx is a context.Context that when cancelled will
// stop the server after the currently executing checks finish.
func (s *Server) Run(ctx context.Context) {
	runningLimiter := make(chan struct{}, s.MaxRunningChecks)
	defer close(runningLimiter)

	pendingChecks := make(chan *check.Check, s.MaxRunningChecks)

	var wg sync.WaitGroup

	// launch goroutine that populates pendingCheck channel from queue indefinitely
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(pendingChecks)

		// we have this in its own goroutine in case checkQueue.Dequeue() takes some time to return and the main
		// check loop (below) is blocking by the runningLimiter.  this allows us to continue to fill the pending
		// checks queue and make the system more responsive

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var chk *check.Check
			if len(pendingChecks) < cap(pendingChecks) {
				chk = s.checkQueue.Dequeue()
				if chk != nil {
					pendingChecks <- chk
				}
			}

			if chk == nil {
				time.Sleep(1000 * time.Millisecond)
			}
		}
	}()

	runningChecks := sync.Map{}
	longRunningTicker := time.NewTicker(60 * time.Second)
	defer longRunningTicker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case chk := <-pendingChecks:
			runningLimiter <- struct{}{}
			runningChecks.Store(chk.Id, time.Now())

			wg.Add(1)
			go func(chk *check.Check) {
				defer wg.Done()
				defer func() {
					if s.AutoReEnqueue {
						s.checkQueue.Enqueue(chk)
					}

					<-runningLimiter
					runningChecks.Delete(chk.Id)
				}()

				onCheckExecuting := s.OnCheckExecuting
				if onCheckExecuting != nil {
					onCheckExecuting(chk)
				}
				startTime := time.Now()
				if err := chk.Execute(); err != nil {
					onCheckErrored := s.OnCheckErrored
					if onCheckErrored != nil {
						onCheckErrored(chk, err)
					}
				}
				onCheckFinished := s.OnCheckFinished
				if onCheckFinished != nil {
					onCheckFinished(chk, time.Now().Sub(startTime))
				}
			}(chk)
		case <-longRunningTicker.C:
			runningChecks.Range(func(key, value interface{}) bool {
				id := key.(string)
				t := value.(time.Time)
				if execTime := time.Now().Sub(t); execTime > 30*time.Second {
					fmt.Fprintf(os.Stderr, "WARNING: Check with ID %s has been executing for >30sec (%d seconds)!\n", id, execTime/time.Second)
				}
				return true
			})
		}
	}

	wg.Wait()

	// put any pending checks back into the queue prior to shut down as they never ran
	for chk := range pendingChecks {
		s.checkQueue.Enqueue(chk)
	}
}
