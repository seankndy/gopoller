package rrdcached

import (
	"github.com/multiplay/go-rrd"
	"time"
)

// GoRrdDialer dials up a new GoRrdClient with each call to Dial()
type GoRrdDialer struct {
	Timeout time.Duration
}

func (d *GoRrdDialer) Dial(addr string) (Client, error) {
	var client *rrd.Client
	var err error

	if d.Timeout == 0 {
		d.Timeout = 10 * time.Second
	}

	if addr != "" && addr[:7] == "unix://" {
		client, err = rrd.NewClient(addr[7:], rrd.Timeout(d.Timeout), rrd.Unix)
	} else {
		client, err = rrd.NewClient(addr, rrd.Timeout(d.Timeout))
	}
	if err != nil {
		return nil, err
	}

	return &GoRrdClient{client}, nil
}

// GoRrdClient is an RRD client adapter for github.com/multiplay/go-rrd to Client interface
type GoRrdClient struct {
	Client *rrd.Client
}

func (c *GoRrdClient) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}

func (c *GoRrdClient) ExecCmd(cmd *Cmd) ([]string, error) {
	return c.Client.ExecCmd(c.convertCmd(cmd))
}

func (c *GoRrdClient) Batch(cmds ...*Cmd) error {
	convertedCmds := make([]*rrd.Cmd, len(cmds))
	for i, cmd := range cmds {
		convertedCmds[i] = c.convertCmd(cmd)
	}
	return c.Client.Batch(convertedCmds...)
}

func (c *GoRrdClient) Last(filename string) (time.Time, error) {
	return c.Client.Last(filename)
}

func (c *GoRrdClient) Create(filename string, ds []DS, rra []RRA, step time.Duration) error {
	convertedDS := make([]rrd.DS, len(ds))
	for i, v := range ds {
		convertedDS[i] = c.convertDS(v)
	}
	convertedRRA := make([]rrd.RRA, len(rra))
	for i, v := range rra {
		convertedRRA[i] = c.convertRRA(v)
	}
	return c.Client.Create(filename, convertedDS, convertedRRA, rrd.Step(step))
}

// convertCmd takes a Cmd from this package and converts it to a go-rrd rrd.Cmd
func (c *GoRrdClient) convertCmd(cmd *Cmd) *rrd.Cmd {
	return rrd.NewCmd(cmd.GetCmd()).WithArgs(cmd.GetArgs()...)
}

// convertDS takes a DS from this package and converts it to a go-rrd rrd.DS
func (c *GoRrdClient) convertDS(ds DS) rrd.DS {
	return rrd.NewDS(ds.String())
}

// convertRRA takes a RRA from this package and converts it to a go-rrd rrd.RRA
func (c *GoRrdClient) convertRRA(rra RRA) rrd.RRA {
	return rrd.NewRRA(rra.String())
}
