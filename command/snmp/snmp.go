package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gollector"
	"math/big"
)

type Client interface {
	Connect(cmd *Command) error
	Close() error
	Get(oids []string) ([]Object, error)
}

// Asn1BER is the type of the SNMP PDU
type Asn1BER byte

// Asn1BER's - http://www.ietf.org/rfc/rfc1442.txt
const (
	EndOfContents     Asn1BER = 0x00
	UnknownType       Asn1BER = 0x00
	Boolean           Asn1BER = 0x01
	Integer           Asn1BER = 0x02
	BitString         Asn1BER = 0x03
	OctetString       Asn1BER = 0x04
	Null              Asn1BER = 0x05
	ObjectIdentifier  Asn1BER = 0x06
	ObjectDescription Asn1BER = 0x07
	IPAddress         Asn1BER = 0x40
	Counter32         Asn1BER = 0x41
	Gauge32           Asn1BER = 0x42
	TimeTicks         Asn1BER = 0x43
	Opaque            Asn1BER = 0x44
	NsapAddress       Asn1BER = 0x45
	Counter64         Asn1BER = 0x46
	Uinteger32        Asn1BER = 0x47
	OpaqueFloat       Asn1BER = 0x78
	OpaqueDouble      Asn1BER = 0x79
	NoSuchObject      Asn1BER = 0x80
	NoSuchInstance    Asn1BER = 0x81
	EndOfMibView      Asn1BER = 0x82
)

type Object struct {
	Type  Asn1BER
	Value any
	Oid   string
}

type Command struct {
	client Client

	Addr      string
	Port      uint16
	Community string
	Version   string

	OidMonitors []OidMonitor
}

func (c *Command) SetClient(client Client) {
	c.client = client
}

var (
	DefaultClient = &GoSnmpClient{}
)

func NewCommand(addr, community string, monitors []OidMonitor) *Command {
	return &Command{
		Addr:        addr,
		Port:        161,
		Community:   community,
		OidMonitors: monitors,
	}
}

func (c *Command) Run(check gollector.Check) (result gollector.Result, err error) {
	var client Client
	if c.client == nil {
		client = DefaultClient
	} else {
		client = c.client
	}

	err = client.Connect(c)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	defer func() {
		errC := client.Close()
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

	objects, err := client.Get(rawOids)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}

	var resultMetrics []gollector.ResultMetric
	resultState := gollector.StateUnknown
	var resultReason string

	for _, object := range objects {
		oidMonitor := oidMonitorsByOid[object.Oid]
		if oidMonitor == nil {
			if object.Oid[:1] == "." {
				oidMonitor = oidMonitorsByOid[object.Oid[1:]]
			}
		}
		if oidMonitor == nil {
			return *gollector.MakeUnknownResult("CMD_FAILURE"),
				fmt.Errorf("snmp.Command.Run(): oid %s could not be found in monitors", object.Oid)
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

				// calculate the difference between previous and current result value, accounting for rollover
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
