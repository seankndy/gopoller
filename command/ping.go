package command

import (
	"fmt"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/seankndy/gollector"
	"time"
)

type PingCommand struct{}

func (c PingCommand) Run(check gollector.Check) (gollector.Result, error) {
	pinger, err := probing.NewPinger("8.8.8.8")
	if err != nil {
		return gollector.Result{
			State:      gollector.StateUnknown,
			ReasonCode: "",
			Metrics:    nil,
		}, err
	}
	pinger.Interval = 250 * time.Millisecond
	pinger.Count = 5
	pinger.Run()
	stats := pinger.Statistics()

	return gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics: []gollector.ResultMetric{
			{Label: "avg", Value: fmt.Sprintf("%.2f", float64(stats.AvgRtt.Microseconds())/1000)},
			{Label: "jitter", Value: fmt.Sprintf("%.2f", float64(stats.StdDevRtt.Microseconds())/1000)},
			{Label: "loss", Value: fmt.Sprintf("%.2f", stats.PacketLoss)},
		},
	}, nil
}
