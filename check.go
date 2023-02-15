package gollector

import (
	"sync"
	"time"
)

type Check struct {
	Schedule CheckSchedule
	Command  Command

	SuppressAlerts bool
	Meta           map[string]string
	Handlers       []Handler

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

func (c *Check) Execute() error {
	result, err := c.Command.Run()

	if c.Handlers != nil {
		// run c and result through handler mutations in order, one-by-one
		for _, h := range c.Handlers {
			h.Mutate(c, &result)
		}

		// now that mutations are finished, launch process handlers into goroutines
		var wg sync.WaitGroup
		wg.Add(len(c.Handlers))
		for _, h := range c.Handlers {
			go func(h Handler) {
				defer wg.Done()

				h.Process(*c, result)
			}(h)
		}
		wg.Wait()
	}

	t := time.Now()
	c.LastCheck = &t
	c.LastResult = &result

	return err
}

type Command interface {
	Run() (Result, error)
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

// Handler mutates and/or processes a Check and it's latest result data
// Process() does not mutate any data, only read
type Handler interface {
	Mutate(check *Check, result *Result)
	Process(check Check, result Result)
}

type Alert struct {
	FromState  ResultState
	ToState    ResultState
	ReasonCode string
	time       time.Time
}
