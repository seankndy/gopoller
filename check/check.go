package check

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"reflect"
	"runtime"
	"sync"
	"time"
)

// Check defines a service or a host to be checked with a given Command and at
// a given Schedule.
type Check struct {
	// Id should be any unique value for this check.
	Id string

	// Schedule determines when this Check is due to be executed.
	Schedule Schedule

	// Command is the command this check runs against the service or host.
	// Examples are snmp, ping, dns, or http commands
	Command Command

	// Meta is a key/value store of any extra data you want to live with the
	// check.
	Meta map[string]any

	// Incident needs to be the current active incident for this check
	// or else nil.
	Incident *Incident

	// SuppressIncidents set to true means when this Check executes and
	// produces an Incident, it discards it.
	SuppressIncidents bool

	// Handlers is a slice of handlers to execute after the Check's Command runs.
	// A Handler has a Mutate() and Process() method for mutating a Check's data
	// and processing it, respectfully.  Mutate() methods are called first in
	// sequential order as specified in this slice.  Then Process() methods are
	// called asynchronously.
	Handlers []Handler

	// LastCheck is a time.Time of the last time this Check executed. This will
	// be updated automatically by Execute(), but be sure it's set to the
	// correct time when loading a check from an external database.
	LastCheck *time.Time

	// LastResult is a Result from the last time this Check executed (or nil).
	// This will be updated automatically by Execute(), but be sure it's set to
	// the correct Result when loading a check from an external database.
	//
	// LastResult could be used by the Command to determine value deltas.  If
	// you are certain this is not required in your case, then this could always
	// be nil.
	LastResult *Result

	// debugLogger is called by Debug. Commands and Handlers call the Check's
	// Debugf() method with debugging information.  This is generally nil unless
	// you want to debug a particular Check.
	debugLogger debugLogger

	// Executed is true when the Check has had Execute() called on it.  You should
	// set this back to false prior to queueing it again.
	Executed bool
}

type debugLogger interface {
	Debugf(format string, args ...any)
}

type Option func(*Check)

// New creates a new Check with the provided Options.
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

func WithMeta(meta map[string]any) Option {
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

func WithDebugLogger(logger debugLogger) Option {
	return func(c *Check) {
		c.debugLogger = logger
	}
}

func (c *Check) SetDebugLogger(logger debugLogger) {
	c.debugLogger = logger
}

// DueAt returns the time when check is due (could be past or future)
func (c *Check) DueAt() time.Time {
	return c.Schedule.DueAt(*c)
}

// IsDue returns true if the check is due for execution
func (c *Check) IsDue() bool {
	return c.DueAt().Compare(time.Now()) <= 0
}

// Debugf should be used liberally by Commands and Handlers to provide debug
// information.
func (c *Check) Debugf(format string, args ...any) {
	if c.debugLogger != nil {
		formatPrefix := fmt.Sprintf("[ID:%s] ", c.Id)

		// get the caller's information
		pc, _, _, ok := runtime.Caller(1)
		if ok {
			formatPrefix += fmt.Sprintf("[%s] ", runtime.FuncForPC(pc).Name())
		}

		c.debugLogger.Debugf(formatPrefix+format, args...)
	}
}

// Execute executes a Check's Command followed by its Handlers.  It then sets
// the Incident (if there is one), LastCheck and LastResult fields on the Check.
func (c *Check) Execute() error {
	c.Executed = true

	var result *Result
	var err error
	if c.Command == nil {
		result, err = MakeUnknownResult("CMD_FAILURE"), errors.New("command not defined in check")
	} else {
		result, err = c.Command.Run(c)
	}

	c.Debugf("result-state=%s result-reason-code=%s result-metrics=%d result-time=%d",
		result.State.String(), result.ReasonCode, len(result.Metrics), result.Time.Unix())

	newIncident := c.makeNewIncidentIfJustified(result)
	c.Debugf("new-incident=%v", newIncident != nil)
	c.resolveOrDiscardPreviousIncident(result, newIncident)

	c.runResultHandlerMutations(result, newIncident)
	errP := c.runResultHandlerProcessing(result, newIncident)
	if errP != nil {
		err = multierror.Append(err, errP)
	}

	t := time.Now()
	c.LastCheck = &t
	c.LastResult = result
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

func (c *Check) runResultHandlerProcessing(result *Result, newIncident *Incident) error {
	if c.Handlers == nil {
		return nil
	}

	var wg sync.WaitGroup
	errorCh := make(chan error)
	wg.Add(len(c.Handlers))
	for _, h := range c.Handlers {
		go func(h Handler) {
			defer wg.Done()

			err := h.Process(c, result, newIncident)

			if err != nil {
				t := reflect.TypeOf(h)
				if t.Kind() == reflect.Ptr {
					t = t.Elem()
				}
				errorCh <- fmt.Errorf("error in handler '%s': %v", t.PkgPath()+"."+t.Name(), err)
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

func (c *Check) makeNewIncidentIfJustified(result *Result) *Incident {
	if !result.justifiesNewIncidentForCheck(*c) {
		return nil
	}

	i := MakeIncidentFromResults(c.LastResult, result)
	return i
}

// resolveOrDiscardPreviousIncident takes a new result and incident and determines if an old incident within the
// check should be resolved or discarded.
func (c *Check) resolveOrDiscardPreviousIncident(newResult *Result, newIncident *Incident) {
	// if an existing incident exists and the current state is OK or there is now a new incident
	if c.Incident != nil && (newResult.State == StateOk || newIncident != nil) {
		if c.Incident.Resolved == nil {
			// resolve it since we are now OK or have new incident
			c.Debugf("resolving previous incident")
			c.Incident.Resolve()
		} else {
			// already resolved(old incident), discard it
			c.Debugf("discarding previous incident")
			c.Incident = nil
		}
	}
}

// Command is a simple interface with a Run(Check) method that returns a Result
// and error.
type Command interface {
	Run(*Check) (*Result, error)
}

// Handler mutates and/or processes a Check after it has executed.  Mutate()
// is called first and sequentially in the order defined in the Check.  This
// allows the second mutation to see the first mutations, etc.  Process() is
// called asynchronously and should never mutate data.
type Handler interface {
	// Mutate allows the handler to mutate any data in the Check, Result or
	// Incident prior to Process()ing it.
	Mutate(check *Check, newResult *Result, newIncident *Incident)

	// Process executes asynchronously and should never mutate data. newIncident
	// is a pointer as it may be nil indicating there isn't a new incident for
	// the Check.
	Process(check *Check, newResult *Result, newIncident *Incident) error
}
