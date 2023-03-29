package smtp

import (
	"errors"
	"github.com/seankndy/gopoller/check"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
	"time"
)

func TestResultMetricsReturnedProperly(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.On("Connect", mock.Anything).Return(nil)
	mockClient.On("Close", mock.Anything).Return(nil)
	mockClient.On("Cmd", mock.Anything).Return(250, 123451*time.Microsecond, nil)

	cmd := &Command{
		Send:                 "HELO test.local",
		ExpectedResponseCode: 250,
	}
	cmd.SetClient(mockClient)
	result, err := cmd.Run(&check.Check{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []check.ResultMetric{
		{
			Label: "resp",
			Value: "123.451",
			Type:  check.ResultMetricGauge,
		},
	}
	got := result.Metrics

	if !reflect.DeepEqual(want, got) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

func TestReturnsNotReadyResultWhenConnectReturnsNotReadyErr(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.On("Connect", mock.Anything).Return(&NotReadyErr{Cause: errors.New("doesntmatter")})
	mockClient.On("Close", mock.Anything).Return(nil).Once()

	cmd := &Command{
		Send:                 "HELO test.local",
		ExpectedResponseCode: 250,
	}
	cmd.SetClient(mockClient)
	result, err := cmd.Run(&check.Check{})

	mock.AssertExpectationsForObjects(t, mockClient)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if result.State != check.StateCrit {
		t.Errorf("wanted result state %v, got %v", check.StateCrit, result.State)
	}
	if result.ReasonCode != "SMTP_NOT_READY" {
		t.Errorf("wanted result reason code SMTP_NOT_READY, got %v", result.ReasonCode)
	}
}

// TODO: needs some more tests to test threshold tripping

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Connect(command *Command) error {
	args := m.Called(command)
	return args.Error(0)
}

func (m *MockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) Cmd(s string) (int, time.Duration, error) {
	args := m.Called(s)
	return args.Int(0), args.Get(1).(time.Duration), args.Error(2)
}
