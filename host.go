package gollector

import (
	"time"
)

type Host struct {
	Name          string
	Ip            string
	AliveCheck    *Check // if AliveCheck CRIT, all other Host checks do not run
	CheckInterval uint
	LastCheck     *time.Time
}
