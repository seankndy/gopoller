package command

import (
	"fmt"
	"github.com/seankndy/gollector"
	"time"
)

type DummyCommand struct{}

func (c DummyCommand) Run(attributes map[string]string) (gollector.Result, error) {
	fmt.Printf("I am a dummy command running check w/ attributes %v\n", attributes)

	time.Sleep(1 * time.Second)

	return gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics:    nil,
	}, nil
}
