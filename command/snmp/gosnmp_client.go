package snmp

import (
	"github.com/gosnmp/gosnmp"
)

type GoSnmpClient struct {
	Client *gosnmp.GoSNMP
}

func NewGoSnmpClient(client *gosnmp.GoSNMP) *GoSnmpClient {
	return &GoSnmpClient{client}
}

func (c *GoSnmpClient) Connect() error {
	return c.Client.Connect()
}

func (c *GoSnmpClient) Close() error {
	return c.Client.Conn.Close()
}

func (c *GoSnmpClient) Get(oids []string) ([]Object, error) {
	numOids := len(oids)
	objects := make([]Object, 0, numOids)

	// if numOids > c.client.MaxOids, chunk them and make ceil(numOids/c.Client.MaxOids) SNMP GET requests
	var chunk int
	if numOids > c.Client.MaxOids {
		chunk = c.Client.MaxOids
	} else {
		chunk = numOids
	}
	for offset := 0; offset < numOids; offset += chunk {
		if chunk > numOids-offset {
			chunk = numOids - offset
		}

		packet, err := c.Client.Get(oids[offset : offset+chunk])
		if err != nil {
			return nil, err
		}

		// transform gosnmp variables into Objects
		for _, v := range packet.Variables {
			objects = append(objects, Object{
				Type:  Asn1BER(v.Type),
				Value: v.Value,
				Oid:   v.Name,
			})
		}
	}

	return objects, nil
}
