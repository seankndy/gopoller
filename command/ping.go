package command

import (
	"fmt"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/seankndy/gollector"
	"time"
)

type PingCommand struct {
	Ip                      string
	Count                   int
	Interval                time.Duration
	Size                    int
	PacketLossWarnThreshold float64
	PacketLossCritThreshold float64
	AvgRttWarnThreshold     float64
	AvgRttCritThreshold     float64
}

func (c PingCommand) Run() (gollector.Result, error) {
	pinger, err := probing.NewPinger(c.Ip)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	pinger.Interval = c.Interval
	pinger.Count = c.Count
	pinger.Size = c.Size
	err = pinger.Run()
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	stats := pinger.Statistics()

	avgMs := float64(stats.AvgRtt.Microseconds()) / 1000
	jitterMs := float64(stats.StdDevRtt.Microseconds()) / 1000
	lossPerc := stats.PacketLoss

	var state gollector.ResultState
	var reasonCode string
	if lossPerc >= c.PacketLossCritThreshold {
		state = gollector.StateCrit
		reasonCode = "PKT_LOSS_HIGH"
	} else if lossPerc >= c.PacketLossWarnThreshold {
		state = gollector.StateWarn
		reasonCode = "PKT_LOSS_HIGH"
	} else if avgMs >= c.AvgRttCritThreshold {
		state = gollector.StateCrit
		reasonCode = "LATENCY_HIGH"
	} else if avgMs >= c.AvgRttWarnThreshold {
		state = gollector.StateWarn
		reasonCode = "LATENCY_HIGH"
	} else {
		state = gollector.StateOk
	}

	return gollector.Result{
		State:      state,
		ReasonCode: reasonCode,
		Metrics: []gollector.ResultMetric{
			{Label: "avg", Value: fmt.Sprintf("%.2f", avgMs)},
			{Label: "jitter", Value: fmt.Sprintf("%.2f", jitterMs)},
			{Label: "loss", Value: fmt.Sprintf("%.2f", lossPerc)},
		},
		Time: time.Now(),
	}, nil
}
