package dummy

import (
	"fmt"
	"github.com/seankndy/gopoller"
)

type Handler struct{}

func (h Handler) Mutate(check *gopoller.Check, result *gopoller.Result, newIncident *gopoller.Incident) {
	return
}

func (h Handler) Process(check gopoller.Check, result gopoller.Result, newIncident *gopoller.Incident) error {
	fmt.Printf("processing data for check %v and result %v\n", check, result)
	return nil
}
