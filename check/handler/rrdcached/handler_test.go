package rrdcached

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"reflect"
	"testing"
	"time"
)

var lastMock func(file string) (time.Time, error)

func TestDoesNotConnectToRrdCacheDWhenGetRrdFileDefsNil(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(func() Client { return &mockRrdClient }, nil)

	chk := &check.Check{}
	result := check.NewResult(check.StateOk, "", nil)

	h.Process(chk, result, nil)

	if mockRrdClient.ConnectCalled > 0 {
		t.Error("Process() connected to rrdcached unexpectedly")
	}
}

func TestDoesNotConnectToRrdCacheDWhenGetRrdFilesReturnsNil(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(func() Client { return &mockRrdClient }, func(*check.Check, *check.Result) []RrdFileDef {
		return nil
	})

	chk := &check.Check{}
	result := check.NewResult(check.StateOk, "", nil)

	h.Process(chk, result, nil)

	if mockRrdClient.ConnectCalled > 0 {
		t.Error("Process() connected to rrdcached unexpectedly")
	}
}

func TestConnectsToRrdCacheDWhenGetRrdFilesReturnsData(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(func() Client { return &mockRrdClient }, func(*check.Check, *check.Result) []RrdFileDef {
		return []RrdFileDef{
			{Filename: "/foo.rrd"},
		}
	})

	chk := &check.Check{}
	result := check.NewResult(check.StateOk, "", nil)

	h.Process(chk, result, nil)

	if mockRrdClient.ConnectCalled == 0 {
		t.Error("Process() did not connect to rrdcached when expected")
	}
}

func TestOnlyCreatesRrdFilesThatDontExist(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(func() Client { return &mockRrdClient }, func(*check.Check, *check.Result) []RrdFileDef {
		return []RrdFileDef{
			{Filename: "/foo1.rrd"},
			{Filename: "/foo2.rrd"},
			{Filename: "/foo3.rrd"},
		}
	})

	chk := &check.Check{}
	result := check.NewResult(check.StateOk, "", nil)

	// this will return a successful response for the file /foo1.rrd only
	lastMock = func(file string) (time.Time, error) {
		var t time.Time
		if file == "/foo1.rrd" {
			return t, nil
		}
		return t, fmt.Errorf(file + ": No such file or directory")
	}

	h.Process(chk, result, nil)

	lastMock = nil

	if mockRrdClient.CreateCalled > 2 {
		t.Error("Created too many RRD files")
	} else if mockRrdClient.CreateCalled < 2 {
		t.Error("Created too few RRD files")
	}
}

func TestIssuesCorrectBatchUpdateCommands(t *testing.T) {
	mockRrdClient := MockRrdClient{}
	h := NewHandler(func() Client { return &mockRrdClient }, func(*check.Check, *check.Result) []RrdFileDef {
		return []RrdFileDef{
			{
				Filename: "/foo1.rrd",
				DataSources: []DS{
					NewCounterDS("metric1", 600, "U", "U"),
					NewGaugeDS("metric2", 600, "U", "U"),
				},
				RoundRobinArchives: []RRA{
					NewAverageRRA(0.5, 1, 86400/300),
				},
				Step: 300 * time.Second,
				DataSourceToMetricMappings: map[string]string{
					"metric1": "mymetric1",
					"metric2": "mymetric2",
				},
			},
			{
				Filename: "/foo2.rrd",
				DataSources: []DS{
					NewCounterDS("metric3", 600, "U", "U"),
					NewGaugeDS("metric4", 600, "U", "U"),
				},
				RoundRobinArchives: []RRA{
					NewAverageRRA(0.5, 1, 86400/300),
				},
				Step: 300 * time.Second,
			},
		}
	})

	tm := time.Unix(556549200, 0)
	chk := &check.Check{}
	result := &check.Result{
		State:      check.StateOk,
		ReasonCode: "",
		Metrics: []check.ResultMetric{
			{Label: "mymetric2", Value: "123456"},
			{Label: "mymetric1", Value: "654321"},
			{Label: "metric3", Value: "456789"},
			{Label: "metric4", Value: "987654"},
			{Label: "mymetric5", Value: "987654"}, // undefined by rrdfile spec, so should not show up in updates
		},
		Time: tm,
	}

	h.Process(chk, result, nil)

	if mockRrdClient.BatchCalled == 0 {
		t.Error("Batch never called on RRD client")
	} else {
		want := []string{
			"update /foo1.rrd 556549200:654321:123456\n",
			"update /foo2.rrd 556549200:456789:987654\n",
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
