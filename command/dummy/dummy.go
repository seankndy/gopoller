package dummy

import (
	"fmt"
	"github.com/seankndy/gollector"
	"time"
)

type Command struct {
	Message string
}

func (c Command) Run() (gollector.Result, error) {
	fmt.Printf("I am a dummy command: %v\n", c.Message)

	time.Sleep(500 * time.Millisecond)

	return gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics:    nil,
	}, nil
}
