# gopoller
gopoller is a network monitoring framework written in Go.  It provides the base set of functionality for you to easily create your own Go-based host/network checks (icmp, snmp, etc) and then mutate and process the result data into whatever format and system you'd like. 
## Basic Usage
Here is a basic example showcasing how to bootstrap the system and start running checks.

```go
package main

import (
	"context"
	"fmt"
	"github.com/seankndy/gopoller/server"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/command/ping"
	"github.com/seankndy/gopoller/handler/dummy"
)

func main() {
	// this is where you will store all the checks you want to periodically execute
	// you could write your own check queue as well (just implement the CheckQueue interface)
	checkQueue := check.NewMemoryCheckQueue()
	// creates the server with max running checks of 3
	svr := server.New(checkQueue, server.WithMaxRunningChecks(3))
	// here we have a couple callbacks to do some rudimentary logging when check start and finish
	svr.OnCheckExecuting = func(chk gopoller.Check) {
		fmt.Printf("Check beginning execution: %v\n", chk)
	}
	svr.OnCheckFinished = func(chk gopoller.Check, runDuration time.Duration) {
		fmt.Printf("Check finished execution: %v (%.3f seconds)\n", chk, runDuration.Seconds())
	}
	
	// queue up a ping couple checks.  these checks would normally come from your own database
	// and be populated programmatically
	checkQueue.Enqueue(*check.NewCheck(
		"check1",
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]gopoller.Handler{
			handler.DummyHandler{},
		}),
		check.WithCommand(&ping.Command{
			Ip:                      "8.8.8.8",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     20,
			AvgRttCritThreshold:     50,
		}),
	))

	checkQueue.Enqueue(*check.NewCheck(
		"check2",
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]gopoller.Handler{
			handler.DummyHandler{},
		}),
		check.WithCommand(&ping.Command{
			Ip:                      "1.1.1.1",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     20,
			AvgRttCritThreshold:     50,
		}),
	))

	// runs forever
	svr.Run(context.Background())
}
```
Check commands return Results with states of either Unknown, Ok, Warn or Crit.  If a check moves from being ok to non-ok or from being non-ok to some other non-ok, then a new Incident is generated for that Check.  This Incident (or nil) along with the Check and Result are passed to the handlers for mutation and processing.