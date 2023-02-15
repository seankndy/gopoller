package main

import (
	"fmt"
	"github.com/seankndy/gollector"
	"github.com/seankndy/gollector/command"
	"os"
	"os/signal"
	"syscall"
	"time"
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
		Command:        command.DummyCommand{Message: "check 1"},
		LastCheck:      nil,
		LastResult:     nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{Message: "check 2"},
		LastCheck:      nil,
		LastResult:     nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{Message: "check 3"},
		LastCheck:      nil,
		LastResult:     nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command:        command.DummyCommand{Message: "check 4"},
		LastCheck:      nil,
		LastResult:     nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:       &tenSecondPeriodic,
		SuppressAlerts: false,
		Meta:           nil,
		Command: command.PingCommand{
			Ip:                      "8.8.8.8",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     30,
			AvgRttCritThreshold:     50,
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
