package gollector

import (
	"fmt"
	"time"
)

type Server struct {
	checkQueue CheckQueue
	// maxRunningChecks is the maximum number of concurrently executing checks
	maxRunningChecks uint64
	stop             chan struct{}
}

type ServerConfig struct {
	MaxRunningChecks uint64
}

func NewServer(config ServerConfig, checkQueue CheckQueue) *Server {
	return &Server{
		checkQueue:       checkQueue,
		maxRunningChecks: config.MaxRunningChecks,
	}
}

func (s *Server) Run() {
	s.stop = make(chan struct{})

	runningLimiter := make(chan struct{}, s.maxRunningChecks)
	defer close(runningLimiter)

	pendingChecks := make(chan *Check, s.maxRunningChecks)
	go func() { // populate pendingCheck channel from queue indefinitely
		defer close(pendingChecks)

		for loop := true; loop; {
			select {
			case <-s.stop:
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

	for loop := true; loop; {
		select {
		case <-s.stop:
			loop = false
			break
		case check := <-pendingChecks:
			runningLimiter <- struct{}{}

			go func() {
				defer func() {
					s.checkQueue.Enqueue(*check)
					<-runningLimiter
				}()

				if err := check.Execute(); err != nil {
					fmt.Printf("failed to execute check: %v\n", err)
				}

				if check.Incident != nil {
					fmt.Println(check.Incident)
				}

			}()
		}
	}

	// only get here when server stopped
	// put any pending checks back into the queue
	for check := range pendingChecks {
		s.checkQueue.Enqueue(*check)
	}

	fmt.Println("Server stopped")
}

func (s *Server) Stop() {
	close(s.stop)
}
