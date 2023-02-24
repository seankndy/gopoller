package junsubpool

import (
	"github.com/seankndy/gollector"
	"github.com/seankndy/gollector/snmp"
)

type Command struct {
	getter snmp.Getter

	Host              snmp.Host
	IpPoolSnmpIndexes []int
}

func (c Command) Run(check gollector.Check) (gollector.Result, error) {
	//TODO implement me
	panic("implement me")
}
