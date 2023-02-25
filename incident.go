package gopoller

import (
	"github.com/google/uuid"
	"time"
)

type Incident struct {
	Id           uuid.UUID
	FromState    ResultState
	ToState      ResultState
	ReasonCode   string
	Time         time.Time
	Resolved     *time.Time
	Acknowledged *time.Time
}

func (i *Incident) Resolve() {
	t := time.Now()
	i.Resolved = &t
}

func (i *Incident) Acknowledge() {
	t := time.Now()
	i.Acknowledged = &t
}

func makeIncidentFromResults(lastResult *Result, currentResult Result) Incident {
	if lastResult == nil {
		lastResult = MakeUnknownResult("")
	}

	return Incident{
		Id:         uuid.New(),
		FromState:  lastResult.State,
		ToState:    currentResult.State,
		ReasonCode: currentResult.ReasonCode,
		Time:       time.Now(),
	}
}
