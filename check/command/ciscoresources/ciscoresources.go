package ciscoresources

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/snmp"
	"math/big"
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

	var cpuPerc, memUsed, memFree *big.Int
	for _, obj := range objects {
		value := snmp.ToBigInt(obj.Value)

		switch obj.Oid {
		case OidCpu:
			cpuPerc = value
		case OidMemUsed:
			memUsed = value
		case OidMemFree:
			memFree = value
		}
	}

	memTotal := big.NewInt(0).Add(memUsed, memFree)
	memoryPerc, _ := new(big.Float).Mul(
		new(big.Float).Quo(new(big.Float).SetInt(memUsed), new(big.Float).SetInt(memTotal)),
		big.NewFloat(100),
	).Int(nil)

	chk.Debugf("cpu=%s mem-total=%s mem-used=%s mem-percent-used=%s",
		cpuPerc.String(), memTotal.String(), memUsed.String(), memoryPerc.String())

	var resultState check.ResultState
	var resultReasonCode string
	resultMetrics := []check.ResultMetric{
		{
			Label: "cpu",
			Value: cpuPerc.String(),
			Type:  check.ResultMetricGauge,
		},
		{
			Label: "memory",
			Value: memoryPerc.String(),
			Type:  check.ResultMetricGauge,
		},
	}
	if cpuPerc.Cmp(big.NewInt(c.PercentCpuCritThreshold)) > 0 {
		resultState = check.StateCrit
		resultReasonCode = "CPU_USAGE_HIGH"
	} else if cpuPerc.Cmp(big.NewInt(c.PercentCpuWarnThreshold)) > 0 {
		resultState = check.StateWarn
		resultReasonCode = "CPU_USAGE_HIGH"
	} else if memoryPerc.Cmp(big.NewInt(c.PercentMemoryCritThreshold)) > 0 {
		resultState = check.StateCrit
		resultReasonCode = "MEM_USAGE_HIGH"
	} else if memoryPerc.Cmp(big.NewInt(c.PercentMemoryWarnThreshold)) > 0 {
		resultState = check.StateWarn
		resultReasonCode = "MEM_USAGE_HIGH"
	} else {
		resultState = check.StateOk
	}

	return check.NewResult(resultState, resultReasonCode, resultMetrics), nil
}
