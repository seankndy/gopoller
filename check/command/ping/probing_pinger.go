package ping

import (
	probing "github.com/prometheus-community/pro-bing"
)

type ProBingPinger struct{}

func (p *ProBingPinger) Run(cmd *Command) (*PingerStats, error) {
	pinger, err := probing.NewPinger(cmd.Addr)
	if err != nil {
		return nil, err
	}

	pinger.Interval = cmd.Interval
	pinger.Count = cmd.Count
	pinger.Size = cmd.Size
	if cmd.UseIcmp {
		pinger.SetPrivileged(true)
	}

	err = pinger.Run()
	if err != nil {
		return nil, err
	}

	stats := pinger.Statistics()

	return &PingerStats{
		PacketLoss: stats.PacketLoss,
		AvgRtt:     stats.AvgRtt,
		StdDevRtt:  stats.StdDevRtt,
	}, nil
}
