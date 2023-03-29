package check

import (
	"github.com/google/uuid"
	"time"
)

// Incident defines a Check that has undergone a non-OK state change.
type Incident struct {
	Id           uuid.UUID
	FromState    ResultState
	ToState      ResultState
	ReasonCode   string
	Time         time.Time
	Resolved     *time.Time
	Acknowledged *time.Time
}

// Resolve sets the Incident to resolved at the current time.
func (i *Incident) Resolve() {
	t := time.Now()
	i.Resolved = &t
}

// Acknowledge sets the Incident to acknowledged at the current time.
func (i *Incident) Acknowledge() {
	t := time.Now()
	i.Acknowledged = &t
}

// IsAcknowledged returns true if incident has been acknowledged.
func (i *Incident) IsAcknowledged() bool {
	return i.Acknowledged != nil
}

// IsResolved returns true if incident has been resolved.
func (i *Incident) IsResolved() bool {
	return i.Resolved != nil
}

// MakeIncidentFromResults creates a new Incident based on a Check last Result,
// and it's current Result.
func MakeIncidentFromResults(lastResult *Result, currentResult *Result) *Incident {
	if lastResult == nil {
		lastResult = MakeUnknownResult("")
	}

	return &Incident{
		Id:         uuid.New(),
		FromState:  lastResult.State,
		ToState:    currentResult.State,
		ReasonCode: currentResult.ReasonCode,
		Time:       time.Now(),
	}
}
