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
		MaxRunningChecks: 2,
	}, checkQueue)

	handleSignals(server)

	tenSecondPeriodic := gollector.PeriodicSchedule{Interval: 10}

	check := gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta: map[string]string{
			"check1": "check1",
		},
		Command:           command.DummyCommand{},
		CommandAttributes: nil,
		LastCheck:         nil,
		LastResult:        nil,
	}
	fmt.Println(check.IsDue())

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta: map[string]string{
			"check1": "check1",
		},
		Command:           command.DummyCommand{},
		CommandAttributes: nil,
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta: map[string]string{
			"check2": "check2",
		},
		Command:           command.DummyCommand{},
		CommandAttributes: nil,
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta: map[string]string{
			"check3": "check3",
		},
		Command:           command.DummyCommand{},
		CommandAttributes: nil,
		LastCheck:         nil,
		LastResult:        nil,
	})

	server.Run()

	fmt.Println("Exiting.")
}

func handleSignals(server *gollector.Server) {
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
					return
				}
			}
		}
	}()
}
