package rrdcached

import (
	"fmt"
	"github.com/seankndy/gollector"
	"strings"
	"time"
)

// Handler processes check result metrics and sends them to a rrdcached server.
type Handler struct {
	client Client

	// GetRrdFileDefs should return a slice of RrdFileDefs defining the RRD files and ResultMetric label-to-ds mappings
	GetRrdFileDefs func(gollector.Check, gollector.Result) []RrdFileDef
}

func NewHandler(client Client, getRrdFileDefs func(gollector.Check, gollector.Result) []RrdFileDef) *Handler {
	return &Handler{
		client:         client,
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
	err = h.client.Connect()
	defer func() {
		errC := h.client.Close()
		if err == nil {
			err = errC
		}
	}()

	rrdFileExists := func(file string) (bool, error) {
		_, err := h.client.Last(file)
		if err != nil {
			if strings.Contains(err.Error(), "No such file") {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	// create rrd files that don't exist
	for _, rrdFile := range rrdFileDefs {
		exists, err := rrdFileExists(rrdFile.Filename)
		if err != nil {
			return err
		}
		if !exists {
			if err := h.client.Create(rrdFile.Filename, rrdFile.DataSources, rrdFile.RoundRobinArchives, rrdFile.Step); err != nil {
				return err
			}
		}
	}

	// update rrd files
	if updateCmds := buildUpdateCommands(rrdFileDefs, result); updateCmds != nil {
		err := h.client.Batch(updateCmds...)
		if err != nil {
			return err
		}
	}

	return nil
}

// RrdFileDef defines a rrd file and it's characteristics
type RrdFileDef struct {
	Filename           string
	DataSources        []DS
	RoundRobinArchives []RRA
	Step               time.Duration

	// TODO: move metric labels to DataSources directly?
	MetricLabelToDS map[string]string
}

func buildUpdateCommands(rrdFileDefs []RrdFileDef, result gollector.Result) []*Cmd {
	var updateCmds []*Cmd
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

		updateCmds = append(updateCmds, NewCmd("update").WithArgs(
			rrdFile.Filename,
			"-t "+strings.Join(dsNames, ":"),
			fmt.Sprintf("%d:%s", result.Time.Unix(), strings.Join(dsValues, ":")),
		))
	}
	return updateCmds
}

// example getRrdFileDefs func:
func getRrdFileDefs(check gollector.Check, result gollector.Result) []RrdFileDef {
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

	var rrdFileDefs []RrdFileDef
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

		rrdFileDefs = append(rrdFileDefs, RrdFileDef{
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
			MetricLabelToDS: map[string]string{
				metric.Label: dsName,
			},
		})
	}
	return rrdFileDefs
}
