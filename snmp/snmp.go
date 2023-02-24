package snmp

import (
	"math/big"
)

type Host struct {
	Addr      string
	Port      uint16
	Community string
	Version   string
	Transport string
}

func NewHost(addr, community string) *Host {
	return &Host{
		Addr:      addr,
		Port:      161,
		Community: community,
		Version:   "2c",
		Transport: "udp",
	}
}

type Getter interface {
	Get(host *Host, oids []string) ([]Object, error)
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

// CalculateCounterDiff calculates the difference between lastValue and currentValue, taking into account rollover at nbits unsigned bits
func CalculateCounterDiff(lastValue *big.Int, currentValue *big.Int, nbits uint8) *big.Int {
	maxCounterValue := new(big.Int).SetUint64(uint64(1<<nbits - 1))
	diff := currentValue.Sub(currentValue, lastValue)
	if diff.Cmp(big.NewInt(0)) < 0 {
		diff = diff.Add(diff, maxCounterValue)
	}
	return diff
}
