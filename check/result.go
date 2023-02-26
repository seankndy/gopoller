package check

import (
	"time"
)

// ResultState represents the state that a Check result is.
type ResultState uint8

const (
	StateOk      ResultState = 0
	StateWarn    ResultState = 1
	StateCrit    ResultState = 2
	StateUnknown ResultState = 3
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

// Result contains the state, reason, metrics and time of a check.Command.
type Result struct {
	State      ResultState
	ReasonCode string
	Metrics    []ResultMetric
	Time       time.Time
}

// NewResult creates a new Result with the provided attributes and the time
// set to now.
func NewResult(state ResultState, reasonCode string, metrics []ResultMetric) *Result {
	return &Result{
		State:      state,
		ReasonCode: reasonCode,
		Metrics:    metrics,
		Time:       time.Now(),
	}
}

// justifiesNewIncidentForCheck determines if the Result 'r' for the Check
// 'check' has undergone state changes that justify the creation of a new
// incident.
func (r Result) justifiesNewIncidentForCheck(check Check) bool {
	// if incident suppression is on, never allow new incident
	if check.SuppressIncidents {
		return false
	}

	lastResult := check.LastResult
	lastIncident := check.Incident

	// if current result is OK, no incident
	if r.State == StateOk {
		return false
	}

	// current result NOT OK and last incident exists
	if lastIncident != nil {
		// last incident to-state different from this result state
		return lastIncident.ToState != r.State
	}

	// current result NOT OK and NO last incident exists and last result exists
	if lastResult != nil {
		// last result state different from new state
		return lastResult.State != r.State
	}

	// not ok, no last incident, no last result
	return true
}

// MakeUnknownResult creates a new unknown Result with the given reason code.
// Unknown states are common during Command errors where a definitive state
// cannot be determined.
func MakeUnknownResult(reasonCode string) *Result {
	return &Result{
		State:      StateUnknown,
		ReasonCode: reasonCode,
		Metrics:    nil,
		Time:       time.Now(),
	}
}

// ResultMetricType represents a type of metric.  Currently, it can be either
// a counter or a gauge.
type ResultMetricType uint8

const (
	// ResultMetricCounter is an ever-incrementing integer that rolls over at
	// its inherent integer size and restarts to 0.
	ResultMetricCounter ResultMetricType = 1
	// ResultMetricGauge is any numeric value from some point in time.
	ResultMetricGauge ResultMetricType = 2
)

// ResultMetric is a metric that lives in a Result and was produced by a Command.
// For example, an HTTP check may have a "resp_time" Gauge metric that measured
// how long it took to get the HTTP response from an endpoint.  Another example
// is an SNMP check that has an "ifHCInOctets" Counter metric that defines the
// inbound bandwidth utilization of an interface.
type ResultMetric struct {
	// Label is an identifier for the metric (ex. avg_rtt_ms, temperature_f).
	Label string

	// Value is the metric's numeric value.  It is a string so that it can hold
	// any number, and it is up to the user to convert this into whatever
	// numerical type they need during processing.
	Value string

	// Type is the type of metric Value is.  Can be ResultMetricCounter or
	// ResultMetricGauge.
	Type ResultMetricType
}
