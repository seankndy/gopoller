package rrdcached

import (
	"github.com/multiplay/go-rrd"
	"time"
)

// GoRrdClient is an RRD client adapter for github.com/multiplay/go-rrd to Client interface
type GoRrdClient struct {
	client  *rrd.Client
	addr    string
	timeout time.Duration
}

func NewGoRrdClient(addr string, timeout time.Duration) *GoRrdClient {
	return &GoRrdClient{
		addr:    addr,
		timeout: timeout,
	}
}

func (c *GoRrdClient) Connect() error {
	var client *rrd.Client
	var err error

	addr := c.addr
	if addr[:7] == "unix://" {
		addr = addr[7:]
		client, err = rrd.NewClient(addr, rrd.Timeout(c.timeout), rrd.Unix)
	} else {
		if addr[:6] == "smtp://" {
			addr = addr[6:]
		}
		client, err = rrd.NewClient(addr, rrd.Timeout(c.timeout))
	}

	c.client = client

	return err
}

func (c *GoRrdClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *GoRrdClient) ExecCmd(cmd *Cmd) ([]string, error) {
	return c.client.ExecCmd(c.convertCmd(cmd))
}

func (c *GoRrdClient) Batch(cmds ...*Cmd) error {
	var convertedCmds []*rrd.Cmd
	for _, cmd := range cmds {
		convertedCmds = append(convertedCmds, c.convertCmd(cmd))
	}
	return c.client.Batch(convertedCmds...)
}

func (c *GoRrdClient) Last(filename string) (time.Time, error) {
	return c.client.Last(filename)
}

func (c *GoRrdClient) Create(filename string, ds []DS, rra []RRA, step time.Duration) error {
	var convertedDS []rrd.DS
	for _, v := range ds {
		convertedDS = append(convertedDS, c.convertDS(v))
	}
	var convertedRRA []rrd.RRA
	for _, v := range rra {
		convertedRRA = append(convertedRRA, c.convertRRA(v))
	}
	return c.client.Create(filename, convertedDS, convertedRRA, rrd.Step(step))
}

// convertCmd takes a Cmd from this package and converts it to a go-rrd rrd.Cmd
func (c *GoRrdClient) convertCmd(cmd *Cmd) *rrd.Cmd {
	return rrd.NewCmd(cmd.GetCmd()).WithArgs(cmd.GetArgs())
}

// convertDS takes a DS from this package and converts it to a go-rrd rrd.DS
func (c *GoRrdClient) convertDS(ds DS) rrd.DS {
	return rrd.NewDS(ds.String())
}

// convertRRA takes a RRA from this package and converts it to a go-rrd rrd.RRA
func (c *GoRrdClient) convertRRA(rra RRA) rrd.RRA {
	return rrd.NewRRA(rra.String())
}
