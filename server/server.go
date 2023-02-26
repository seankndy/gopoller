package server

import (
	"context"
	"github.com/seankndy/gopoller/check"
	"sync"
	"time"
)

type Server struct {
	checkQueue check.Queue

	// Should server re-enqueue checks back to the check queue after they finish running
	AutoReEnqueue bool

	// The maximum number of concurrently executing checks
	MaxRunningChecks int

	// Callback triggerred just prior to check execution (useful for logging)
	OnCheckExecuting func(check check.Check)

	// Callback triggerred if check command errors (useful for logging)
	OnCheckErrored func(check check.Check, err error)

	// Callback triggered just after a check finishes execution (useful for logging)
	OnCheckFinished func(check check.Check, runDuration time.Duration)
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

func (s *Server) Run(ctx context.Context) {
	runningLimiter := make(chan struct{}, s.MaxRunningChecks)
	defer close(runningLimiter)

	pendingChecks := make(chan *check.Check, s.MaxRunningChecks)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // populate pendingCheck channel from queue indefinitely
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
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()

loop:
	for chk := range pendingChecks {
		select {
		case <-ctx.Done():
			s.checkQueue.Enqueue(*chk) // put the check back, we're shutting down
			break loop
		case runningLimiter <- struct{}{}:
			wg.Add(1)
			go func(chk *check.Check) {
				defer wg.Done()
				defer func() {
					if s.AutoReEnqueue {
						s.checkQueue.Enqueue(*chk)
					}
					<-runningLimiter
				}()

				onCheckExecuting := s.OnCheckExecuting
				if onCheckExecuting != nil {
					onCheckExecuting(*chk)
				}
				startTime := time.Now()
				if err := chk.Execute(); err != nil {
					onCheckErrored := s.OnCheckErrored
					if onCheckErrored != nil {
						onCheckErrored(*chk, err)
					}
				}
				onCheckFinished := s.OnCheckFinished
				if onCheckFinished != nil {
					onCheckFinished(*chk, time.Now().Sub(startTime))
				}
			}(chk)
		}
	}

	wg.Wait()

	// put any pending checks back into the queue prior to shut down as they never ran
	for chk := range pendingChecks {
		s.checkQueue.Enqueue(*chk)
	}
}
