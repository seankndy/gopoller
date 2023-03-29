package ping

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
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
	UseIcmp  bool

	PacketLossWarnThreshold float64
	PacketLossCritThreshold float64
	AvgRttWarnThreshold     time.Duration
	AvgRttCritThreshold     time.Duration
	StdDevRttWarnThreshold  time.Duration
	StdDevRttCritThreshold  time.Duration
}

func (c *Command) SetPinger(pinger Pinger) {
	c.pinger = pinger
}

var (
	DefaultPinger = &ProBingPinger{}
)

func (c *Command) Run(*check.Check) (*check.Result, error) {
	var pinger Pinger
	if c.pinger != nil {
		pinger = c.pinger
	} else {
		pinger = DefaultPinger
	}

	stats, err := pinger.Run(c)
	if err != nil {
		return check.MakeUnknownResult("CMD_FAILURE"), err
	}

	avgMs := float64(stats.AvgRtt.Microseconds()) / float64(time.Microsecond)
	jitterMs := float64(stats.StdDevRtt.Microseconds()) / float64(time.Microsecond)
	lossPerc := stats.PacketLoss

	var state check.ResultState
	var reasonCode string
	if lossPerc > c.PacketLossCritThreshold {
		state = check.StateCrit
		reasonCode = "PKT_LOSS_HIGH"
	} else if lossPerc > c.PacketLossWarnThreshold {
		state = check.StateWarn
		reasonCode = "PKT_LOSS_HIGH"
	} else if c.AvgRttCritThreshold > 0 && stats.AvgRtt > c.AvgRttCritThreshold {
		state = check.StateCrit
		reasonCode = "LATENCY_HIGH"
	} else if c.AvgRttWarnThreshold > 0 && stats.AvgRtt > c.AvgRttWarnThreshold {
		state = check.StateWarn
		reasonCode = "LATENCY_HIGH"
	} else if c.StdDevRttCritThreshold > 0 && stats.StdDevRtt > c.StdDevRttCritThreshold {
		state = check.StateCrit
		reasonCode = "JITTER_HIGH"
	} else if c.StdDevRttWarnThreshold > 0 && stats.StdDevRtt > c.StdDevRttWarnThreshold {
		state = check.StateWarn
		reasonCode = "JITTER_HIGH"
	} else {
		state = check.StateOk
	}

	return check.NewResult(state, reasonCode, []check.ResultMetric{
		{Label: "avg", Value: fmt.Sprintf("%.2f", avgMs)},
		{Label: "jitter", Value: fmt.Sprintf("%.2f", jitterMs)},
		{Label: "loss", Value: fmt.Sprintf("%.2f", lossPerc)},
	}), nil
}
