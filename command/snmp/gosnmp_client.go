package snmp

import (
	"github.com/gosnmp/gosnmp"
	"time"
)

type GoSnmpClient struct {
	client *gosnmp.GoSNMP
}

func (c *GoSnmpClient) Connect(cmd *Command) error {
	var version gosnmp.SnmpVersion
	switch cmd.Version {
	case "1":
		version = gosnmp.Version1
	case "3":
		version = gosnmp.Version3
	default:
		version = gosnmp.Version2c
	}

	c.client = &gosnmp.GoSNMP{
		Target:             cmd.Addr,
		Port:               cmd.Port,
		Transport:          "udp",
		Community:          cmd.Community,
		Version:            version,
		Retries:            3,
		Timeout:            2 * time.Second,
		ExponentialTimeout: true,
	}

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
