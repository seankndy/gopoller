package snmp

import (
	"github.com/gosnmp/gosnmp"
	"time"
)

type GoSnmpClient struct {
	client *gosnmp.GoSNMP
}

func NewGoSnmpClient(target, community string) *GoSnmpClient {
	return &GoSnmpClient{&gosnmp.GoSNMP{
		Target:             target,
		Port:               161,
		Transport:          "udp",
		Community:          community,
		Version:            gosnmp.Version2c,
		Retries:            3,
		Timeout:            2 * time.Second,
		ExponentialTimeout: true,
	}}
}

func (c *GoSnmpClient) SetPort(port uint16) {
	c.client.Port = port
}

func (c *GoSnmpClient) SetVersion(version gosnmp.SnmpVersion) {
	c.client.Version = version
}

func (c *GoSnmpClient) SetTransport(transport string) {
	c.client.Transport = transport
}

func (c *GoSnmpClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}

func (c *GoSnmpClient) SetRetries(retries int) {
	c.client.Retries = retries
}

func (c *GoSnmpClient) Connect() error {
	return c.client.Connect()
}

func (c *GoSnmpClient) Close() error {
	return c.client.Conn.Close()
}

func (c *GoSnmpClient) Get(oids []string) ([]Object, error) {
	numOids := len(oids)
	objects := make([]Object, 0, numOids)

	// if numOids > c.client.MaxOids, chunk them and make ceil(numOids/c.client.MaxOids) SNMP GET requests
	var chunk int
	if numOids > c.client.MaxOids {
		chunk = c.client.MaxOids
	} else {
		chunk = numOids
	}
	for offset := 0; offset < numOids; offset += chunk {
		if chunk > numOids-offset {
			chunk = numOids - offset
		}

		packet, err := c.client.Get(oids[offset : offset+chunk])
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
