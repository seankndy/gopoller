package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"github.com/seankndy/gollector"
	"time"
)

type Command struct {
	Ip        string
	Community string
	Version   string
	Oids      []string
}

func (c Command) Run() (gollector.Result, error) {
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

	variables, err := getSnmpVariables(snmp, c.Oids)
	if err != nil {
		return *gollector.MakeUnknownResult("CMD_FAILURE"), err
	}
	for i, variable := range variables {
		fmt.Printf("%d: oid: %s ", i, variable.Name)

		// the Value of each variable returned by Get() implements
		// interface{}. You could do a type switch...
		switch variable.Type {
		case gosnmp.OctetString:
			bytes := variable.Value.([]byte)
			fmt.Printf("string: %s\n", string(bytes))
		default:
			// ... or often you're just interested in numeric values.
			// ToBigInt() will return the Value as a BigInt, for plugging
			// into your calculations.
			fmt.Printf("number: %d\n", gosnmp.ToBigInt(variable.Value))
		}
	}

	return *gollector.MakeUnknownResult(""), nil
}

func (c Command) getSnmpVersionForGoSnmp() gosnmp.SnmpVersion {
	switch c.Version {
	case "1":
		return gosnmp.Version1
	case "3":
		return gosnmp.Version3
	default:
		return gosnmp.Version2c
	}
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
