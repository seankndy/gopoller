package snmp

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/snmp"
	"math/big"
	"strings"
	"time"
)

type OidMonitor struct {
	Oid  string
	Name string

	PostProcessValue float64

	WarnMinReasonCode string
	WarnMinThreshold  float64

	CritMinReasonCode string
	CritMinThreshold  float64

	WarnMaxReasonCode string
	WarnMaxThreshold  float64

	CritMaxReasonCode string
	CritMaxThreshold  float64

	// WarnStatus and CritStatus are for status OIDs that return fixed values indicating a state (example: ifStatus)
	WarnStatusReasonCode string
	WarnStatusValue      float64

	CritStatusReasonCode string
	CritStatusValue      float64
}

func NewOidMonitor(oid, name string) *OidMonitor {
	return &OidMonitor{
		Oid:              oid,
		Name:             name,
		PostProcessValue: 1.0,
	}
}

func (m OidMonitor) determineResultStateAndReasonFromResultValue(value *big.Float) (check.ResultState, string) {
	if m.CritStatusReasonCode != "" && value.Cmp(big.NewFloat(m.CritStatusValue)) == 0 {
		return check.StateCrit, m.CritStatusReasonCode
	} else if m.WarnStatusReasonCode != "" && value.Cmp(big.NewFloat(m.WarnStatusValue)) == 0 {
		return check.StateWarn, m.WarnStatusReasonCode
	} else if m.CritMinReasonCode != "" && value.Cmp(big.NewFloat(m.CritMinThreshold)) < 0 {
		return check.StateCrit, m.CritMinReasonCode
	} else if m.WarnMinReasonCode != "" && value.Cmp(big.NewFloat(m.WarnMinThreshold)) < 0 {
		return check.StateWarn, m.WarnMinReasonCode
	} else if m.CritMaxReasonCode != "" && value.Cmp(big.NewFloat(m.CritMaxThreshold)) > 0 {
		return check.StateCrit, m.CritMaxReasonCode
	} else if m.WarnMaxReasonCode != "" && value.Cmp(big.NewFloat(m.WarnMaxThreshold)) > 0 {
		return check.StateWarn, m.WarnMaxReasonCode
	}

	return check.StateOk, ""
}

