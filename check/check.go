package check

import (
	"github.com/hashicorp/go-multierror"
	"sync"
	"time"
)

type Check struct {
	// Id should be any unique value for this check
	Id string

	// Schedule determines when this Check is due to be executed.
	Schedule Schedule
	Command  Command
	Meta     map[string]string

	Incident          *Incident
	SuppressIncidents bool

	Handlers []Handler

	LastCheck  *time.Time
	LastResult *Result
}

type Option func(*Check)

func New(id string, options ...Option) *Check {
	check := &Check{
		Id: id,
	}

	for _, option := range options {
		option(check)
	}

	return check
}

func WithCommand(cmd Command) Option {
	return func(c *Check) {
		c.Command = cmd
	}
}

func WithHandlers(handlers []Handler) Option {
	return func(c *Check) {
		c.Handlers = handlers
	}
}

func WithMeta(meta map[string]string) Option {
	return func(c *Check) {
		c.Meta = meta
	}
}

func WithSuppressedIncidents() Option {
	return func(c *Check) {
		c.SuppressIncidents = true
	}
}

func WithPeriodicSchedule(intervalSeconds int) Option {
	return func(c *Check) {
		c.Schedule = &PeriodicSchedule{IntervalSeconds: intervalSeconds}
	}
}

func WithSchedule(schedule Schedule) Option {
	return func(c *Check) {
		c.Schedule = schedule
	}
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
	result, err := c.Command.Run(*c)

	newIncident := c.makeNewIncidentIfJustified(result)
	c.resolveOrDiscardPreviousIncident(result, newIncident)

	c.runResultHandlerMutations(&result, newIncident)
	errP := c.runResultHandlerProcessing(result, newIncident)
	if errP != nil {
		err = multierror.Append(err, errP)
	}

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

func (c *Check) runResultHandlerProcessing(result Result, newIncident *Incident) error {
	if c.Handlers == nil {
		return nil
	}

	var wg sync.WaitGroup
	errorCh := make(chan error)
	wg.Add(len(c.Handlers))
	for _, h := range c.Handlers {
		go func(h Handler) {
			defer wg.Done()

			err := h.Process(*c, result, newIncident)

			if err != nil {
				errorCh <- err
			}
		}(h)
	}
	// wait on the group and close errorCh within a goroutine otherwise if 2+ of the processing goroutines do produce
	// errors, one will block writing to errorCh while wg.Wait() is also blocking and thus deadlock.  we have to read
	// from errorCh below to pop values from the errorCh to free space for other goroutines to write their errors
	go func() {
		wg.Wait()
		close(errorCh)
	}()

	var errors error
	for err := range errorCh {
		errors = multierror.Append(errors, err)
	}
	return errors
}

func (c *Check) makeNewIncidentIfJustified(result Result) *Incident {
	if !result.justifiesNewIncidentForCheck(*c) {
		return nil
	}

	i := MakeIncidentFromResults(c.LastResult, result)
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
	Run(Check) (Result, error)
}

// Handler mutates and/or processes a Check, and its latest result data
// Process() does not mutate any data, only read.  newIncident is a pointer only because it can be nil.
type Handler interface {
	Mutate(check *Check, newResult *Result, newIncident *Incident)
	Process(check Check, newResult Result, newIncident *Incident) error
}
