package snmp

import (
	"errors"
	"github.com/seankndy/gopoller/check"
	"github.com/seankndy/gopoller/snmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func TestReturnsUnknownResultAndErrorOnSnmpGetFailure(t *testing.T) {
	getterMock := new(MockGetter)
	getterMock.On("Get", mock.Anything).Return([]snmp.Object{}, errors.New("timeout after 3 retries"))

	cmd := &Command{OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo"),
	}}
	cmd.SetGetter(getterMock)
	result, err := cmd.Run(&check.Check{})

	assert.Equal(t, check.StateUnknown, result.State, "invalid result state")
	assert.NotNil(t, err)
}

func TestReturnsResultWithMetricsFromSnmp(t *testing.T) {
	getterMock := new(MockGetter)
	getterMock.On("Get", mock.Anything).Return([]snmp.Object{
		{
			Type:  snmp.Uinteger32,
			Value: uint32(1234567),
			Oid:   "1.2.3.4.5.6.7.8",
		},
		{
			Type:  snmp.Uinteger32,
			Value: uint32(7654321),
			Oid:   "1.2.3.4.5.6.7.8.9",
		},
		{
			Type:  snmp.Counter64,
			Value: uint64(18237189237498),
			Oid:   "1.2.3.4.5.6.7.8.9.1",
		},
	}, nil)

	cmd := &Command{OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo1"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9", "foo2"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9.1", "foo3"),
	}}
	cmd.SetGetter(getterMock)
	result, _ := cmd.Run(&check.Check{})

	assert.Equal(t, []check.ResultMetric{
		{
			Label: "foo1",
			Value: "1234567",
			Type:  check.ResultMetricGauge,
		},
		{
			Label: "foo2",
			Value: "7654321",
			Type:  check.ResultMetricGauge,
		},
		{
			Label: "foo3",
			Value: "18237189237498",
			Type:  check.ResultMetricCounter,
		},
	}, result.Metrics)
}

func TestPostProcessValuesAreAppliedToGauges(t *testing.T) {
	getterMock := new(MockGetter)
	getterMock.On("Get", mock.Anything).Return([]snmp.Object{
		{
			Type:  snmp.Uinteger32,
			Value: uint32(1234567),
			Oid:   "1.2.3.4.5.6.7.8",
		},
	}, nil)

	cmd := &Command{OidMonitors: []OidMonitor{
		{
			Oid:              "1.2.3.4.5.6.7.8",
			Name:             "foo1",
			PostProcessValue: 0.001,
		},
	}}
	cmd.SetGetter(getterMock)
	result, _ := cmd.Run(&check.Check{})
	assert.Equal(t, []check.ResultMetric{
		{
			Label: "foo1",
			Value: "1234.567",
			Type:  check.ResultMetricGauge,
		},
	}, result.Metrics)
}

func TestPostProcessValuesAreNotAppliedToCounters(t *testing.T) {
	getterMock := new(MockGetter)
	getterMock.On("Get", mock.Anything).Return([]snmp.Object{
		{
			Type:  snmp.Counter32,
			Value: uint32(1234567),
			Oid:   "1.2.3.4.5.6.7.8",
		},
		{
			Type:  snmp.Counter64,
			Value: uint64(1234567123131),
			Oid:   "1.2.3.4.5.6.7.9",
		},
	}, nil)

	cmd := &Command{OidMonitors: []OidMonitor{
		{
			Oid:              "1.2.3.4.5.6.7.8",
			Name:             "foo1",
			PostProcessValue: 0.001,
		},
		{
			Oid:              "1.2.3.4.5.6.7.9",
			Name:             "foo2",
			PostProcessValue: 1.1,
		},
	}}
	cmd.SetGetter(getterMock)
	result, _ := cmd.Run(&check.Check{})

	assert.Equal(t, []check.ResultMetric{
		{
			Label: "foo1",
			Value: "1234567",
			Type:  check.ResultMetricCounter,
		},
		{
			Label: "foo2",
			Value: "1234567123131",
			Type:  check.ResultMetricCounter,
		},
	}, result.Metrics)
}

