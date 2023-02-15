package gollector

import (
	"fmt"
	"time"
)

type Server struct {
	checkQueue       CheckQueue
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

	pendingChecks := make(chan *Check, s.maxRunningChecks)
	runningLimiter := make(chan struct{}, s.maxRunningChecks)

	// populate pendingCheck channel from queue indefinitely
	go func() {
		for loop := true; loop; {
			select {
			case _, ok := <-s.stop:
				if !ok {
					loop = false
					break
				}
			default:
			}

			if check := s.checkQueue.Dequeue(); check != nil {
				pendingChecks <- check
			}

			time.Sleep(250 * time.Millisecond)
		}

		close(pendingChecks)
	}()

	for loop := true; loop; {
		select {
		case _, ok := <-s.stop:
			if !ok {
				loop = false
				break
			}
		case check := <-pendingChecks:
			runningLimiter <- struct{}{}

			go func() {
				defer func() {
					if check != nil {
						s.checkQueue.Enqueue(*check)
					}

					<-runningLimiter
				}()

				result, err := check.Execute()
				if err != nil {
					fmt.Printf("failed to execute check: %v\n", err)
					return
				}

				fmt.Println(result)
			}()
		}
	}
	close(runningLimiter)

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
