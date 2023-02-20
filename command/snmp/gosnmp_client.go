package snmp

import (
	"github.com/gosnmp/gosnmp"
)

type GoSnmpClient struct {
	Client *gosnmp.GoSNMP
}

func NewGoSnmpClient(client *gosnmp.GoSNMP) *GoSnmpClient {
	return &GoSnmpClient{Client: client}
}

func (c *GoSnmpClient) Connect() error {
	return c.Client.Connect()
}

func (c *GoSnmpClient) Close() error {
	return c.Client.Conn.Close()
}

func (c *GoSnmpClient) Get(oids []string) ([]GetResultVariable, error) {
	result, err := c.Client.Get(oids)
	if err != nil {
		return nil, err
	}

	// transform gosnmp variables into GetResults
	getResults := make([]GetResultVariable, len(oids))
	for _, variable := range result.Variables {
		getResults = append(getResults, GetResultVariable{
			Type:  Asn1BER(variable.Type),
			Value: variable.Value,
			Oid:   variable.Name,
		})
	}
	return getResults, nil
}

func (c *GoSnmpClient) MaxOids() int {
	return c.Client.MaxOids
}

func getSnmpVersionForGoSnmp(version string) gosnmp.SnmpVersion {
	switch version {
	case "1":
		return gosnmp.Version1
	case "3":
		return gosnmp.Version3
	default:
		return gosnmp.Version2c
	}
}
