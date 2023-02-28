package dummy

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
)

type Handler struct{}

func (h *Handler) Mutate(check *check.Check, result *check.Result, newIncident *check.Incident) {
	return
}

func (h *Handler) Process(check check.Check, result check.Result, newIncident *check.Incident) error {
	fmt.Printf("processing data for check %v and result %v\n", check, result)
	return nil
}
