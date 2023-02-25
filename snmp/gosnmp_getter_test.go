package snmp

import (
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestBasicGetQuery(t *testing.T) {
	addr, port, community := getTestingAddrPortAndCommunity(t)

	goSnmpGetter := new(GoSnmpGetter)
	objects, err := goSnmpGetter.Get(&Host{
		Addr:      addr,
		Port:      port,
		Community: community,
		Version:   "2c",
		Transport: "udp",
	}, []string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.5.0"}) // sysDescr and sysName

	require.Nil(t, err)
	require.Len(t, objects, 2)
	require.Equal(t, objects[0].Oid, ".1.3.6.1.2.1.1.1.0")
	require.Equal(t, objects[1].Oid, ".1.3.6.1.2.1.1.5.0")
	require.Equal(t, objects[0].Type, OctetString)
	require.Equal(t, objects[1].Type, OctetString)
	require.Greater(t, len(objects[0].Value.([]byte)), 0)
	require.Greater(t, len(objects[1].Value.([]byte)), 0)
}

func getTestingAddrPortAndCommunity(t *testing.T) (string, uint16, string) {
	addr := os.Getenv("GOPOLLER_SNMP_ADDR")
	community := os.Getenv("GOPOLLER_SNMP_COMMUNITY")
	if community == "" {
		community = "public"
	}

	if addr == "" {
		t.Skip("GOPOLLER_SNMP_ADDR env variable is not set, skipping test")
	}

	parts := strings.Split(addr, ":")
	var port uint64
	if len(parts) == 1 {
		port = 161
	} else {
		addr = parts[0]
		var err error
		port, err = strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			t.Skipf("invalid port: %v, skipping test\n", parts[1])
		}
	}

	return addr, uint16(port), community
}
