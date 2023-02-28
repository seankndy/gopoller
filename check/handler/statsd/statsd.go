package statsd

import (
	"fmt"
	"github.com/seankndy/gopoller/check"
	"net"
	"strings"
	"time"
)

type Handler struct {
	// Addr is the address of statsd server.
	Addr string
	// Port is the UDP port number of statsd server.
	Port uint16
	// MetricPrefix defines the statsd path prefix for a given Check and Result (default "")
	MetricPrefix func(*check.Check, *check.Result) string
}

func (h *Handler) Mutate(*check.Check, *check.Result, *check.Incident) {
	return
}

func (h *Handler) Process(chk check.Check, newResult check.Result, _ *check.Incident) (err error) {
	if newResult.Metrics == nil {
		return
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("udp", h.Addr)
	if err != nil {
		return
	}
	defer func() {
		err = conn.Close()
	}()

	err = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return
	}

	msg := h.buildProtocolMessage(&chk, &newResult)
	var written int
	for written < len(msg) {
		var n int
		n, err = conn.Write([]byte(msg[written:]))
		if err != nil {
			return
		}
		written += n
	}

	return
}

func (h *Handler) buildProtocolMessage(chk *check.Check, result *check.Result) string {
	var metricPrefix string
	if h.MetricPrefix != nil {
		metricPrefix = strings.TrimRight(h.MetricPrefix(chk, result), ".")
	}

	var msg strings.Builder
	for _, metric := range result.Metrics {
		if metric.Value[:1] == "-" { // negative number
			// see https://github.com/statsd/statsd/blob/master/docs/metric_types.md#gauges
			msg.WriteString(fmt.Sprintf("%s.%s:0|g\n", metricPrefix, metric.Label))
		}
		msg.WriteString(fmt.Sprintf("%s.%s:%s|g\n", metricPrefix, metric.Label, metric.Value))
	}
	return msg.String()
}
