package command

import (
	"fmt"
	"github.com/seankndy/gollector"
)

type DummyCommand struct{}

func (c DummyCommand) Name() string {
	return "DummyCommand"
}

func (c DummyCommand) Run(check gollector.Check) (gollector.Result, error) {
	fmt.Printf("I am a dummy command running check %v\n", check)

	return gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics:    nil,
	}, nil
}
