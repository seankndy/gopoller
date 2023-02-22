package main

import (
	"context"
	"fmt"
	"github.com/seankndy/gollector"
	"github.com/seankndy/gollector/command/dns"
	"github.com/seankndy/gollector/command/ping"
	"github.com/seankndy/gollector/command/smtp"
	"github.com/seankndy/gollector/command/snmp"
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
	server.OnCheckErrored = func(check gollector.Check, err error) {
		fmt.Printf("CHECK ERROR: %v", err)
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
		Meta:              map[string]string{"check1": "check1"},
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

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              map[string]string{"check2": "check2"},
		Command: snmp.NewCommand("209.193.82.100", "public", []snmp.OidMonitor{
			*snmp.NewOidMonitor(".1.3.6.1.2.1.2.2.1.7.554", "ifAdminStatus"),
		}),
		Handlers: []gollector.Handler{
			dummy2.Handler{},
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              map[string]string{"check3": "check3"},
		Command: &dns.Command{
			ServerIp:              "209.193.72.2",
			ServerPort:            53,
			ServerTimeout:         3 * time.Second,
			Query:                 "www.vcn.com",
			QueryType:             dns.Host,
			Expected:              &[]string{"209.193.72.54"},
			WarnRespTimeThreshold: 20 * time.Millisecond,
			CritRespTimeThreshold: 40 * time.Millisecond,
		},
		Handlers: []gollector.Handler{
			dummy2.Handler{},
		},
		LastCheck:  nil,
		LastResult: nil,
	})

	checkQueue.Enqueue(gollector.Check{
		Schedule:          &tenSecondPeriodic,
		SuppressIncidents: false,
		Meta:              map[string]string{"check4": "check4"},
		Command: &smtp.Command{
			Addr:                  "smtp.vcn.com",
			Port:                  25,
			Timeout:               5 * time.Second,
			WarnRespTimeThreshold: 25 * time.Millisecond,
			CritRespTimeThreshold: 50 * time.Millisecond,
			Send:                  "HELO gollector.local",
			ExpectedResponseCode:  250,
		},
		Handlers: []gollector.Handler{
			dummy2.Handler{},
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
					fmt.Println("Stopping...")
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
