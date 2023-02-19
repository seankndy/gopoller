package gollector

import (
	"context"
	"sync"
	"time"
)

type Server struct {
	ctx           context.Context
	cancelContext func()
	checkQueue    CheckQueue

	// Should server re-enqueue checks back to the check queue after they finish running
	AutoReEnqueue bool

	// The maximum number of concurrently executing checks
	MaxRunningChecks uint64

	// Callback triggerred just prior to check execution (useful for logging)
	OnCheckExecuting func(check Check)

	// Callback triggerred if check command errors (useful for logging)
	OnCheckErrored func(check Check, err error)

	// Callback triggered just after a check finishes execution (useful for logging)
	OnCheckFinished func(check Check, runDuration time.Duration)
}

func NewServer(ctx context.Context, checkQueue CheckQueue) *Server {
	ctx, cancel := context.WithCancel(ctx)

	return &Server{
		ctx:              ctx,
		cancelContext:    cancel,
		checkQueue:       checkQueue,
		MaxRunningChecks: 100,
		AutoReEnqueue:    true,
	}
}

func (s *Server) Run() {
	runningLimiter := make(chan struct{}, s.MaxRunningChecks)
	defer close(runningLimiter)

	pendingChecks := make(chan *Check, s.MaxRunningChecks)
	go func() { // populate pendingCheck channel from queue indefinitely
		defer close(pendingChecks)

		for loop := true; loop; {
			select {
			case <-s.ctx.Done():
				loop = false
				break
			default:
			}

			if check := s.checkQueue.Dequeue(); check != nil {
				pendingChecks <- check
			} else {
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for loop := true; loop; {
		select {
		case <-s.ctx.Done():
			loop = false
			break
		case check := <-pendingChecks:
			runningLimiter <- struct{}{}

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if s.AutoReEnqueue {
						s.checkQueue.Enqueue(*check)
					}
					<-runningLimiter
				}()

				onCheckExecuting := s.OnCheckExecuting
				if onCheckExecuting != nil {
					onCheckExecuting(*check)
				}
				startTime := time.Now()
				if err := check.Execute(); err != nil {
					onCheckErrored := s.OnCheckErrored
					if onCheckErrored != nil {
						onCheckErrored(*check, err)
					}
				}
				onCheckFinished := s.OnCheckFinished
				if onCheckFinished != nil {
					onCheckFinished(*check, time.Now().Sub(startTime))
				}
			}()
		}
	}
	wg.Wait()

	if s.AutoReEnqueue {
		// put any pending checks back into the queue prior to shut down
		for check := range pendingChecks {
			s.checkQueue.Enqueue(*check)
		}
	}
}

func (s *Server) Stop() {
	s.cancelContext()
}
