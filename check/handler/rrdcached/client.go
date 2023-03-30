package rrdcached

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Client interface {
	Connect() error
	Close() error
	// ExecCmd executes rrdcached command, then returns lines from rrdcached output
	ExecCmd(*Cmd) ([]string, error)
	Batch(...*Cmd) error
	Last(filename string) (time.Time, error)
	Ping() error
	Create(filename string, ds []DS, rra []RRA, step time.Duration) error
}

// Cmd defines an RRDCacheD command
type Cmd struct {
	cmd  string
	args []any
}

// NewCmd creates a new Cmd.
func NewCmd(cmd string) *Cmd {
	return &Cmd{cmd: cmd}
}

// WithArgs sets the command arguments.
func (c *Cmd) WithArgs(args ...any) *Cmd {
	cmd := NewCmd(c.cmd)
	cmd.args = args
	return cmd
}

func (c *Cmd) String() string {
	parts := append([]any{c.cmd}, c.args...)
	return fmt.Sprintln(parts...)
}

func (c *Cmd) GetCmd() string {
	return c.cmd
}

func (c *Cmd) GetArgs() []any {
	return c.args
}

type DST string

const (
	Gauge    DST = "GAUGE"
	Counter      = "COUNTER"
	Derive       = "DERIVE"
	Dcounter     = "DCOUNTER"
	Dderive      = "DDERIVE"
	Absolute     = "ABSOLUTE"
)

// DS represents a RRD data source definition
type DS struct {
	name      string
	dst       DST
	heartbeat int
	min       string
	max       string
}

func NewDS(name string, dst DST, heartbeat int, min, max string) DS {
	r := regexp.MustCompile("[^a-zA-Z0-9_]")
	name = r.ReplaceAllString(name, "")

	if len(name) > 19 {
		name = name[:19]
	}

	return DS{
		name:      name,
		dst:       dst,
		heartbeat: heartbeat,
		min:       min,
		max:       max,
	}
}

func NewCounterDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Counter, heartbeat, min, max)
}

func NewGaugeDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Gauge, heartbeat, min, max)
}

func NewDeriveDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Derive, heartbeat, min, max)
}

func NewDDeriveDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Dderive, heartbeat, min, max)
}

func NewDCounterDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Dcounter, heartbeat, min, max)
}

func NewAbsoluteDS(name string, heartbeat int, min, max string) DS {
	return NewDS(name, Absolute, heartbeat, min, max)
}

func (v DS) String() string {
	return fmt.Sprintf("DS:%s:%s:%d:%s:%s", v.name, v.dst, v.heartbeat, v.min, v.max)
}

func (v DS) Name() string {
	return v.name
}

func (v DS) DST() DST {
	return v.dst
}

const (
	Average = "AVERAGE"
	Min     = "MIN"
	Max     = "MAX"
	Last    = "LAST"

	HoltWintersPredict          = "HWPREDICT"
	MultipliedHoltWinterPredict = "MHWPREDICT"
	Seasonal                    = "SEASONAL"
	DevSeasonal                 = "DEVSEASONAL"
	DevPredict                  = "DEVPREDICT"
	Failures                    = "FAILURES"
)

// RRA represents a RRD round-robin archive definition
type RRA string

func newRRA(rra string, values ...any) RRA {
	parts := make([]string, len(values)+1)
	parts[0] = rra
	for i, v := range values {
		parts[i+1] = fmt.Sprint(v)
	}
	return RRA(fmt.Sprintf("RRA:%s", strings.Join(parts, ":")))
}

func NewMinRRA(xff float64, steps, rows int) RRA {
	return newRRA(Min, xff, steps, rows)
}

func NewAverageRRA(xff float64, steps, rows int) RRA {
	return newRRA(Average, xff, steps, rows)
}

func NewMaxRRA(xff float64, steps, rows int) RRA {
	return newRRA(Max, xff, steps, rows)
}

func NewLastRRA(xff float64, steps, rows int) RRA {
	return newRRA(Last, xff, steps, rows)
}

func NewHWPredictRRA(rows int, alpha, beta float32, period int, idx int) RRA {
	return newRRA(HoltWintersPredict, rows, alpha, beta, period, idx)
}

func NewMHWPredictRRA(rows int, alpha, beta float32, period int, idx int) RRA {
	return newRRA(MultipliedHoltWinterPredict, rows, alpha, beta, period, idx)
}

func NewSeasonalRRA(period int, gamma float32, idx int, window float32) RRA {
	return newRRA(Seasonal, period, gamma, idx, fmt.Sprintf("smoothing-window=%v", window))
}

func NewDevSeasonalRRA(period int, gamma float32, idx int, window float32) RRA {
	return newRRA(DevSeasonal, period, gamma, idx, fmt.Sprintf("smoothing-window=%v", window))
}

func NewDevPredictRRA(rows, idx int) RRA {
	return newRRA(DevPredict, rows, idx)
}

func NewFailuresRRA(rows, threshold, window, idx int) RRA {
	return newRRA(Failures, rows, threshold, window, idx)
}

func (v RRA) String() string {
	return string(v)
}