func TestMetricValuesTrippingConfiguredThresholds(t *testing.T) {
	tenSecondsAgo := time.Now().Add(-10 * time.Second)

	tests := []struct {
		name            string
		check           check.Check
		snmpObjects     []snmp.Object
		oidMonitors     []OidMonitor
		wantResultState check.ResultState
		wantReasonCode  string
	}{
		{
			name:  "warn_after_an_ok", // test that if we encounter a WARN after we already got an OK, we get a WARN result
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.8",
				},
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100000000),
					Oid:   "1.2.3.4.5.6.7.9",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:              "1.2.3.4.5.6.7.8",
					Name:             "foo1",
					PostProcessValue: 1.0,
				},
				{
					Oid:               "1.2.3.4.5.6.7.9",
					Name:              "foo2",
					PostProcessValue:  0.00001,
					WarnMinThreshold:  1000000000,
					WarnMinReasonCode: "FOO2_MIN",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "FOO2_MIN",
		},
		{
			name:  "crit_after_a_warn_after_an_ok",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.8",
				},
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100000000),
					Oid:   "1.2.3.4.5.6.7.9",
				},
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.1",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:              "1.2.3.4.5.6.7.8",
					Name:             "foo1",
					PostProcessValue: 1.0,
				},
				{
					Oid:               "1.2.3.4.5.6.7.9",
					Name:              "foo2",
					PostProcessValue:  0.00001,
					WarnMinThreshold:  1000000000,
					WarnMinReasonCode: "FOO2_MIN",
				},
				{
					Oid:               "1.2.3.4.5.6.7.1",
					Name:              "foo3",
					PostProcessValue:  0.00001,
					CritMinThreshold:  101,
					CritMinReasonCode: "FOO3_MIN",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO3_MIN",
		},
		{
			name: "counter64_crit_after_an_ok",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo2",
						Value: "1000000",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.8",
				},
				{
					Type:  snmp.Counter64,
					Value: uint64(1000100),
					Oid:   "1.2.3.4.5.6.7.9",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:              "1.2.3.4.5.6.7.8",
					Name:             "foo1",
					PostProcessValue: 1.0,
				},
				{
					Oid:               "1.2.3.4.5.6.7.9",
					Name:              "foo2",
					PostProcessValue:  1.0,
					CritMinReasonCode: "FOO2_MIN",
					CritMinThreshold:  11,
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO2_MIN",
		},
		{
			name: "counter64_warn_after_an_ok",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo2",
						Value: "1000000",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.8",
				},
				{
					Type:  snmp.Counter64,
					Value: uint64(1000100),
					Oid:   "1.2.3.4.5.6.7.9",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:              "1.2.3.4.5.6.7.8",
					Name:             "foo1",
					PostProcessValue: 1.0,
				},
				{
					Oid:               "1.2.3.4.5.6.7.9",
					Name:              "foo2",
					PostProcessValue:  1.0,
					CritMinReasonCode: "FOO2_MIN",
					CritMinThreshold:  11,
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO2_MIN",
		},
		{
			name: "counter64_ok_after_a_warn",
			check: check.Check{LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
				{
					Label: "foo2",
					Value: "1000000",
					Type:  check.ResultMetricCounter,
				},
			})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(100),
					Oid:   "1.2.3.4.5.6.7.8",
				},
				{
					Type:  snmp.Counter64,
					Value: uint64(1000100),
					Oid:   "1.2.3.4.5.6.7.9",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					WarnMaxThreshold:  99,
					WarnMaxReasonCode: "FOO1_MAX",
				},
				{
					Oid:               "1.2.3.4.5.6.7.9",
					Name:              "foo2",
					PostProcessValue:  1.0,
					CritMinReasonCode: "FOO2_MIN",
					CritMinThreshold:  50,
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "FOO1_MAX",
		},
		{
			name:  "warn_min",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(1234566),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					WarnMinThreshold:  1234567,
					WarnMinReasonCode: "FOO_MIN",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "FOO_MIN",
		},
		{
			name:  "warn_max",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(1234568),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					WarnMaxThreshold:  1234567,
					WarnMaxReasonCode: "FOO_MAX",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "FOO_MAX",
		},
		{
			name:  "crit_min",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(1234566),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					CritMinThreshold:  1234567,
					CritMinReasonCode: "FOO_MIN",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO_MIN",
		},
		{
			name:  "crit_max",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(1234568),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					CritMaxThreshold:  1234567,
					CritMaxReasonCode: "FOO_MAX",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO_MAX",
		},
		{
			name:  "ok",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(500),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					PostProcessValue:  1.0,
					WarnMinThreshold:  100,
					WarnMinReasonCode: "WARN_MIN",
					WarnMaxThreshold:  1000,
					WarnMaxReasonCode: "WARN_MAX",
					CritMinThreshold:  50,
					CritMinReasonCode: "CRIT_MIN",
					CritMaxThreshold:  1500,
					CritMaxReasonCode: "CRIT_MAX",
				},
			},
			wantResultState: check.StateOk,
			wantReasonCode:  "",
		},
		{
			name: "counter_rollover32_warn_min",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo1",
						Value: "4294962295",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Counter32,
					Value: uint32(4999), // previous 4294962295, current 4999, delta 9999
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  999,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  1000,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "WARN_MIN",
		},
		{
			name: "counter_rollover64_warn_min",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo1",
						Value: "18446744073709551515",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Counter64,
					Value: uint32(1099), // previous 18446744073709551515, current 1099, delta 1199
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  118,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  121,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "WARN_MIN",
		},
		{
			name: "counter64_crit_min",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo1",
						Value: "1479408114955",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Counter64,
					Value: uint64(1479408115955),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  101,
					CritMinReasonCode: "CRIT_MIN",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "CRIT_MIN",
		},
		{
			name: "crit_before_warn",
			check: check.Check{
				LastCheck: &tenSecondsAgo,
				LastResult: check.NewResult(check.StateOk, "", []check.ResultMetric{
					{
						Label: "foo1",
						Value: "18446744073709551515",
						Type:  check.ResultMetricCounter,
					},
				})},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Counter64,
					Value: uint32(1099), // previous 18446744073709551515, current 1099, delta 1199
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  120,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  120,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "CRIT_MIN",
		},
		{
			name:  "crit_status_value",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(8),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:                  "1.2.3.4.5.6.7.8",
					Name:                 "foo1",
					PostProcessValue:     1.0,
					CritStatusValue:      8,
					CritStatusReasonCode: "FOO",
				},
			},
			wantResultState: check.StateCrit,
			wantReasonCode:  "FOO",
		},
		{
			name:  "warn_status_value",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Gauge32,
					Value: uint32(8),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:                  "1.2.3.4.5.6.7.8",
					Name:                 "foo1",
					PostProcessValue:     1.0,
					WarnStatusValue:      8,
					WarnStatusReasonCode: "FOO",
				},
			},
			wantResultState: check.StateWarn,
			wantReasonCode:  "FOO",
		},
		{
			name:  "ok_status_value",
			check: check.Check{},
			snmpObjects: []snmp.Object{
				{
					Type:  snmp.Uinteger32,
					Value: uint32(8),
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:                  "1.2.3.4.5.6.7.8",
					Name:                 "foo1",
					PostProcessValue:     1.0,
					CritStatusValue:      9,
					CritStatusReasonCode: "FOO",
				},
			},
			wantResultState: check.StateOk,
			wantReasonCode:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getterMock := new(MockGetter)
			getterMock.On("Get", mock.Anything).Return(tt.snmpObjects, nil)

			cmd := &Command{OidMonitors: tt.oidMonitors}
			cmd.SetGetter(getterMock)
			result, _ := cmd.Run(&tt.check)

			assert.Equal(t, tt.wantResultState, result.State, "invalid result state")
			assert.Equal(t, tt.wantReasonCode, result.ReasonCode, "invalid result reason code")
		})
	}
}

func TestStringGaugeValuesAreParsedAsFloats(t *testing.T) {
	getterMock := new(MockGetter)
	getterMock.On("Get", mock.Anything).Return([]snmp.Object{
		{
			Type:  snmp.OctetString,
			Value: "  0.12345678",
			Oid:   "1.2.3.4.5.6.7.8",
		},
	}, nil)

	cmd := &Command{OidMonitors: []OidMonitor{
		{
			Oid:              "1.2.3.4.5.6.7.8",
			Name:             "foo1",
			PostProcessValue: 1.0,
		},
	}}
	cmd.SetGetter(getterMock)
	result, _ := cmd.Run(&check.Check{})
	assert.Equal(t, []check.ResultMetric{
		{
			Label: "foo1",
			Value: "0.12345678",
			Type:  check.ResultMetricGauge,
		},
	}, result.Metrics)
}

type MockGetter struct {
	mock.Mock
}

func (m *MockGetter) Get(host *snmp.Host, oids []string) ([]snmp.Object, error) {
	args := m.Called(oids)
	return args.Get(0).([]snmp.Object), args.Error(1)
}
