package snmp

import (
	"errors"
	"fmt"
	"github.com/seankndy/gollector"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

func TestReturnsUnknownResultAndErrorOnSnmpConnectFailure(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect", mock.Anything).Return(errors.New("could not reach host"))

	cmd := &Command{}
	cmd.SetClient(clientMock)

	result, err := cmd.Run(gollector.Check{})
	if result.State != gollector.StateUnknown {
		t.Errorf("wanted state to be %v, got %v", gollector.StateUnknown, result.State)
	}
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestReturnsUnknownResultAndErrorOnSnmpGetFailure(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect", mock.Anything).Return(nil)
	clientMock.On("Close").Return(nil)
	clientMock.On("Get", mock.Anything).Return([]Object{}, errors.New("timeout after 3 retries"))

	cmd := &Command{OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo"),
	}}
	cmd.SetClient(clientMock)

	result, err := cmd.Run(gollector.Check{})
	if result.State != gollector.StateUnknown {
		t.Errorf("wanted state to be %v, got %v", gollector.StateUnknown, result.State)
	}
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestReturnsResultWithMetricsFromSnmp(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect", mock.Anything).Return(nil)
	clientMock.On("Close").Return(nil)
	clientMock.On("Get", mock.Anything).Return([]Object{
		{
			Type:  Uinteger32,
			Value: uint32(1234567),
			Oid:   "1.2.3.4.5.6.7.8",
		},
		{
			Type:  Uinteger32,
			Value: uint32(7654321),
			Oid:   "1.2.3.4.5.6.7.8.9",
		},
		{
			Type:  Counter64,
			Value: uint64(18237189237498),
			Oid:   "1.2.3.4.5.6.7.8.9.1",
		},
	}, nil)

	cmd := &Command{OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo1"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9", "foo2"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9.1", "foo3"),
	}}
	cmd.SetClient(clientMock)

	result, _ := cmd.Run(gollector.Check{})
	want := []gollector.ResultMetric{
		{
			Label: "foo1",
			Value: "1234567",
			Type:  gollector.ResultMetricGauge,
		},
		{
			Label: "foo2",
			Value: "7654321",
			Type:  gollector.ResultMetricGauge,
		},
		{
			Label: "foo3",
			Value: "18237189237498",
			Type:  gollector.ResultMetricCounter,
		},
	}
	got := result.Metrics
	if !reflect.DeepEqual(want, got) {
		t.Errorf("invalid metrics returned: wanted %v, got %v", want, got)
	}
}

func TestPostProcessValuesAreAppliedToGauges(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect", mock.Anything).Return(nil)
	clientMock.On("Close").Return(nil)
	clientMock.On("Get", mock.Anything).Return([]Object{
		{
			Type:  Uinteger32,
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
	cmd.SetClient(clientMock)
	result, _ := cmd.Run(gollector.Check{})
	want := []gollector.ResultMetric{
		{
			Label: "foo1",
			Value: "1234.567",
			Type:  gollector.ResultMetricGauge,
		},
	}
	got := result.Metrics
	if !reflect.DeepEqual(want, got) {
		t.Errorf("invalid metrics returned: wanted %v, got %v", want, got)
	}
}

func TestPostProcessValuesAreNotAppliedToCounters(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect", mock.Anything).Return(nil)
	clientMock.On("Close").Return(nil)
	clientMock.On("Get", mock.Anything).Return([]Object{
		{
			Type:  Counter32,
			Value: uint32(1234567),
			Oid:   "1.2.3.4.5.6.7.8",
		},
		{
			Type:  Counter64,
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
	cmd.SetClient(clientMock)

	result, _ := cmd.Run(gollector.Check{})
	want := []gollector.ResultMetric{
		{
			Label: "foo1",
			Value: "1234567",
			Type:  gollector.ResultMetricCounter,
		},
		{
			Label: "foo2",
			Value: "1234567123131",
			Type:  gollector.ResultMetricCounter,
		},
	}
	got := result.Metrics
	if !reflect.DeepEqual(want, got) {
		t.Errorf("invalid metrics returned: wanted %v, got %v", want, got)
	}
}

func TestMetricValuesTrippingConfiguredThresholds(t *testing.T) {
	tests := []struct {
		name            string
		check           gollector.Check
		snmpObjects     []Object
		oidMonitors     []OidMonitor
		wantResultState gollector.ResultState
		wantReasonCode  string
	}{
		{
			name:  "warn_min",
			check: gollector.Check{},
			snmpObjects: []Object{
				{
					Type:  Uinteger32,
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
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "FOO_MIN",
		},
		{
			name:  "warn_max",
			check: gollector.Check{},
			snmpObjects: []Object{
				{
					Type:  Uinteger32,
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
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "FOO_MAX",
		},
		{
			name:  "crit_min",
			check: gollector.Check{},
			snmpObjects: []Object{
				{
					Type:  Uinteger32,
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
			wantResultState: gollector.StateCrit,
			wantReasonCode:  "FOO_MIN",
		},
		{
			name:  "crit_max",
			check: gollector.Check{},
			snmpObjects: []Object{
				{
					Type:  Uinteger32,
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
			wantResultState: gollector.StateCrit,
			wantReasonCode:  "FOO_MAX",
		},
		{
			name:  "ok",
			check: gollector.Check{},
			snmpObjects: []Object{
				{
					Type:  Uinteger32,
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
			wantResultState: gollector.StateOk,
			wantReasonCode:  "",
		},
		{
			name: "counter_rollover32_warn_min",
			check: gollector.Check{LastResult: gollector.NewResult(gollector.StateOk, "", []gollector.ResultMetric{
				{
					Label: "foo1",
					Value: "4294962295",
					Type:  gollector.ResultMetricCounter,
				},
			})},
			snmpObjects: []Object{
				{
					Type:  Counter32,
					Value: uint32(4999), // previous 4294962295, current 4999, delta 9999
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  9998,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  10000,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "WARN_MIN",
		},
		{
			name: "counter_rollover64_warn_min",
			check: gollector.Check{LastResult: gollector.NewResult(gollector.StateOk, "", []gollector.ResultMetric{
				{
					Label: "foo1",
					Value: "18446744073709551515",
					Type:  gollector.ResultMetricCounter,
				},
			})},
			snmpObjects: []Object{
				{
					Type:  Counter64,
					Value: uint32(1099), // previous 18446744073709551515, current 1099, delta 1199
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  1198,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  1200,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "WARN_MIN",
		},
		{
			name: "crit_before_warn",
			check: gollector.Check{LastResult: gollector.NewResult(gollector.StateOk, "", []gollector.ResultMetric{
				{
					Label: "foo1",
					Value: "18446744073709551515",
					Type:  gollector.ResultMetricCounter,
				},
			})},
			snmpObjects: []Object{
				{
					Type:  Counter64,
					Value: uint32(1099), // previous 18446744073709551515, current 1099, delta 1199
					Oid:   "1.2.3.4.5.6.7.8",
				},
			},
			oidMonitors: []OidMonitor{
				{
					Oid:               "1.2.3.4.5.6.7.8",
					Name:              "foo1",
					CritMinThreshold:  1200,
					CritMinReasonCode: "CRIT_MIN",
					WarnMinThreshold:  1200,
					WarnMinReasonCode: "WARN_MIN",
				},
			},
			wantResultState: gollector.StateCrit,
			wantReasonCode:  "CRIT_MIN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientMock := new(MockClient)
			clientMock.On("Connect", mock.Anything).Return(nil)
			clientMock.On("Close").Return(nil)
			clientMock.On("Get", mock.Anything).Return(tt.snmpObjects, nil)

			cmd := &Command{OidMonitors: tt.oidMonitors}
			cmd.SetClient(clientMock)

			result, _ := cmd.Run(tt.check)

			fmt.Println(result)

			{
				want := tt.wantResultState
				got := result.State
				if want != got {
					t.Errorf("wanted result state %v, got %v", want, got)
				}
			}
			{
				want := tt.wantReasonCode
				got := result.ReasonCode
				if want != got {
					t.Errorf("wanted reason code %v, got %v", want, got)
				}
			}
		})
	}
}

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Connect(cmd *Command) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) Get(oids []string) ([]Object, error) {
	args := m.Called(oids)
	return args.Get(0).([]Object), args.Error(1)
}
