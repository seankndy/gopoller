package dummy

import (
	"fmt"
	"github.com/seankndy/gollector"
)

type Handler struct{}

func (h Handler) Mutate(check *gollector.Check, result *gollector.Result, newIncident *gollector.Incident) {
	return
}

func (h Handler) Process(check gollector.Check, result gollector.Result, newIncident *gollector.Incident) error {
	fmt.Printf("processing data for check %v and result %v\n", check, result)
	return nil
}