func (m OidMonitor) String() string {
	return fmt.Sprintf(
		"name=%s oid=%s ppv=%f warn-min-thresh=%f crit-min-thres=%f warn-max-thres=%f crit-max-thres=%f warn-min-reason=%s crit-min-reason=%s warn-max-reason=%s crit-max-reason=%s",
		m.Name, m.Oid, m.PostProcessValue, m.WarnMinThreshold, m.CritMinThreshold, m.WarnMaxThreshold, m.CritMaxThreshold, m.WarnMinReasonCode, m.CritMinReasonCode, m.WarnMaxReasonCode, m.CritMaxReasonCode,
	)
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

func (c *Command) Run(chk *check.Check) (*check.Result, error) {
	var getter snmp.Getter
	if c.getter == nil {
		getter = snmp.DefaultGetter
	} else {
		getter = c.getter
	}

	chk.Debugf("beginning snmp execution for check: %s", chk)

	// create a map of oid->oidMonitors for fast OidMonitor lookup when processing the result values below
	oidMonitorsByOid := make(map[string]*OidMonitor, len(c.OidMonitors))
	// build raw slice of oids from c.OidMonitors to pass to getSnmpObjects()
	rawOids := make([]string, len(c.OidMonitors))
	for k, _ := range c.OidMonitors {
		rawOids[k] = c.OidMonitors[k].Oid
		oidMonitorsByOid[c.OidMonitors[k].Oid] = &c.OidMonitors[k]

		chk.Debugf("oid monitor: %s", c.OidMonitors[k])
	}

	currentTime := time.Now()
	objects, err := getter.Get(&c.Host, rawOids)
	if err != nil {
		if strings.Contains(err.Error(), "request timeout") {
			return check.MakeUnknownResult("CONNECTION_ERROR"), nil
		}
		return check.MakeUnknownResult("CMD_FAILURE"), err
	}

	var resultMetrics []check.ResultMetric
	resultState := check.StateUnknown
	var resultReason string

	for _, object := range objects {
		chk.Debugf("got oid=%s type=%v value=%v", object.Oid, object.Type, object.Value)

		oidMonitor := oidMonitorsByOid[object.Oid]
		if oidMonitor == nil {
			if object.Oid[:1] == "." {
				oidMonitor = oidMonitorsByOid[object.Oid[1:]]
			}
		}
		if oidMonitor == nil {
			return check.MakeUnknownResult("CMD_FAILURE"),
				fmt.Errorf("snmp.Command.Run(): oid %s could not be found in monitors", object.Oid)
		}

		if object.Type == snmp.Null {
			chk.Debugf("skipping oid=%s as it is null/nil", object.Oid)
			continue
		}

		var resultMetricValue string
		var resultMetricType check.ResultMetricType

		// for counter types, we compare the difference between the last result and this current result to the
		// monitor's thresholds, and also we do not apply PostProcessValue to the result
		// for non-counter types, we compare the raw value to the monitor thresholds, and we do apply PostProcessValue
		// to the value

		if object.Type == snmp.Counter64 || object.Type == snmp.Counter32 {
			value := snmp.ToBigInt(object.Value)

			resultMetricType = check.ResultMetricCounter
			resultMetricValue = value.Text(10)

			// if state isn't the worst state already (CRIT), then we need to check if this snmp object exceeds any
			// thresholds as it may be a worse state than what we are so far
			if resultState != check.StateCrit {
				// get last metric to calculate difference
				lastMetric := getChecksLastResultMetricByLabel(chk, oidMonitor.Name)

				chk.Debugf("counter oid %s last metric: %s", object.Oid, lastMetric)

				var lastValue *big.Int
				if lastMetric != nil {
					lastValue, _ = new(big.Int).SetString(lastMetric.Value, 10)
				}

				// only if we have a last value can we calculate counter differences
				if lastValue != nil && chk.LastCheck != nil {
					// calculate the difference between previous and current result value, accounting for rollover
					diff := snmp.CalculateCounterDiff(lastValue, value)
					timeDiff := new(big.Int).SetInt64(currentTime.Unix())
					timeDiff.Sub(timeDiff, new(big.Int).SetInt64(chk.LastCheck.Unix()))
					diff.Div(diff, timeDiff)

					chk.Debugf("counter oid %s has difference value of %s", object.Oid, diff)

					s, r := oidMonitor.determineResultStateAndReasonFromResultValue(convertBigIntToBigFloat(diff))
					if s.Overrides(resultState) {
						chk.Debugf("counter oid %s result state (%s, %s) overrides previous state (%s, %s)", object.Oid, s.String(), r, resultState.String(), resultReason)
						resultState, resultReason = s, r
					}
				}
			}
		} else {
			var value *big.Float

			switch val := object.Value.(type) {
			case string:
				chk.Debugf("gauge oid %s is a string value (%s)", object.Oid, val)
				value = convertStringToBigFloat(val)
			case []byte:
				chk.Debugf("gauge oid %s is a []byte value (%s)", object.Oid, val)
				value = convertStringToBigFloat(string(val))
			default:
				chk.Debugf("gauge value %v is not a string or []byte, assuming it's an integer", object.Value)
				value = convertBigIntToBigFloat(snmp.ToBigInt(object.Value))
			}

			s, r := oidMonitor.determineResultStateAndReasonFromResultValue(value)
			if s.Overrides(resultState) {
				chk.Debugf("gauge oid %s result state (%s, %s) overrides previous state (%s, %s)", object.Oid, s.String(), r, resultState.String(), resultReason)
				resultState, resultReason = s, r
			}

			resultMetricType = check.ResultMetricGauge
			// multiply object value by the post-process value, but only for non-counter types
			resultMetricValue = value.Mul(value, big.NewFloat(oidMonitor.PostProcessValue)).Text('f', -1)
		}

		resultMetrics = append(resultMetrics, check.ResultMetric{
			Label: oidMonitor.Name,
			Value: resultMetricValue,
			Type:  resultMetricType,
		})
	}

	return check.NewResult(resultState, resultReason, resultMetrics), nil
}

func getChecksLastResultMetricByLabel(chk *check.Check, label string) *check.ResultMetric {
	if chk.LastResult != nil {
		for k, _ := range chk.LastResult.Metrics {
			if chk.LastResult.Metrics[k].Label == label {
				return &chk.LastResult.Metrics[k]
			}
		}
	}

	return nil
}

func convertBigIntToBigFloat(bigInt *big.Int) *big.Float {
	return new(big.Float).SetPrec(uint(bigInt.BitLen())).SetInt(bigInt)
}

func convertStringToBigFloat(str string) *big.Float {
	f := new(big.Float).SetPrec(64)
	if _, ok := f.SetString(strings.TrimSpace(str)); !ok {
		f.SetFloat64(0)
	}
	return f
}
