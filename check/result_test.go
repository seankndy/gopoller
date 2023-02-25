package check

import (
	"testing"
)

func TestJustifiesNewIncidentForCheck(t *testing.T) {
	tests := []struct {
		check  Check
		result Result
		want   bool
	}{
		{ // anytime incident suppression is enabled, no new incident
			check:  Check{SuppressIncidents: true},
			result: Result{State: StateCrit},
			want:   false,
		},
		{ // anytime current result is OK, no new incident
			check:  Check{Incident: &Incident{ToState: StateCrit}},
			result: Result{State: StateOk},
			want:   false,
		},
		{
			check:  Check{Incident: &Incident{ToState: StateWarn}},
			result: Result{State: StateOk},
			want:   false,
		},
		{
			check:  Check{LastResult: &Result{State: StateCrit}},
			result: Result{State: StateOk},
			want:   false,
		},
		{ // previous incident is Ok, new result is Crit, new incident
			check:  Check{Incident: &Incident{ToState: StateOk}},
			result: Result{State: StateCrit},
			want:   true,
		},
		{ // previous incident is Crit, new result is Crit, no new incident
			check:  Check{Incident: &Incident{ToState: StateCrit}},
			result: Result{State: StateCrit},
			want:   false,
		},
		{ // previous incident is Crit, new result is Warn, new incident
			check: Check{Incident: &Incident{
				ToState: StateCrit,
			}},
			result: Result{State: StateWarn},
			want:   true,
		},
		{ // no previous incident, but last result was Warn, new result is Crit, new incident
			check:  Check{LastResult: &Result{State: StateWarn}},
			result: Result{State: StateCrit},
			want:   true,
		},
		{ // no previous incident, but last result was Crit, new result is Warn, new incident
			check:  Check{LastResult: &Result{State: StateCrit}},
			result: Result{State: StateWarn},
			want:   true,
		},
		{ // no previous incident or last result and new result is warn, new incident
			check:  Check{},
			result: Result{State: StateWarn},
			want:   true,
		},
		{ // no previous incident or last result and new result is crit, new incident
			check:  Check{},
			result: Result{State: StateCrit},
			want:   true,
		},
	}

	for _, tt := range tests {
		got := tt.result.justifiesNewIncidentForCheck(tt.check)

		if got != tt.want {
			t.Errorf("justifiesNewIncidentForCheck() = %v, want %v", got, tt.want)
		}
	}
}

func TestResultState_String(t *testing.T) {
	tests := []struct {
		state ResultState
		want  string
	}{
		{
			state: StateUnknown,
			want:  "UNKNOWN",
		},
		{
			state: StateOk,
			want:  "OK",
		},
		{
			state: StateCrit,
			want:  "CRIT",
		},
		{
			state: StateWarn,
			want:  "WARN",
		},
	}

	for _, tt := range tests {
		got := tt.state.String()

		if got != tt.want {
			t.Errorf("ResultState.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestMakeUnknownResult(t *testing.T) {
	r := MakeUnknownResult("TEST")

	if r.State != StateUnknown {
		t.Errorf("MakeUnknownResult() returned Result with non-Unknown state, got %v", r.State)
	}

	if r.ReasonCode != "TEST" {
		t.Errorf("MakeUnknownResult() returned Result with bad ReasonCode, got %v", r.ReasonCode)
	}

	if r.Metrics != nil {
		t.Errorf("MakeUnknownResult() returned Result with Metrics %v, expected none", r.Metrics)
	}
}
