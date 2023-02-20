package snmp

import (
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gollector"
	"math/big"
)

type Command struct {
	Client      Client
	Ip          string
	Community   string
	Version     string
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
		oidMonitor := &c.OidMonitors[k]
		rawOids[k] = oidMonitor.Oid
		oidMonitorsByOid[oidMonitor.Oid] = oidMonitor
	}

	objects, err := getSnmpObjects(c.Client, rawOids)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	var resultMetrics []gollector.ResultMetric
	resultState := gollector.StateUnknown
	var resultReason string

	for _, object := range objects {
		oidMonitor := oidMonitorsByOid[object.Oid]

		value := gosnmp.ToBigInt(object.Value)

		var resultMetricValue string
		var resultMetricType gollector.ResultMetricType

		switch object.Type {
		case Counter32, Counter64: // variable is counter
			resultMetricType = gollector.ResultMetricCounter

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

			if resultState == gollector.StateUnknown {
				resultState, resultReason = oidMonitor.DetermineResultStateAndReasonFromValue(diff)
			}
		default:
			resultMetricType = gollector.ResultMetricGauge

			if resultState == gollector.StateUnknown {
				resultState, resultReason = oidMonitor.DetermineResultStateAndReasonFromValue(value)
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

	return *gollector.MakeUnknownResult(""), nil
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

func (o *OidMonitor) DetermineResultStateAndReasonFromValue(value *big.Int) (gollector.ResultState, string) {
	if value.Cmp(big.NewInt(o.WarnMinThreshold)) < 0 {
		return gollector.StateWarn, o.WarnMinReasonCode
	}

	return gollector.StateOk, ""
}

func getResultStateAndReason(value *big.Float, oidMonitor *OidMonitor) (gollector.ResultState, string) {
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

func getSnmpObjects(client Client, oids []string) ([]Object, error) {
	numOids := len(oids)
	objects := make([]Object, 0, numOids)

	// if numOids > snmp.MaxOids, chunk them and make ceil(numOids/snmp.MaxOids) SNMP GET requests
	var chunk int
	if numOids > client.MaxOids() {
		chunk = client.MaxOids()
	} else {
		chunk = numOids
	}
	for offset := 0; offset < numOids; offset += chunk {
		if chunk > numOids-offset {
			chunk = numOids - offset
		}

		objs, err := client.Get(oids[offset : offset+chunk])
		if err != nil {
			return nil, err
		}

		for _, v := range objs {
			objects = append(objects, v)
		}
	}

	return objects, nil
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
