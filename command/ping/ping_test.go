package ping

import (
	"errors"
	"github.com/seankndy/gollector"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
	"time"
)

func TestResultMetricsReturnedProperly(t *testing.T) {
	mockPinger := new(MockPinger)
	mockPinger.On("Run", mock.Anything).Return(&PingerStats{
		PacketLoss: 69.2,
		AvgRtt:     23450 * time.Microsecond,
		StdDevRtt:  12340 * time.Microsecond,
	}, nil)

	cmd := &Command{}
	cmd.SetPinger(mockPinger)
	result, err := cmd.Run(gollector.Check{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []gollector.ResultMetric{
		{Label: "avg", Value: "23.45"},
		{Label: "jitter", Value: "12.34"},
		{Label: "loss", Value: "69.20"},
	}
	got := result.Metrics

	if !reflect.DeepEqual(want, got) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

func TestThresholdsTripStatesProperly(t *testing.T) {
	tests := []struct {
		name            string
		cmd             *Command
		stats           *PingerStats
		wantResultState gollector.ResultState
		wantReasonCode  string
	}{
		{
			name: "ploss_warn",
			cmd: &Command{
				PacketLossWarnThreshold: 69,
				PacketLossCritThreshold: 70,
				AvgRttWarnThreshold:     50,
				AvgRttCritThreshold:     60,
			},
			stats: &PingerStats{
				PacketLoss: 69.2,
				AvgRtt:     23450 * time.Microsecond,
				StdDevRtt:  12340 * time.Microsecond,
			},
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "PKT_LOSS_HIGH",
		},
		{
			name: "ploss_crit",
			cmd: &Command{
				PacketLossWarnThreshold: 69,
				PacketLossCritThreshold: 70,
				AvgRttWarnThreshold:     50,
				AvgRttCritThreshold:     60,
			},
			stats: &PingerStats{
				PacketLoss: 70.1,
				AvgRtt:     23450 * time.Microsecond,
				StdDevRtt:  12340 * time.Microsecond,
			},
			wantResultState: gollector.StateCrit,
			wantReasonCode:  "PKT_LOSS_HIGH",
		},
		{
			name: "avg_rtt_warn",
			cmd: &Command{
				PacketLossWarnThreshold: 0,
				PacketLossCritThreshold: 5,
				AvgRttWarnThreshold:     23449 * time.Microsecond,
				AvgRttCritThreshold:     23451 * time.Microsecond,
			},
			stats: &PingerStats{
				PacketLoss: 0,
				AvgRtt:     23450 * time.Microsecond,
				StdDevRtt:  12340 * time.Microsecond,
			},
			wantResultState: gollector.StateWarn,
			wantReasonCode:  "LATENCY_HIGH",
		},
		{
			name: "avg_rtt_crit",
			cmd: &Command{
				PacketLossWarnThreshold: 0,
				PacketLossCritThreshold: 5,
				AvgRttWarnThreshold:     23448 * time.Microsecond,
				AvgRttCritThreshold:     23449 * time.Microsecond,
			},
			stats: &PingerStats{
				PacketLoss: 0,
				AvgRtt:     23450 * time.Microsecond,
				StdDevRtt:  12340 * time.Microsecond,
			},
			wantResultState: gollector.StateCrit,
			wantReasonCode:  "LATENCY_HIGH",
		},
		{
			name: "all_ok",
			cmd: &Command{
				PacketLossWarnThreshold: 3,
				PacketLossCritThreshold: 5,
				AvgRttWarnThreshold:     20 * time.Millisecond,
				AvgRttCritThreshold:     25 * time.Millisecond,
			},
			stats: &PingerStats{
				PacketLoss: 0.5,
				AvgRtt:     19 * time.Millisecond,
				StdDevRtt:  5 * time.Millisecond,
			},
			wantResultState: gollector.StateOk,
			wantReasonCode:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPinger := new(MockPinger)
			mockPinger.On("Run", mock.Anything).Return(tt.stats, nil)

			tt.cmd.SetPinger(mockPinger)

			result, err := tt.cmd.Run(gollector.Check{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantResultState != result.State {
				t.Errorf("wanted result state %v, got %v", tt.wantResultState, result.State)
			}

			if tt.wantReasonCode != result.ReasonCode {
				t.Errorf("wanted result reason code %v, got %v", tt.wantReasonCode, result.ReasonCode)
			}
		})
	}
}

func TestReturnsUnknownResultAndErrOnPingerError(t *testing.T) {
	mockPinger := new(MockPinger)
	mockPinger.On("Run", mock.Anything).Return(&PingerStats{}, errors.New("some error happened"))

	cmd := &Command{}
	cmd.SetPinger(mockPinger)
	result, err := cmd.Run(gollector.Check{})
	if err == nil {
		t.Error("expected error, got nil")
	} else if err.Error() != "some error happened" {
		t.Errorf("expected error with 'some error happened', got %v", err)
	}

	if result.State != gollector.StateUnknown {
		t.Errorf("wanted result state %v, got %v", gollector.StateUnknown, result.State)
	}
	if result.ReasonCode != "CMD_FAILURE" {
		t.Errorf("wanted result reason code CMD_FAILURE, got %v", result.ReasonCode)
	}
}

type MockPinger struct {
	mock.Mock
}

func (m *MockPinger) Run(cmd *Command) (*PingerStats, error) {
	args := m.Called(cmd)
	return args.Get(0).(*PingerStats), args.Error(1)
}
