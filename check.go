package gollector

import (
	"sync"
	"time"
)

type Check struct {
	Schedule CheckSchedule
	Command  Command
	Meta     map[string]string

	Incident          *Incident
	SuppressIncidents bool

	Handlers []Handler

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

	newIncident := c.makeNewIncidentIfJustified(result)
	c.resolveOrDiscardPreviousIncident(result, newIncident)

	c.runResultHandlerMutations(&result, newIncident)
	c.runResultHandlerProcessing(result, newIncident)

	t := time.Now()
	c.LastCheck = &t
	c.LastResult = &result
	if newIncident != nil {
		c.Incident = newIncident
	}

	return err
}

func (c *Check) runResultHandlerMutations(result *Result, newIncident *Incident) {
	if c.Handlers != nil {
		for _, h := range c.Handlers {
			h.Mutate(c, result, newIncident)
		}
	}
}

func (c *Check) runResultHandlerProcessing(result Result, newIncident *Incident) {
	if c.Handlers != nil {
		var wg sync.WaitGroup
		wg.Add(len(c.Handlers))
		for _, h := range c.Handlers {
			go func(h Handler) {
				defer wg.Done()

				h.Process(*c, result, newIncident)
			}(h)
		}
		wg.Wait()
	}
}

func (c *Check) makeNewIncidentIfJustified(result Result) *Incident {
	if !result.justifiesNewIncidentForCheck(*c) {
		return nil
	}

	i := makeIncidentFromResults(c.LastResult, result)
	return &i
}

// resolveOrDiscardPreviousIncident takes a new result and incident and determines if an old incident within the
// check should be resolved or discarded.
func (c *Check) resolveOrDiscardPreviousIncident(newResult Result, newIncident *Incident) {
	// if an existing incident exists and the current state is OK or there is now a new incident
	if c.Incident != nil && (newResult.State == StateOk || newIncident != nil) {
		if c.Incident.Resolved == nil {
			// resolve it since we are now OK or have new incident
			c.Incident.Resolve()
		} else {
			// already resolved(old incident), discard it
			c.Incident = nil
		}
	}
}

type Command interface {
	Run() (Result, error)
}

// Handler mutates and/or processes a Check and it's latest result data
// Process() does not mutate any data, only read
type Handler interface {
	Mutate(check *Check, newResult *Result, newIncident *Incident)
	Process(check Check, newResult Result, newIncident *Incident)
}