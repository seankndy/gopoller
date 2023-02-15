package main

import (
	"fmt"
	"github.com/seankndy/gollector"
	"github.com/seankndy/gollector/command"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	checkQueue := gollector.NewMemoryCheckQueue()
	server := gollector.NewServer(gollector.ServerConfig{
		MaxRunningChecks: 3,
	}, checkQueue)

	handleSignals(server, checkQueue)

	tenSecondPeriodic := gollector.PeriodicSchedule{Interval: 10}

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{},
		CommandAttributes: map[string]string{
			"check1": "check1",
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{},
		CommandAttributes: map[string]string{
			"check2": "check2",
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{},
		CommandAttributes: map[string]string{
			"check3": "check3",
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{},
		CommandAttributes: map[string]string{
			"check4": "check4",
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.PingCommand{},
		CommandAttributes: map[string]string{
			"ip":       "209.193.82.100",
			"interval": "100",
			"count":    "10",
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	server.Run()

	fmt.Println("Exiting.")
}

func handleSignals(server *gollector.Server, checkQueue gollector.CheckQueue) {
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT)

		defer func() {
			signal.Stop(sigCh)
			close(sigCh)
		}()

		for {
			select {
			case sig := <-sigCh:
				if sig == syscall.SIGINT {
					server.Stop()
					// now flush the queue prior to shut down
					checkQueue.Flush()
					return
				}
			}
		}
	}()
}
