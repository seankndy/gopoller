package command

import (
	"fmt"
	"github.com/seankndy/gollector"
	"time"
)

type DummyCommand struct {
	Message string
}

func (c DummyCommand) Run() (gollector.Result, error) {
	fmt.Printf("I am a dummy command: %v\n", c.Message)

	time.Sleep(1 * time.Second)

	return gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics:    nil,
	}, nil
}
