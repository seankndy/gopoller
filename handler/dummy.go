package handler

import (
	"fmt"
	"github.com/seankndy/gollector"
)

type DummyHandler struct{}

func (h DummyHandler) Mutate(check *gollector.Check, result *gollector.Result) {
	fmt.Printf("mutating data for check %v and result %v\n", check, result)
}

func (h DummyHandler) Process(check gollector.Check, result gollector.Result) {
	fmt.Printf("processing data for check %v and result %v\n", check, result)
}
