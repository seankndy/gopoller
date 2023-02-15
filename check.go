package gollector

import (
	"time"
)

type Check struct {
	Schedule CheckSchedule

	SuppressAlerts bool
	Meta           map[string]string

	Command           Command
	CommandAttributes map[string]string

	LastCheck  *time.Time
	LastResult *Result
}

// DueAt returns the time when check is due (could be past or future)
func (c *Check) DueAt() time.Time {
	return c.Schedule.DueAt(*c)
}

// IsDue returns true if the check is due for execution
func (c *Check) IsDue() bool {
	return c.DueAt().Compare(time.Now()) <= 0
}

func (c *Check) Execute() (Result, error) {
	result, err := c.Command.Run(c.CommandAttributes)

	t := time.Now()
	c.LastCheck = &t

	return result, err
}

type Command interface {
	Run(attributes map[string]string) (Result, error)
}

type BaseCommand struct{}

func (c BaseCommand) MergeAttributes(defaults map[string]string, actual map[string]string) map[string]string {
	combined := make(map[string]string)
	for k, v := range defaults {
		combined[k] = v
	}
	for k, v := range actual {
		combined[k] = v
	}
	return combined
}

type ResultState uint8

const (
	StateOk      ResultState = 0
	StateWarn                = 1
	StateCrit                = 2
	StateUnknown             = 3
)

func (s ResultState) String() string {
	switch s {
	case StateOk:
		return "OK"
	case StateWarn:
		return "WARN"
	case StateCrit:
		return "CRIT"
	default:
		return "UNKNOWN"
	}
}

type Result struct {
	State      ResultState
	ReasonCode string
	Metrics    []ResultMetric
}

func MakeUnknownResult(reasonCode string) Result {
	return Result{
		State:      StateUnknown,
		ReasonCode: reasonCode,
		Metrics:    nil,
	}
}

type ResultMetric struct {
	Label string
	Value string
}

type Handler interface {
	Mutate(check Check, result *Result)
	Process(check Check, result Result)
}

type Alert struct {
	FromState  ResultState
	ToState    ResultState
	ReasonCode string
	time       time.Time
}
