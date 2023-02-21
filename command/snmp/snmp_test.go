package snmp

import (
	"errors"
	"github.com/seankndy/gollector"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

func TestReturnsUnknownResultAndErrorOnSnmpConnectFailure(t *testing.T) {
	clientMock := new(MockClient)
	clientMock.On("Connect").Return(errors.New("could not reach host"))

	cmd := &Command{Client: clientMock}
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
	clientMock.On("Connect").Return(nil)
	clientMock.On("Close").Return(nil)
	clientMock.On("Get", mock.Anything).Return([]Object{}, errors.New("timeout after 3 retries"))

	cmd := &Command{Client: clientMock, OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo"),
	}}
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
	clientMock.On("Connect").Return(nil)
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

	cmd := &Command{Client: clientMock, OidMonitors: []OidMonitor{
		*NewOidMonitor("1.2.3.4.5.6.7.8", "foo1"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9", "foo2"),
		*NewOidMonitor("1.2.3.4.5.6.7.8.9.1", "foo3"),
	}}
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

// TODO: test oid monitor thresholds

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Connect() error {
	args := m.Called()
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
