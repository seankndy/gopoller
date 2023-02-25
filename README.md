# gopoller
gopoller is a network monitoring framework written in Go.  It provides the base set of functionality for you to easily create your own Go-based host/network checks (icmp, snmp, etc) and then mutate and process the result data into whatever format and system you'd like. 
## Basic Usage
Here is a basic example showcasing how to bootstrap the system and start running checks.

```go
package main

import (
	"fmt"
	"github.com/seankndy/gopoller"
	"github.com/seankndy/gopoller/command/ping"
	"github.com/seankndy/gopoller/handler/dummy"
)

func main() {
	// this is where you will store all the checks you want to periodically execute
	// you could write your own check queue as well (just implement the CheckQueue interface)
	checkQueue := gopoller.NewMemoryCheckQueue()
	// creates the server
	server := gopoller.NewServer(checkQueue)
	// here we set the max running checks to 2, you probably want something much higher
	server.MaxRunningChecks = 2
	// here we tell the server to auto-reenqueue the check after its done executing
	// alternatively you could set this to false and then use your own system for populating checkQueue
	server.AutoReEnqueue = true
	// here we have a couple callbacks to do some rudimentary logging when check start and finish
	server.OnCheckExecuting = func(check gopoller.Check) {
		fmt.Printf("Check beginning execution: %v\n", check)
	}
	server.OnCheckFinished = func(check gopoller.Check, runDuration time.Duration) {
		fmt.Printf("Check finished execution: %v (%.3f seconds)\n", check, runDuration.Seconds())
	}

	// this is a periodic schedule that i will reuse for every check i put into the queue
	// in the real-world, your checks will probably have varying intervals
	tenSecondPeriodic := gopoller.PeriodicSchedule{IntervalSeconds: 10}

	// queue up a ping couple checks.  these checks would normally come from your own database
	// and be populated programmatically
	checkQueue.Enqueue(gopoller.Check{
		Schedule:          &tenSecondPeriodic,
		// if false, the system will never create incidents for this check
		SuppressIncidents: false,
		// meta is a map[string]string to store extra data to carry along with the check
		Meta:              nil,
		// this is the command to execute along with several self-explanatory parameters
		Command: ping.Command{
			Ip:                      "8.8.8.8",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     20,
			AvgRttCritThreshold:     50,
		},
		// these are the handlers that will be called once the check finishes
		Handlers: []gopoller.Handler{
			handler.DummyHandler{},
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gopoller.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command: ping.Command{
			Ip:                      "1.1.1.1",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     20,
			AvgRttCritThreshold:     50,
		},
		Handlers: []gopoller.Handler{
			dummy.Handler{},
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	// runs forever
	server.Run()
}
```
Check commands return Results with states of either Unknown, Ok, Warn or Crit.  If a check moves from being ok to non-ok or from being non-ok to some other non-ok, then a new Incident is generated for that Check.  This Incident (or nil) along with the Check and Result are passed to the handlers for mutation and processing.