package main

import (
	"context"
	"fmt"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/check/command/dns"
	"github.com/seankndy/gopoller/check/command/http"
	"github.com/seankndy/gopoller/check/command/junsubpool"
	"github.com/seankndy/gopoller/check/command/ping"
	"github.com/seankndy/gopoller/check/command/smtp"
	"github.com/seankndy/gopoller/check/command/snmp"
	"github.com/seankndy/gopoller/check/handler/dummy"
	"github.com/seankndy/gopoller/memqueue"
	"github.com/seankndy/gopoller/server"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	checkQueue := memqueue.NewQueue()

	lastCheck1 := time.Now().Add(-100 * time.Second)
	check1 := check.New(
		"check1",
		check.WithCommand(&ping.Command{
			Addr:                    "209.193.82.100",
			Count:                   5,
			Interval:                100 * time.Millisecond,
			Size:                    64,
			PacketLossWarnThreshold: 90,
			PacketLossCritThreshold: 95,
			AvgRttWarnThreshold:     20 * time.Millisecond,
			AvgRttCritThreshold:     50 * time.Millisecond,
		}),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	check1.LastCheck = &lastCheck1
	checkQueue.Enqueue(check1)

	lastCheck2 := time.Now().Add(-90 * time.Second)
	check2 := check.New(
		"check2",
		check.WithCommand(snmp.NewCommand("209.193.82.100", "public", []snmp.OidMonitor{
			*snmp.NewOidMonitor(".1.3.6.1.2.1.2.2.1.7.554", "ifAdminStatus"),
		})),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	check2.LastCheck = &lastCheck2
	checkQueue.Enqueue(check2)

	check3 := check.New(
		"check3",
		check.WithCommand(&dns.Command{
			ServerIp:              "209.193.72.2",
			ServerPort:            53,
			ServerTimeout:         3 * time.Second,
			Query:                 "www.vcn.com",
			QueryType:             dns.Host,
			Expected:              &[]string{"209.193.72.54"},
			WarnRespTimeThreshold: 20 * time.Millisecond,
			CritRespTimeThreshold: 40 * time.Millisecond,
		}),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	checkQueue.Enqueue(check3)

	check4 := check.New(
		"check4",
		check.WithCommand(&smtp.Command{
			Addr:                  "smtp.vcn.com",
			Port:                  25,
			Timeout:               3 * time.Second,
			WarnRespTimeThreshold: 25 * time.Millisecond,
			CritRespTimeThreshold: 50 * time.Millisecond,
			Send:                  "HELO gopoller.local",
			ExpectedResponseCode:  250,
		}),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	checkQueue.Enqueue(check4)

	check5 := check.New(
		"check5",
		check.WithCommand(junsubpool.NewCommand("209.193.82.44", "public", []int{1000002, 1000003, 1000004, 1000005, 1000006, 1000007, 1000008, 1000012, 1000015, 1000017, 1000019}, 95, 99)),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	checkQueue.Enqueue(check5)

	check6 := check.New(
		"check6",
		check.WithCommand(&http.Command{
			ReqUrl:                "https://www.google.com",
			ReqMethod:             "GET",
			ReqTimeout:            1000 * time.Millisecond,
			SkipSslVerify:         false,
			ExpectedResponseCode:  200,
			WarnRespTimeThreshold: 250 * time.Millisecond,
			CritRespTimeThreshold: 500 * time.Millisecond,
		}),
		check.WithPeriodicSchedule(10),
		check.WithHandlers([]check.Handler{&dummy.Handler{}}),
	)
	checkQueue.Enqueue(check6)

	// create and run the server
	svr := server.New(checkQueue, server.WithMaxRunningChecks(2))
	svr.OnCheckExecuting = func(chk *check.Check) {
		//fmt.Printf("Check beginning execution: %v\n", check)
	}
	svr.OnCheckErrored = func(chk *check.Check, err error) {
		fmt.Printf("CHECK ERROR: %v", err)
	}
	svr.OnCheckFinished = func(chk *check.Check, runDuration time.Duration) {
		fmt.Printf("Check finished execution: %v (%.3f seconds)\n", chk, runDuration.Seconds())
	}
	svr.Run(ctx)

	fmt.Println("Exiting.")
}
