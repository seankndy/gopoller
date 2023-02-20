package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gollector"
	"math/big"
)

type Command struct {
	Client      Client
	OidMonitors []OidMonitor
}

func (c *Command) Run(check gollector.Check) (result gollector.Result, err error) {
	err = c.Client.Connect()
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	defer func() {
		errC := c.Client.Close()
		if errC != nil {
			err = errC
		}
	}()

	// create a map of oid->oidMonitors for fast OidMonitor lookup when processing the result values below
	oidMonitorsByOid := make(map[string]*OidMonitor, len(c.OidMonitors))
	// build raw slice of oids from c.OidMonitors to pass to getSnmpObjects()
	rawOids := make([]string, len(c.OidMonitors))
	for k, _ := range c.OidMonitors {
		rawOids[k] = c.OidMonitors[k].Oid
		oidMonitorsByOid[c.OidMonitors[k].Oid] = &c.OidMonitors[k]
	}

	objects, err := c.Client.Get(rawOids)
	if err != nil {
		fmt.Println(err)
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	var resultMetrics []gollector.ResultMetric
	resultState := gollector.StateUnknown
	var resultReason string

	for _, object := range objects {
		oidMonitor, ok := oidMonitorsByOid[object.Oid]
		if !ok {
			return *gollector.MakeUnknownResult("CMD_FAILURE"), nil
		}

		value := gosnmp.ToBigInt(object.Value)

		var resultMetricValue string
		var resultMetricType gollector.ResultMetricType

		// for counter types, we compare the difference between the last result and this current result to the
		// monitor's thresholds, and also we do not apply PostProcessValue to the result
		// for non-counter types, we compare the raw value to the monitor thresholds, and we do apply PostProcessValue
		// to the value

		if object.Type == Counter64 || object.Type == Counter32 {
			resultMetricType = gollector.ResultMetricCounter
			resultMetricValue = value.Text(10)

			// if state is still Unknown, check if this snmp object exceeds any thresholds
			if resultState == gollector.StateUnknown {
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

				var diff *big.Int
				if object.Type == Counter64 {
					diff = calculateCounterDiff(lastValue, value, 64)
				} else {
					diff = calculateCounterDiff(lastValue, value, 32)
				}

				resultState, resultReason = oidMonitor.determineResultStateAndReasonFromResultValue(diff)
			}
		} else {
			resultMetricType = gollector.ResultMetricGauge

			// if state is still Unknown, check if this snmp object exceeds any thresholds
			if resultState == gollector.StateUnknown {
				resultState, resultReason = oidMonitor.determineResultStateAndReasonFromResultValue(value)
			}

			// multiply object value by the post-process value, but only for non-counter types
			valueF := big.NewFloat(0).SetPrec(uint(value.BitLen())).SetInt(value)
			resultMetricValue = valueF.Mul(valueF, big.NewFloat(oidMonitor.PostProcessValue)).Text('f', -1)
		}

		resultMetrics = append(resultMetrics, gollector.ResultMetric{
			Label: oidMonitor.Name,
			Value: resultMetricValue,
			Type:  resultMetricType,
		})

	}

	return *gollector.NewResult(resultState, resultReason, resultMetrics), nil
}

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

func (o *OidMonitor) determineResultStateAndReasonFromResultValue(value *big.Int) (gollector.ResultState, string) {
	if o.CritMinReasonCode != "" && value.Cmp(big.NewInt(o.CritMinThreshold)) < 0 {
		return gollector.StateCrit, o.CritMinReasonCode
	} else if o.WarnMinReasonCode != "" && value.Cmp(big.NewInt(o.WarnMinThreshold)) < 0 {
		return gollector.StateWarn, o.WarnMinReasonCode
	} else if o.CritMaxReasonCode != "" && value.Cmp(big.NewInt(o.CritMaxThreshold)) > 0 {
		return gollector.StateCrit, o.CritMaxReasonCode
	} else if o.WarnMaxReasonCode != "" && value.Cmp(big.NewInt(o.WarnMaxThreshold)) > 0 {
		return gollector.StateWarn, o.WarnMaxReasonCode
	}

	return gollector.StateOk, ""
}

func getChecksLastResultMetricByLabel(check *gollector.Check, label string) *gollector.ResultMetric {
	if check.LastResult != nil {
		for k, _ := range check.LastResult.Metrics {
			if check.LastResult.Metrics[k].Label == label {
				return &check.LastResult.Metrics[k]
			}
		}
	}

	return nil
}

// calculateCounterDiff calculates the difference between lastValue and currentValue, taking into account rollover at nbits unsigned bits
func calculateCounterDiff(lastValue *big.Int, currentValue *big.Int, nbits uint8) *big.Int {
	maxCounterValue := new(big.Int).SetUint64(uint64(1<<nbits - 1))
	diff := currentValue.Sub(currentValue, lastValue)
	if diff.Cmp(big.NewInt(0)) < 0 {
		diff = diff.Add(diff, maxCounterValue)
	}
	return diff
}
