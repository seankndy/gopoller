package rrdcached

import (
	"fmt"
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

// DS represents a RRD data source definition
type DS string

func NewDS(raw string) DS {
	return DS(raw)
}

func (v DS) String() string {
	return string(v)
}

func (v DS) GetName() string {
	parts := strings.Split(string(v), ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// RRA represents a RRD round-robin archive definition
type RRA string

func NewRRA(raw string) RRA {
	return RRA(raw)
}

func (v RRA) String() string {
	return string(v)
}
