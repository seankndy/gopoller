package snmp

import (
	"fmt"
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

func (c *Command) Run(check gollector.Check) (gollector.Result, error) {
	err := c.Client.Connect()
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	defer c.Client.Close()

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
	for i, object := range objects {
		oidMonitor := oidMonitorsByOid[object.Oid]
		fmt.Printf("%d: oid: %s ", i, object.Oid)

		valueI := gosnmp.ToBigInt(object.Value)
		valueF := big.NewFloat(0).SetPrec(uint(valueI.BitLen()))
		valueF.SetInt(valueI)
		valueF.Mul(valueF, big.NewFloat(oidMonitor.PostProcessValue))

		switch object.Type {
		case Counter32, Counter64: // variable is counter, calculate difference from last result
			lastMetric := getChecksLastResultMetricByLabel(&check, oidMonitor.Name)
			if lastMetric != nil {
				//lastValueF, _, err := big.ParseFloat(lastMetric.Value, 10, -1, big.ToNearestEven)

			}
		default:

		}
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
