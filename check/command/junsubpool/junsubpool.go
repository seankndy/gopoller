// Package junsubpool provides a Check command that monitors the IP pool usage on a Juniper BNG.
package junsubpool

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/snmp"
	"strings"
)

const (
	OidPoolAddrTotal  = ".1.3.6.1.4.1.2636.3.51.1.1.4.1.1.1.10"
	OidPoolAddrsInUse = ".1.3.6.1.4.1.2636.3.51.1.1.4.1.1.1.11"
)

type Command struct {
	getter snmp.Getter

	Host                            snmp.Host
	IpPoolSnmpIndexes               []int
	PercentUtilizationWarnThreshold float64
	PercentUtilizationCritThreshold float64
}

func NewCommand(addr, community string, ipPoolIndexes []int, percWarnThreshold, percCritThreshold float64) *Command {
	return &Command{
		Host:                            *snmp.NewHost(addr, community),
		IpPoolSnmpIndexes:               ipPoolIndexes,
		PercentUtilizationWarnThreshold: percWarnThreshold,
		PercentUtilizationCritThreshold: percCritThreshold,
	}
}

func (c *Command) SetGetter(getter snmp.Getter) {
	c.getter = getter
}

func (c *Command) Run(*check.Check) (*check.Result, error) {
	var getter snmp.Getter
	if c.getter == nil {
		getter = snmp.DefaultGetter
	} else {
		getter = c.getter
	}

	objects, err := getter.Get(&c.Host, c.getOids())
	if err != nil {
		return check.MakeUnknownResult("CMD_FAILURE"), err
	}

	var total, used uint64
	for _, obj := range objects {
		var value uint64
		switch obj.Type {
		case snmp.Counter64:
			value = obj.Value.(uint64)
		case snmp.Counter32, snmp.Uinteger32, snmp.Gauge32:
			value = uint64(obj.Value.(uint32))
		case snmp.Integer:
			value = uint64(obj.Value.(int))
		}

		if strings.HasPrefix(obj.Oid, OidPoolAddrTotal) {
			total += value
		} else if strings.HasPrefix(obj.Oid, OidPoolAddrsInUse) {
			used += value
		}
	}

	percentUsed := float64(used) / float64(total) * 100.0

	var resultState check.ResultState
	var resultReasonCode string
	resultMetrics := []check.ResultMetric{
		{
			Label: "total_pool_usage",
			Value: fmt.Sprintf("%d", used),
			Type:  check.ResultMetricGauge,
		},
	}
	if percentUsed > c.PercentUtilizationCritThreshold {
		resultState = check.StateCrit
		resultReasonCode = "IP_POOL_USAGE_HIGH"
	} else if percentUsed > c.PercentUtilizationWarnThreshold {
		resultState = check.StateWarn
		resultReasonCode = "IP_POOL_USAGE_HIGH"
	} else {
		resultState = check.StateOk
	}

	return check.NewResult(resultState, resultReasonCode, resultMetrics), nil
}

func (c *Command) getOids() []string {
	oids := make([]string, 0, len(c.IpPoolSnmpIndexes)*2) // *2 because we are using 2 oids per index in the loop below
	for _, idx := range c.IpPoolSnmpIndexes {
		oids = append(oids, fmt.Sprintf("%s.%d", OidPoolAddrTotal, idx))
		oids = append(oids, fmt.Sprintf("%s.%d", OidPoolAddrsInUse, idx))
	}
	return oids
}
