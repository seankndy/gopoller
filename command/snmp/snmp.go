package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gopoller"
	"github.com/seankndy/gopoller/snmp"
	"math/big"
)

type OidMonitor struct {
	Oid               string
	Name              string
	PostProcessValue  float64
	WarnMinThreshold  int64
	CritMinThreshold  int64
	WarnMaxThreshold  int64
	CritMaxThreshold  int64
	WarnMinReasonCode string
	CritMinReasonCode string
	WarnMaxReasonCode string
	CritMaxReasonCode string
}

func NewOidMonitor(oid, name string) *OidMonitor {
	return &OidMonitor{
		Oid:              oid,
		Name:             name,
		PostProcessValue: 1.0,
	}
}

func (o *OidMonitor) determineResultStateAndReasonFromResultValue(value *big.Int) (gopoller.ResultState, string) {
	if o.CritMinReasonCode != "" && value.Cmp(big.NewInt(o.CritMinThreshold)) < 0 {
		return gopoller.StateCrit, o.CritMinReasonCode
	} else if o.WarnMinReasonCode != "" && value.Cmp(big.NewInt(o.WarnMinThreshold)) < 0 {
		return gopoller.StateWarn, o.WarnMinReasonCode
	} else if o.CritMaxReasonCode != "" && value.Cmp(big.NewInt(o.CritMaxThreshold)) > 0 {
		return gopoller.StateCrit, o.CritMaxReasonCode
	} else if o.WarnMaxReasonCode != "" && value.Cmp(big.NewInt(o.WarnMaxThreshold)) > 0 {
		return gopoller.StateWarn, o.WarnMaxReasonCode
	}

	return gopoller.StateOk, ""
}

type Command struct {
	getter snmp.Getter

	Host        snmp.Host
	OidMonitors []OidMonitor
}

func (c *Command) SetGetter(getter snmp.Getter) {
	c.getter = getter
}

func NewCommand(addr, community string, monitors []OidMonitor) *Command {
	return &Command{
		Host:        *snmp.NewHost(addr, community),
		OidMonitors: monitors,
	}
}

func (c *Command) Run(check gopoller.Check) (result gopoller.Result, err error) {
	var getter snmp.Getter
	if c.getter == nil {
		getter = snmp.DefaultGetter
	} else {
		getter = c.getter
	}

	// create a map of oid->oidMonitors for fast OidMonitor lookup when processing the result values below
	oidMonitorsByOid := make(map[string]*OidMonitor, len(c.OidMonitors))
	// build raw slice of oids from c.OidMonitors to pass to getSnmpObjects()
	rawOids := make([]string, len(c.OidMonitors))
	for k, _ := range c.OidMonitors {
		rawOids[k] = c.OidMonitors[k].Oid
		oidMonitorsByOid[c.OidMonitors[k].Oid] = &c.OidMonitors[k]
	}

	objects, err := getter.Get(&c.Host, rawOids)
	if err != nil {
		return *gopoller.MakeUnknownResult("CMD_FAILURE"), err
	}

	var resultMetrics []gopoller.ResultMetric
	resultState := gopoller.StateUnknown
	var resultReason string

	for _, object := range objects {
		oidMonitor := oidMonitorsByOid[object.Oid]
		if oidMonitor == nil {
			if object.Oid[:1] == "." {
				oidMonitor = oidMonitorsByOid[object.Oid[1:]]
			}
		}
		if oidMonitor == nil {
			return *gopoller.MakeUnknownResult("CMD_FAILURE"),
				fmt.Errorf("snmp.Command.Run(): oid %s could not be found in monitors", object.Oid)
		}

		value := gosnmp.ToBigInt(object.Value)

		var resultMetricValue string
		var resultMetricType gopoller.ResultMetricType

		// for counter types, we compare the difference between the last result and this current result to the
		// monitor's thresholds, and also we do not apply PostProcessValue to the result
		// for non-counter types, we compare the raw value to the monitor thresholds, and we do apply PostProcessValue
		// to the value

		if object.Type == snmp.Counter64 || object.Type == snmp.Counter32 {
			resultMetricType = gopoller.ResultMetricCounter
			resultMetricValue = value.Text(10)

			// if state is still Unknown, check if this snmp object exceeds any thresholds
			if resultState == gopoller.StateUnknown {
				// get last metric to calculate difference
				lastMetric := getChecksLastResultMetricByLabel(&check, oidMonitor.Name)
				var lastValue *big.Int
				if lastMetric != nil {
					var ok bool
					lastValue, ok = new(big.Int).SetString(lastMetric.Value, 10)
					if !ok {
						lastValue = big.NewInt(0)
					}
				} else {
					lastValue = big.NewInt(0)
				}

				// calculate the difference between previous and current result value, accounting for rollover
				var diff *big.Int
				if object.Type == snmp.Counter64 {
					diff = snmp.CalculateCounterDiff(lastValue, value, 64)
				} else {
					diff = snmp.CalculateCounterDiff(lastValue, value, 32)
				}

				resultState, resultReason = oidMonitor.determineResultStateAndReasonFromResultValue(diff)
			}
		} else {
			resultMetricType = gopoller.ResultMetricGauge

			// if state is still Unknown, check if this snmp object exceeds any thresholds
			if resultState == gopoller.StateUnknown {
				resultState, resultReason = oidMonitor.determineResultStateAndReasonFromResultValue(value)
			}

			// multiply object value by the post-process value, but only for non-counter types
			valueF := big.NewFloat(0).SetPrec(uint(value.BitLen())).SetInt(value)
			resultMetricValue = valueF.Mul(valueF, big.NewFloat(oidMonitor.PostProcessValue)).Text('f', -1)
		}

		resultMetrics = append(resultMetrics, gopoller.ResultMetric{
			Label: oidMonitor.Name,
			Value: resultMetricValue,
			Type:  resultMetricType,
		})

	}

	return *gopoller.NewResult(resultState, resultReason, resultMetrics), nil
}

func getChecksLastResultMetricByLabel(check *gopoller.Check, label string) *gopoller.ResultMetric {
	if check.LastResult != nil {
		for k, _ := range check.LastResult.Metrics {
			if check.LastResult.Metrics[k].Label == label {
				return &check.LastResult.Metrics[k]
			}
		}
	}

	return nil
}
