package rrdcached

import (
	"fmt"
	"github.com/seankndy/gollector"
	"reflect"
	"testing"
	"time"
)

var lastMock func(file string) (time.Time, error)

func TestDoesNotConnectToRrdCacheDWhenGetRrdFileDefsNil(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(&mockRrdClient, nil)

	check := gollector.Check{}
	result := *gollector.NewResult(gollector.StateOk, "", nil)

	h.Process(check, result, nil)

	if mockRrdClient.ConnectCalled > 0 {
		t.Error("Process() connected to rrdcached unexpectedly")
	}
}

func TestDoesNotConnectToRrdCacheDWhenGetRrdFilesReturnsNil(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(&mockRrdClient, func(gollector.Check, gollector.Result) []RrdFileDef {
		return nil
	})

	check := gollector.Check{}
	result := *gollector.NewResult(gollector.StateOk, "", nil)

	h.Process(check, result, nil)

	if mockRrdClient.ConnectCalled > 0 {
		t.Error("Process() connected to rrdcached unexpectedly")
	}
}

func TestConnectsToRrdCacheDWhenGetRrdFilesReturnsData(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(&mockRrdClient, func(gollector.Check, gollector.Result) []RrdFileDef {
		return []RrdFileDef{
			{Filename: "/foo.rrd"},
		}
	})

	check := gollector.Check{}
	result := *gollector.NewResult(gollector.StateOk, "", nil)

	h.Process(check, result, nil)

	if mockRrdClient.ConnectCalled == 0 {
		t.Error("Process() did not connect to rrdcached when expected")
	}
}

func TestOnlyCreatesRrdFilesThatDontExist(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(&mockRrdClient, func(gollector.Check, gollector.Result) []RrdFileDef {
		return []RrdFileDef{
			{Filename: "/foo1.rrd"},
			{Filename: "/foo2.rrd"},
			{Filename: "/foo3.rrd"},
		}
	})

	check := gollector.Check{}
	result := *gollector.NewResult(gollector.StateOk, "", nil)

	// this will return a successful response for the file /foo1.rrd only
	lastMock = func(file string) (time.Time, error) {
		var t time.Time
		if file == "/foo1.rrd" {
			return t, nil
		}
		return t, fmt.Errorf(file + ": No such file or directory")
	}

	h.Process(check, result, nil)

	lastMock = nil

	if mockRrdClient.CreateCalled > 2 {
		t.Error("Created too many RRD files")
	} else if mockRrdClient.CreateCalled < 2 {
		t.Error("Created too few RRD files")
	}
}

func TestIssuesCorrectBatchUpdateCommands(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(&mockRrdClient, func(gollector.Check, gollector.Result) []RrdFileDef {
		return []RrdFileDef{
			{
				Filename: "/foo1.rrd",
				DataSources: []DS{
					NewDS(fmt.Sprintf("DS:%s:%s:%d:U:U", "metric1", "COUNTER", 600)),
					NewDS(fmt.Sprintf("DS:%s:%s:%d:U:U", "metric2", "GAUGE", 600)),
				},
				RoundRobinArchives: []RRA{
					NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:1:%d", 86400/300)),
					NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:%d:%d", 1800/300, 86400*7/300/(1800/300))),
				},
				Step: 300 * time.Second,
				MetricLabelToDS: map[string]string{
					"mymetric1": "metric1",
					"mymetric2": "metric2",
				},
			},
			{
				Filename: "/foo2.rrd",
				DataSources: []DS{
					NewDS(fmt.Sprintf("DS:%s:%s:%d:U:U", "metric3", "COUNTER", 600)),
					NewDS(fmt.Sprintf("DS:%s:%s:%d:U:U", "metric4", "GAUGE", 600)),
				},
				RoundRobinArchives: []RRA{
					NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:1:%d", 86400/300)),
					NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:%d:%d", 1800/300, 86400*7/300/(1800/300))),
				},
				Step: 300 * time.Second,
				MetricLabelToDS: map[string]string{
					"mymetric3": "metric3",
					"mymetric4": "metric4",
				},
			},
		}
	})

	tm := time.Date(1987, time.August, 21, 7, 0, 0, 0, time.Local)
	check := gollector.Check{}
	result := gollector.Result{
		State:      gollector.StateOk,
		ReasonCode: "",
		Metrics: []gollector.ResultMetric{
			{Label: "mymetric2", Value: "123456"},
			{Label: "mymetric1", Value: "654321"},
			{Label: "mymetric3", Value: "456789"},
			{Label: "mymetric4", Value: "987654"},
		},
		Time: tm,
	}

	h.Process(check, result, nil)

	if mockRrdClient.BatchCalled == 0 {
		t.Error("Batch never called on RRD client")
	} else {
		want := []string{
			"update /foo1.rrd -t metric1:metric2 556549200:654321:123456\n",
			"update /foo2.rrd -t metric3:metric4 556549200:456789:987654\n",
		}
		var got []string
		for _, line := range mockRrdClient.BatchCmds[0] {
			got = append(got, line.String())
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("Bad update commands, wanted %v, got %v", want, got)
		}
	}
}

type MockRrdClient struct {
	ConnectCalled   int
	CloseCalled     int
	CreateCalled    int
	CreateFilenames []string
	BatchCalled     int
	BatchCmds       map[int][]*Cmd
}

func (m *MockRrdClient) Connect() error {
	m.ConnectCalled++
	return nil
}

func (m *MockRrdClient) Close() error {
	m.CloseCalled++
	return nil
}

func (m *MockRrdClient) ExecCmd(cmd *Cmd) ([]string, error) {
	return []string{}, nil
}

func (m *MockRrdClient) Batch(cmd ...*Cmd) error {
	if m.BatchCmds == nil {
		m.BatchCmds = make(map[int][]*Cmd)
	}
	m.BatchCmds[m.BatchCalled] = cmd
	m.BatchCalled++

	return nil
}

func (m *MockRrdClient) Last(filename string) (time.Time, error) {
	if lastMock != nil {
		return lastMock(filename)
	}

	var t time.Time
	return t, nil
}

func (m *MockRrdClient) Create(filename string, ds []DS, rra []RRA, step time.Duration) error {
	m.CreateCalled++

	m.CreateFilenames = append(m.CreateFilenames, filename)

	return nil
}
