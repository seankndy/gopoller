package ciscoresources

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/snmp"
)

const (
	OidCpu     = ".1.3.6.1.4.1.9.2.1.57.0"
	OidMemFree = ".1.3.6.1.4.1.9.9.48.1.1.1.6.1"
	OidMemUsed = ".1.3.6.1.4.1.9.9.48.1.1.1.5.1"
)

type Command struct {
	getter snmp.Getter

	Host                       snmp.Host
	PercentCpuWarnThreshold    int64
	PercentCpuCritThreshold    int64
	PercentMemoryWarnThreshold int64
	PercentMemoryCritThreshold int64
}

func NewCommand(addr, community string, percCpuWarnThreshold, percCpuCritThreshold, percMemWarnThreshold, percMemCritThreshold int64) *Command {
	return &Command{
		Host:                       *snmp.NewHost(addr, community),
		PercentCpuWarnThreshold:    percCpuWarnThreshold,
		PercentCpuCritThreshold:    percCpuCritThreshold,
		PercentMemoryWarnThreshold: percMemWarnThreshold,
		PercentMemoryCritThreshold: percMemCritThreshold,
	}
}

func (c *Command) SetGetter(getter snmp.Getter) {
	c.getter = getter
}

func (c *Command) Run(chk *check.Check) (*check.Result, error) {
	var getter snmp.Getter
	if c.getter == nil {
		getter = snmp.DefaultGetter
	} else {
		getter = c.getter
	}

	objects, err := getter.Get(&c.Host, []string{OidCpu, OidMemUsed, OidMemFree})
	if err != nil {
		return check.MakeUnknownResult("CMD_FAILURE"), err
	}
	if len(objects) != 3 {
		return check.MakeUnknownResult("CMD_FAILURE"), fmt.Errorf("expected 3 snmp objects, got %d", len(objects))
	}

	var cpuPerc, memUsed, memFree int64
	for _, obj := range objects {
		var value int64
		switch obj.Type {
		case snmp.Uinteger32, snmp.Gauge32:
			value = int64(obj.Value.(uint32))
		case snmp.Integer:
			value = int64(obj.Value.(int))
		default:
			chk.Debugf("unsupported snmp object type (non-integer): %v", obj.Type)
			continue
		}

		switch obj.Oid {
		case OidCpu:
			cpuPerc = value
		case OidMemUsed:
			memUsed = value
		case OidMemFree:
			memFree = value
		}
	}

	memoryPerc := int64(float64(memUsed) / (float64(memFree) + float64(memUsed)) * 100.0)

	var resultState check.ResultState
	var resultReasonCode string
	resultMetrics := []check.ResultMetric{
		{
			Label: "cpu",
			Value: fmt.Sprintf("%d", cpuPerc),
			Type:  check.ResultMetricGauge,
		},
		{
			Label: "memory",
			Value: fmt.Sprintf("%d", memoryPerc),
			Type:  check.ResultMetricGauge,
		},
	}
	if cpuPerc > c.PercentCpuCritThreshold {
		resultState = check.StateCrit
		resultReasonCode = "CPU_USAGE_HIGH"
	} else if cpuPerc > c.PercentCpuWarnThreshold {
		resultState = check.StateWarn
		resultReasonCode = "CPU_USAGE_HIGH"
	} else if memoryPerc > c.PercentMemoryCritThreshold {
		resultState = check.StateCrit
		resultReasonCode = "MEM_USAGE_HIGH"
	} else if memoryPerc > c.PercentMemoryWarnThreshold {
		resultState = check.StateWarn
		resultReasonCode = "MEM_USAGE_HIGH"
	} else {
		resultState = check.StateOk
	}

	return check.NewResult(resultState, resultReasonCode, resultMetrics), nil
}
