package rrdcached

import (
	"fmt"
	"github.com/multiplay/go-rrd"
	"github.com/seankndy/gollector"
	"strings"
	"time"
)

// Handler processes check result metrics and sends them to a rrdcached server.
type Handler struct {
	addr    string
	timeout time.Duration

	// GetRrdFileDefs should return a slice of RrdFileDefs defining the RRD files and ResultMetric label-to-ds mappings
	GetRrdFileDefs func(gollector.Check, gollector.Result) []RrdFileDef
}

// RrdFileDef defines a rrd file and it's characteristics
type RrdFileDef struct {
	Filename           string
	DataSources        []DS
	RoundRobinArchives []RRA
	Step               time.Duration

	MetricLabelToDS map[string]string
}

func getRrdFilenameAndDsForMetric(fileDefs []RrdFileDef, metric gollector.ResultMetric) (*string, *string) {
	for _, file := range fileDefs {
		if dsName, ok := file.MetricLabelToDS[metric.Label]; ok {
			return &file.Filename, &dsName
		}
	}
	return nil, nil
}

// DS represents a RRD data source definition
type DS string

func (v DS) String() string {
	return string(v)
}

func (v DS) GetName() string {
	parts := strings.Split(string(v), ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// RRA represents a RRD round-robin archive definition
type RRA string

func (v RRA) String() string {
	return string(v)
}

func NewDS(raw string) DS {
	return DS(raw)
}

func NewRRA(raw string) RRA {
	return RRA(raw)
}

func NewHandler(addr string, timeout time.Duration, getRrdFileDefs func(gollector.Check, gollector.Result) []RrdFileDef) *Handler {
	return &Handler{
		addr:           addr,
		timeout:        timeout,
		GetRrdFileDefs: getRrdFileDefs,
	}
}

func (h Handler) Mutate(check *gollector.Check, result *gollector.Result, newIncident *gollector.Incident) {
	return
}

func (h Handler) Process(check gollector.Check, result gollector.Result, newIncident *gollector.Incident) (err error) {
	getRrdFileDefs := h.GetRrdFileDefs
	if getRrdFileDefs == nil {
		return
	}
	rrdFileDefs := getRrdFileDefs(check, result)
	if rrdFileDefs == nil {
		return
	}

	// connect to rrdcached
	client, err := h.createRrdClient()
	defer func() {
		errC := client.Close()
		if err == nil {
			err = errC
		}
	}()

	rrdFileExists := func(file string) bool {
		if _, err := client.Last(file); err != nil && strings.Contains(err.Error(), "No such file") {
			return false
		}
		return true
	}

	// create rrd files that don't exist
	for _, rrdFile := range rrdFileDefs {
		if !rrdFileExists(rrdFile.Filename) {
			dataSources := make([]rrd.DS, len(rrdFile.DataSources))
			for i := 0; i < len(rrdFile.DataSources); i++ {
				dataSources[i] = rrd.NewDS(rrdFile.DataSources[i].String())
			}
			roundRobinArchives := make([]rrd.RRA, len(rrdFile.RoundRobinArchives))
			for i := 0; i < len(rrdFile.RoundRobinArchives); i++ {
				roundRobinArchives[i] = rrd.NewRRA(rrdFile.RoundRobinArchives[i].String())
			}

			if err := client.Create(rrdFile.Filename, dataSources, roundRobinArchives, rrd.Step(rrdFile.Step)); err != nil {
				return
			}
		}
	}

	// update rrd files
	var updateCmds []*rrd.Cmd
	for _, rrdFile := range rrdFileDefs {
		var dsNames, dsValues []string
		for metricLabel, dsName := range rrdFile.MetricLabelToDS {
			var metric *gollector.ResultMetric

			for _, m := range result.Metrics {
				if m.Label == metricLabel {
					metric = &m
					break
				}
			}

			if metric != nil {
				dsNames = append(dsNames, dsName)
				dsValues = append(dsValues, metric.Value)
			}
		}

		updateCmds = append(updateCmds, rrd.NewCmd("update").WithArgs(
			rrdFile.Filename,
			"-t "+strings.Join(dsNames, ":"),
			fmt.Sprintf("%d:%s", result.Time.Unix(), strings.Join(dsValues, ":")),
		))
	}
	if updateCmds != nil {
		err := client.Batch(updateCmds...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h Handler) createRrdClient() (*rrd.Client, error) {
	var client *rrd.Client
	var err error
	addr := h.addr
	if addr[:7] == "unix://" {
		addr = addr[7:]
		client, err = rrd.NewClient(addr, rrd.Timeout(h.timeout), rrd.Unix)
	} else {
		if addr[:6] == "tcp://" {
			addr = addr[6:]
		}
		client, err = rrd.NewClient(addr, rrd.Timeout(h.timeout))
	}
	return client, err
}

// example getspecs func:
func getSpecs(check gollector.Check, result gollector.Result) *RrdSpec {
	_, isPeriodic := check.Schedule.(gollector.PeriodicSchedule)
	// no spec if no metrics or if the underlying check isn't on an interval schedule
	if result.Metrics == nil || !isPeriodic {
		return nil
	}

	interval := check.Schedule.(gollector.PeriodicSchedule).IntervalSeconds

	rrdDsName := func(metric gollector.ResultMetric) string {
		label := metric.Label
		// RRD DS can only be 19 chars max
		if len(label) > 19 {
			label = label[0:19]
		}
		return label
	}

	spec := RrdSpec{}
	for _, metric := range result.Metrics {
		dsName := rrdDsName(metric)
		var dsType string
		if metric.Type == gollector.ResultMetricCounter {
			dsType = "COUNTER"
		} else {
			dsType = "GAUGE"
		}
		dsStep := interval * 2

		weeklyAvg := 1800
		monthlyAvg := 7200
		yearlyAvg := 43200

		spec.Files = append(spec.Files, RrdFile{
			Filename: "/Users/sean/rrd_test/" + check.Id + "/" + dsName,
			DataSources: []DS{
				NewDS(fmt.Sprintf("DS:%s:%s:%d:U:U", rrdDsName(metric), dsType, dsStep)),
			},
			RoundRobinArchives: []RRA{
				NewRRA(fmt.Sprintf("RRA:MIN:0.5:1:%d", 86400/interval)),
				NewRRA(fmt.Sprintf("RRA:MIN:0.5:%d:%d", weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:MIN:0.5:%d:%d", monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:MIN:0.5:%d:%d", yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval))),

				NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:1:%d", 86400/interval)),
				NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:%d:%d", weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:%d:%d", monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:AVERAGE:0.5:%d:%d", yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval))),

				NewRRA(fmt.Sprintf("RRA:MAX:0.5:1:%d", 86400/interval)),
				NewRRA(fmt.Sprintf("RRA:MAX:0.5:%d:%d", weeklyAvg/interval, 86400*7/interval/(weeklyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:MAX:0.5:%d:%d", monthlyAvg/interval, 86400*31/interval/(monthlyAvg/interval))),
				NewRRA(fmt.Sprintf("RRA:MAX:0.5:%d:%d", yearlyAvg/interval, 86400*366/interval/(yearlyAvg/interval))),
			},
			Step: time.Duration(interval) * time.Second,
		})

		spec.MetricLabelToDS[metric.Label] = dsName
	}
	return &spec
}
