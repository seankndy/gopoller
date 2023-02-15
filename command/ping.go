package command

import (
	"fmt"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/seankndy/gollector"
	"strconv"
	"time"
)

type PingCommand struct {
	gollector.BaseCommand
}

func (c PingCommand) Run(attributes map[string]string) (gollector.Result, error) {
	attributes = c.MergeAttributes(map[string]string{
		"ip":       "127.0.0.1",
		"count":    "5",
		"interval": "150",
	}, attributes)

	pinger, err := probing.NewPinger(attributes["ip"])
	if err != nil {
		return gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	v, err := strconv.Atoi(attributes["interval"])
	if err != nil {
		return gollector.MakeUnknownResult("CMD_FAILURE"), nil
	}
	pinger.Interval = time.Duration(v) * time.Millisecond

	v, err = strconv.Atoi(attributes["count"])
	if err != nil {
		return gollector.MakeUnknownResult("CMD_FAILURE"), nil
	}
	pinger.Count = v

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
