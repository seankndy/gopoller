package rrdcached

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"strings"
	"time"
)

// ClientFactory should return a new RRDCacheD Client which will be called with
// each invocation of Handler.Process()
type ClientFactory func() Client

// Handler processes check result metrics and sends them to a rrdcached server.
type Handler struct {
	clientFactory ClientFactory

	// GetRrdFileDefs should return a slice of RrdFileDefs defining the RRD file specifications for a given Check and
	// it's Result data.
	GetRrdFileDefs func(*check.Check, *check.Result) []RrdFileDef
}

func NewHandler(clientFactory ClientFactory, getRrdFileDefs func(*check.Check, *check.Result) []RrdFileDef) *Handler {
	return &Handler{
		clientFactory:  clientFactory,
		GetRrdFileDefs: getRrdFileDefs,
	}
}

func (h *Handler) Mutate(*check.Check, *check.Result, *check.Incident) {
	return
}

func (h *Handler) Process(chk *check.Check, result *check.Result, _ *check.Incident) (err error) {
	getRrdFileDefs := h.GetRrdFileDefs
	if getRrdFileDefs == nil {
		chk.Debugf("no rrd file def func defined")
		return
	}
	rrdFileDefs := getRrdFileDefs(chk, result)
	if rrdFileDefs == nil {
		chk.Debugf("no rrd file defs returned from GetRrdFileDefs func")
		return
	}

	// create our RRDCacheD client
	client := h.clientFactory()

	// connect to rrdcached
	err = client.Connect()
	if err != nil {
		return fmt.Errorf("error connecting to rrdcached: %v", err)
	}
	defer func() {
		errC := client.Close()
		if errC != nil && err == nil {
			err = fmt.Errorf("error closing connection to rrdcached: %v", errC)
		}
	}()

	rrdFileExists := func(file string) (bool, error) {
		_, err = client.Last(file)
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
		var exists bool
		if exists, err = rrdFileExists(rrdFile.Filename); err != nil {
			return fmt.Errorf("error checking if rrd file exists: %v", err)
		} else if !exists {
			chk.Debugf("rrd file %s does not exist, attempting to create it", rrdFile.Filename)
			if err = client.Create(rrdFile.Filename, rrdFile.DataSources, rrdFile.RoundRobinArchives, rrdFile.Step); err != nil {
				return fmt.Errorf("error creating rrd file: %v", err)
			}
		} else {
			chk.Debugf("rrd file %s exists", rrdFile.Filename)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// update rrd files
	if updateCmds := buildUpdateCommands(rrdFileDefs, result); updateCmds != nil {
		cmdStrings := make([]string, len(updateCmds))
		for i, uc := range updateCmds {
			cmdStrings[i] = strings.TrimSpace(uc.String())
		}
		chk.Debugf("sending BATCH update: %s", strings.Join(cmdStrings, ", "))

		err = client.Batch(updateCmds...)
		if err != nil {
			return fmt.Errorf("error batch-updating rrd files: %v", err)
		}
	}

	return
}

// RrdFileDef defines a rrd file and it's characteristics
type RrdFileDef struct {
	Filename           string
	DataSources        []DS
	RoundRobinArchives []RRA
	Step               time.Duration

	// Optional metric label to data source name mapping.  By default, metric labels will map to DS names identically.
	// Use this if your metric name from the check command is different from your DS name.
	DataSourceToMetricMappings map[string]string
}

func buildUpdateCommands(rrdFileDefs []RrdFileDef, result *check.Result) []*Cmd {
	var updateCmds []*Cmd
	for _, rrdFile := range rrdFileDefs {
		var dsValues []string

		for _, ds := range rrdFile.DataSources {
			metricLabel := ds.Name()

			if rrdFile.DataSourceToMetricMappings != nil {
				if v, ok := rrdFile.DataSourceToMetricMappings[ds.Name()]; ok {
					metricLabel = v
				}
			}

			var metric *check.ResultMetric
			for _, m := range result.Metrics {
				if m.Label == metricLabel {
					metric = &m
					break
				}
			}
			if metric != nil {
				dsValues = append(dsValues, metric.Value)
			}
		}

		updateCmds = append(updateCmds, NewCmd("update").WithArgs(
			rrdFile.Filename,
			fmt.Sprintf("%d:%s", result.Time.Unix(), strings.Join(dsValues, ":")),
		))
	}
	return updateCmds
}
