package main

import (
	"context"
	"fmt"
	"github.com/seankndy/gollector"
	"github.com/seankndy/gollector/command/dummy"
	"github.com/seankndy/gollector/command/ping"
	dummy2 "github.com/seankndy/gollector/handler/dummy"
	"github.com/seankndy/gollector/handler/rrdcached"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	checkQueue := gollector.NewMemoryCheckQueue()
	server := gollector.NewServer(context.Background(), checkQueue)
	server.MaxRunningChecks = 3
	server.AutoReEnqueue = true
	server.OnCheckExecuting = func(check gollector.Check) {
		fmt.Printf("Check beginning execution: %v\n", check)
	}
	server.OnCheckFinished = func(check gollector.Check, runDuration time.Duration) {
		fmt.Printf("Check finished execution: %v (%.3f seconds)\n", check, runDuration.Seconds())
	}

	// signal handler
	handleSignals(server, checkQueue)

	tenSecondPeriodic := gollector.PeriodicSchedule{IntervalSeconds: 10}

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command:           dummy.Command{Message: "check 1"},
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command:           dummy.Command{Message: "check 2"},
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command:           dummy.Command{Message: "check 3"},
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command:           dummy.Command{Message: "check 4"},
		LastCheck:         nil,
		LastResult:        nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              nil,
		Command: ping.Command{
			Ip:                      "209.193.82.100",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     1,
			AvgRttCritThreshold:     50,
		},
		Handlers: []gollector.Handler{
			dummy2.Handler{},
		},
		LastCheck:  nil,
		LastResult: nil,
	})
	/*
		checkQueue.Enqueue(gollector.Check{
			Schedule:          &tenSecondPeriodic,
			SuppressIncidents: false,
			Meta:              nil,
			Command: snmp.Command{
				Ip:        "209.193.82.100",
				Community: "public",
				Version:   "2c",
				Oids:      []string{"1.3.6.1.2.1.2.2.1.7.554"},
			},
			Handlers: []gollector.Handler{
				dummy2.Handler{},
			},
			LastCheck:  nil,
			LastResult: nil,
		})
	*/
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

// example getRrdFileDefs func:
func getRrdFileDefs(check gollector.Check, result gollector.Result) []rrdcached.RrdFileDef {
	_, isPeriodic := check.Schedule.(gollector.PeriodicSchedule)
	// no spec if no metrics or if the underlying check isn't on an interval schedule
	if result.Metrics == nil || !isPeriodic {
		return nil
	}

	interval := check.Schedule.(gollector.PeriodicSchedule).IntervalSeconds

	var rrdFileDefs []rrdcached.RrdFileDef
	for _, metric := range result.Metrics {
		var dst rrdcached.DST
		if metric.Type == gollector.ResultMetricCounter {
			dst = rrdcached.Counter
		} else {
			dst = rrdcached.Gauge
		}
		ds := rrdcached.NewDS(metric.Label, dst, interval*2, "U", "U")

		weeklyAvg := 1800
		monthlyAvg := 7200
		yearlyAvg := 43200

		rrdFileDefs = append(rrdFileDefs, rrdcached.RrdFileDef{
			Filename:    "/Users/sean/rrd_test/" + check.Id + "/" + ds.Name(),
			DataSources: []rrdcached.DS{ds},
			RoundRobinArchives: []rrdcached.RRA{
				rrdcached.NewMinRRA(0.5, 1, 86400/interval),
				rrdcached.NewMinRRA(0.5, weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval)),
				rrdcached.NewMinRRA(0.5, monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval)),
				rrdcached.NewMinRRA(0.5, yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval)),

				rrdcached.NewAverageRRA(0.5, 1, 86400/interval),
				rrdcached.NewAverageRRA(0.5, weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval)),
				rrdcached.NewAverageRRA(0.5, monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval)),
				rrdcached.NewAverageRRA(0.5, yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval)),

				rrdcached.NewMaxRRA(0.5, 1, 86400/interval),
				rrdcached.NewMaxRRA(0.5, weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval)),
				rrdcached.NewMaxRRA(0.5, monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval)),
				rrdcached.NewMaxRRA(0.5, yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval)),
			},
			Step: time.Duration(interval) * time.Second,
		})
	}
	return rrdFileDefs
}
