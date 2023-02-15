package gollector

import (
	"fmt"
	"time"
)

type Server struct {
	checkQueue     CheckQueue
	runningLimiter chan struct{}
	pendingChecks  chan *Check
	stop           chan struct{}
}

type ServerConfig struct {
	MaxRunningChecks uint64
}

func NewServer(config ServerConfig, checkQueue CheckQueue) *Server {
	return &Server{
		checkQueue:     checkQueue,
		runningLimiter: make(chan struct{}, config.MaxRunningChecks),
		pendingChecks:  make(chan *Check, config.MaxRunningChecks),
		stop:           make(chan struct{}),
	}
}

func (s *Server) Run() {
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

			check, err := s.checkQueue.Dequeue()
			if err != nil {
				fmt.Printf("failed to dequeue check: %v\n", err)
				time.Sleep(1000 * time.Millisecond)
				continue
			}

			if check != nil {
				s.pendingChecks <- check
			}

			time.Sleep(250 * time.Millisecond)
		}

		close(s.pendingChecks)
	}()

	for loop := true; loop; {
		select {
		case _, ok := <-s.stop:
			if !ok {
				loop = false
				break
			}
		case check := <-s.pendingChecks:
			s.runningLimiter <- struct{}{}

			go func() {
				defer func() {
					if check != nil {
						s.checkQueue.Enqueue(*check)
					}

					<-s.runningLimiter
				}()

				result, err := check.Execute()
				if err != nil {
					fmt.Printf("failed to execute check: %v\n", err)
					return
				}

				fmt.Println(result)
			}()
		default:
		}

		time.Sleep(250 * time.Millisecond)
	}

	// only get here when server stopped
	// put any pending checks back in the queue
	for check := range s.pendingChecks {
		s.checkQueue.Enqueue(*check)
	}
	// now flush the queue prior to shut down
	s.checkQueue.Flush()

	fmt.Println("Server stopped")
}

func (s *Server) Stop() {
	close(s.stop)
}
