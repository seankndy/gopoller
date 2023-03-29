package snmp

import (
	"fmt"
	"github.com/gosnmp/gosnmp"
	"time"
)

type GoSnmpGetter struct{}

// Get connects to SNMP 'host' and gets the provided oids in chunks, disconnects, and returns an Object slice
func (c *GoSnmpGetter) Get(host *Host, oids []string) ([]Object, error) {
	client, err := c.connect(host)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	objects := make([]Object, 0, len(oids))

	// if len(oids) > client.MaxOids, chunk them and make ceil(len(oids)/client.MaxOids) SNMP GET requests
	var chunk int
	if len(oids) > client.MaxOids {
		chunk = client.MaxOids
	} else {
		chunk = len(oids)
	}
	for offset := 0; offset < len(oids); offset += chunk {
		if chunk > len(oids)-offset {
			chunk = len(oids) - offset
		}

		packet, err := client.Get(oids[offset : offset+chunk])
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

	if len(oids) > 0 && len(objects) == 0 {
		return nil, fmt.Errorf("no snmp objects returned from host")
	}

	return objects, nil
}

func (c *GoSnmpGetter) connect(host *Host) (*gosnmp.GoSNMP, error) {
	var version gosnmp.SnmpVersion
	switch host.Version {
	case "1":
		version = gosnmp.Version1
	case "3":
		version = gosnmp.Version3
	default:
		version = gosnmp.Version2c
	}

	client := &gosnmp.GoSNMP{
		Target:             host.Addr,
		Port:               host.Port,
		Transport:          host.Transport,
		Community:          host.Community,
		Version:            version,
		Retries:            1,
		Timeout:            3 * time.Second,
		ExponentialTimeout: false,
	}

	err := client.Connect()
	if err != nil {
		return nil, err
	}

	return client, nil
}
