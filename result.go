package gollector

import "time"

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

type Result struct {
	State      ResultState
	ReasonCode string
	Metrics    []ResultMetric
	Time       time.Time
}

func NewResult(state ResultState, reasonCode string, metrics []ResultMetric) *Result {
	return &Result{
		State:      state,
		ReasonCode: reasonCode,
		Metrics:    metrics,
		Time:       time.Now(),
	}
}

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

func MakeUnknownResult(reasonCode string) *Result {
	return &Result{
		State:      StateUnknown,
		ReasonCode: reasonCode,
		Metrics:    nil,
		Time:       time.Now(),
	}
}

type ResultMetricType uint8

const (
	ResultMetricCounter ResultMetricType = 1
	ResultMetricGauge                    = 2
)

type ResultMetric struct {
	Label string
	Value string
	Type  ResultMetricType
}
