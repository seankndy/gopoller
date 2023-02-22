package ping

import (
	"fmt"
	"github.com/seankndy/gollector"
	"time"
)

type Pinger interface {
	Run(*Command) (*PingerStats, error)
}

type PingerStats struct {
	// PacketLoss is the percentage of packets lost
	PacketLoss float64

	// AvgRtt is the average round-trip time
	AvgRtt time.Duration

	// StdDevRtt is the standard deviation of the round-trip times
	StdDevRtt time.Duration
}

type Command struct {
	pinger Pinger

	Addr     string
	Count    int
	Interval time.Duration
	Size     int

	PacketLossWarnThreshold float64
	PacketLossCritThreshold float64
	AvgRttWarnThreshold     time.Duration
	AvgRttCritThreshold     time.Duration
}

func (c *Command) SetPinger(pinger Pinger) {
	c.pinger = pinger
}

var (
	DefaultPinger = &ProBingPinger{}
)

func (c *Command) Run(gollector.Check) (gollector.Result, error) {
	var pinger Pinger
	if c.pinger != nil {
		pinger = c.pinger
	} else {
		pinger = DefaultPinger
	}

	stats, err := pinger.Run(c)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	avgMs := float64(stats.AvgRtt.Microseconds()) / float64(time.Microsecond)
	jitterMs := float64(stats.StdDevRtt.Microseconds()) / float64(time.Microsecond)
	lossPerc := stats.PacketLoss

	var state gollector.ResultState
	var reasonCode string
	if lossPerc > c.PacketLossCritThreshold {
		state = gollector.StateCrit
		reasonCode = "PKT_LOSS_HIGH"
	} else if lossPerc > c.PacketLossWarnThreshold {
		state = gollector.StateWarn
		reasonCode = "PKT_LOSS_HIGH"
	} else if stats.AvgRtt > c.AvgRttCritThreshold {
		state = gollector.StateCrit
		reasonCode = "LATENCY_HIGH"
	} else if stats.AvgRtt > c.AvgRttWarnThreshold {
		state = gollector.StateWarn
		reasonCode = "LATENCY_HIGH"
	} else {
		state = gollector.StateOk
	}

	return *gollector.NewResult(state, reasonCode, []gollector.ResultMetric{
		{Label: "avg", Value: fmt.Sprintf("%.2f", avgMs)},
		{Label: "jitter", Value: fmt.Sprintf("%.2f", jitterMs)},
		{Label: "loss", Value: fmt.Sprintf("%.2f", lossPerc)},
	}), nil
}
