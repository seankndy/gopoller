package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gollector"
	"math/big"
	"time"
)

type Command struct {
	Ip          string
	Community   string
	Version     string
	OidMonitors []OidMonitor
}

func (c *Command) Run(check gollector.Check) (gollector.Result, error) {
	snmp := &gosnmp.GoSNMP{
		Target:             c.Ip,
		Port:               161,
		Transport:          "udp",
		Community:          c.Community,
		Version:            c.getSnmpVersionForGoSnmp(),
		Timeout:            2 * time.Second,
		Retries:            3,
		ExponentialTimeout: true,
		MaxOids:            gosnmp.MaxOids,
	}

	err := snmp.Connect()
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	defer snmp.Conn.Close()

	// create a map of oid->oidMonitors for fast OidMonitor lookup when processing the result values below
	oidMonitorsByOid := make(map[string]*OidMonitor, len(c.OidMonitors))
	// build raw slice of oids from c.OidMonitors to pass to getSnmpVariables()
	rawOids := make([]string, len(c.OidMonitors))
	for k, _ := range c.OidMonitors {
		oidMonitor := &c.OidMonitors[k]
		rawOids[k] = oidMonitor.Oid
		oidMonitorsByOid[oidMonitor.Oid] = oidMonitor
	}

	variables, err := getSnmpVariables(snmp, rawOids)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	for i, variable := range variables {
		oidMonitor := oidMonitorsByOid[variable.Name]
		fmt.Printf("%d: oid: %s ", i, variable.Name)

		valueI := gosnmp.ToBigInt(variable.Value)
		valueF := big.NewFloat(0).SetPrec(uint(valueI.BitLen()))
		valueF.SetInt(valueI)
		valueF.Mul(valueF, big.NewFloat(oidMonitor.PostProcessValue))

		switch variable.Type {
		case gosnmp.Counter32, gosnmp.Counter64: // variable is counter, calculate difference from last result
			lastMetric := getChecksLastResultMetricByLabel(&check, oidMonitor.Name)
			if lastMetric != nil {
				//lastValueF, _, err := big.ParseFloat(lastMetric.Value, 10, -1, big.ToNearestEven)

			}
		default:

		}
	}

	return *gollector.MakeUnknownResult(""), nil
}

func (c *Command) getSnmpVersionForGoSnmp() gosnmp.SnmpVersion {
	switch c.Version {
	case "1":
		return gosnmp.Version1
	case "3":
		return gosnmp.Version3
	default:
		return gosnmp.Version2c
	}
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

func getSnmpVariables(snmp *gosnmp.GoSNMP, oids []string) ([]gosnmp.SnmpPDU, error) {
	numOids := len(oids)
	variables := make([]gosnmp.SnmpPDU, 0, numOids)

	// if numOids > snmp.MaxOids, chunk them and make ceil(numOids/snmp.MaxOids) SNMP GET requests
	var chunk int
	if numOids > snmp.MaxOids {
		chunk = snmp.MaxOids
	} else {
		chunk = numOids
	}
	for offset := 0; offset < numOids; offset += chunk {
		if chunk > numOids-offset {
			chunk = numOids - offset
		}

		result, err := snmp.Get(oids[offset : offset+chunk])
		if err != nil {
			return nil, err
		}

		for _, variable := range result.Variables {
			variables = append(variables, variable)
		}
	}

	return variables, nil
}
